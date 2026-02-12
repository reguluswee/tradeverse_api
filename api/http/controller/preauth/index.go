package preauth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"chaos/api/api/common"
	"chaos/api/api/service"
	"chaos/api/codes"
	"chaos/api/log"
	"chaos/api/model"
	"chaos/api/security"
	"chaos/api/system"
	"chaos/api/utils"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

const DEFAULT_MSG = "Welcome to N on BSC \n Please sign this message to continue"

var loginInLock sync.Map
var registerInLock sync.Map

type AuthRequestKey struct {
	AuthKey string `json:"auth_key" binding:"required,min=5"`
}
type VerifyAuthRequest struct {
	ID   uint64 `json:"id" binding:"required,min=1"`
	Sign string `json:"sign" binding:"required"`
	Ref  string `json:"ref"`
}

func GetAuthMsg(c *gin.Context) {
	var req AuthRequestKey
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "invalid request" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()

	var authObj model.AuthMessage
	err := db.Model(&model.AuthMessage{}).
		Where("auth_key = ? and expire_time > ?", req.AuthKey, time.Now()).
		Order("create_time desc").
		First(&authObj).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	if authObj.ID == 0 {
		authObj = model.AuthMessage{
			AuthKey:    req.AuthKey,
			AuthMsg:    DEFAULT_MSG,
			CreateTime: time.Now(),
			ExpireTime: time.Now().Add(5 * time.Minute),
			Nonce:      system.GenerateNonce(10),
			Type:       10, // sign in start
		}
		err := db.Save(&authObj).Error
		if err != nil {
			log.Error("create auth msg error: ", err)
		}
	}

	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"
	res.Data = struct {
		ID      uint64 `json:"id"`
		Message string `json:"message"`
	}{
		ID:      authObj.ID,
		Message: authObj.Format(),
	}
	c.JSON(http.StatusOK, res)
}

func VerifyMessage(c *gin.Context) {
	var req VerifyAuthRequest
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "invalid request" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()

	var authObj model.AuthMessage
	err := db.Model(&model.AuthMessage{}).
		Where("id = ?", req.ID).
		First(&authObj).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			res.Code = codes.CODE_ERR_OBJ_NOT_FOUND
			res.Msg = "record not found"
			c.JSON(http.StatusOK, res)
			return
		}
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	log.Info("verify message obj is: ", authObj.ID, authObj.AuthKey, authObj.AuthMsg, authObj.ExpireTime)
	log.Infof("verify message req is: %v", req)
	if authObj.ExpireTime.Before(time.Now()) {
		res.Code = codes.CODE_ERR_REQ_EXPIRED
		res.Msg = "please get a new message"
		c.JSON(http.StatusOK, res)
		return
	}

	// start to verify message
	if !authObj.ComputeAuthDigest(req.Sign) {
		res.Code = codes.CODE_ERR_SIG_COMMON
		res.Msg = "invalid sign"
		c.JSON(http.StatusOK, res)
		return
	}

	_, loaded := loginInLock.LoadOrStore(authObj.ID, struct{}{})
	defer func() {
		loginInLock.Delete(authObj.ID)
	}()
	if loaded {
		res.Code = codes.CODE_ERR_PROCESSING
		res.Msg = "operating, please do not repeat the operation"
		c.JSON(http.StatusOK, res)
		return
	}

	var existWallet model.UserProvider
	var userMain model.UserMain
	err = db.Model(&model.UserProvider{}).
		Where("provider_id = ? and provider_type = ?", authObj.AuthKey, model.PROVIDER_TYPE_WALLET).
		First(&existWallet).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("query wallet error: ", authObj.AuthKey, err)
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = "system error"
		c.JSON(http.StatusOK, res)
		return
	}

	g, err := system.New(1, "Asia/Shanghai")
	if err != nil {
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = "system generator error"
		c.JSON(http.StatusOK, res)
		return
	}

	var existRef model.UserRef
	if req.Ref != "" {
		db.Model(&model.UserRef{}).Where("ref_code = ?", req.Ref).First(&existRef)
	}

	if existRef.ID == 0 && existWallet.ID == 0 { // if both not exist, return error
		res.Code = codes.CODE_ERR_PROCESSING
		res.Msg = "referral code not found"
		c.JSON(http.StatusOK, res)
		return
	}

	if existWallet.ID == 0 {
		userNo, _ := g.Generate(context.Background())
		tx := db.Begin()
		userMain = model.UserMain{
			UserNo:   userNo,
			Email:    nil,
			Password: "",
			AddTime:  time.Now(),
			RefID:    existRef.ID,
			Status:   "00", // waiting for email verification
		}
		err = tx.Save(&userMain).Error
		if err != nil {
			log.Error("save user provider error: ", err)
			tx.Rollback()
			return
		}

		existWallet = model.UserProvider{
			MainID:        userMain.ID,
			ProviderID:    authObj.AuthKey,
			ProviderLabel: "bsc",
			ProviderType:  model.PROVIDER_TYPE_WALLET,
			AddTime:       time.Now(),
		}

		err = tx.Save(&existWallet).Error
		if err != nil {
			log.Error("save user provider error: ", err)
			tx.Rollback()
			return
		}

		tx.Commit()
	}

	_, _ = service.GetUserRef(existWallet.MainID)

	expireTs := time.Now().Add(common.TOKEN_DURATION).Unix()
	tokenOrig := fmt.Sprintf("%d|%d|%d|%d", existWallet.MainID, existWallet.ID, UserLoginTypeWallet, expireTs)
	tokenEnc, err := security.Encrypt([]byte(tokenOrig))
	if err != nil {
		res.Code = codes.CODE_ERR_SECURITY
		res.Msg = "token gen error:" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"
	res.Data = gin.H{
		"user_no":           userMain.UserNo,
		"email":             userMain.Email,
		"provisional_token": tokenEnc,
	}
	c.JSON(http.StatusOK, res)
}

func Register(c *gin.Context) {
	var req RegisterRequest
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "invalid request" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "Invalid email format"
		c.JSON(http.StatusOK, res)
		return
	}

	_, loaded := registerInLock.LoadOrStore(req.Email, struct{}{})
	defer func() {
		registerInLock.Delete(req.Email)
	}()
	if loaded {
		res.Code = codes.CODE_ERR_PROCESSING
		res.Msg = "operating, please do not repeat the operation"
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()

	var userInfo model.UserMain
	err := db.Model(&model.UserMain{}).
		Where("email = ?", req.Email).
		First(&userInfo).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	if userInfo.ID > 0 && userInfo.Status != "00" {
		res.Code = codes.CODE_ERR_EXIST_OBJ
		res.Msg = "email repeated"
		c.JSON(http.StatusOK, res)
		return
	}

	g, _ := system.New(1, "Asia/Shanghai")
	userNo, _ := g.Generate(context.Background())
	passwordHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password)))

	var existRef model.UserRef
	if len(req.Ref) > 0 {
		db.Model(&model.UserRef{}).Where("ref_code = ?", req.Ref).First(&existRef)
	}
	log.Info("existRef: ", existRef)
	if existRef.ID == 0 {
		res.Code = codes.CODE_ERR_PROCESSING
		res.Msg = "referral code not found"
		c.JSON(http.StatusOK, res)
		return
	}

	if userInfo.ID == 0 {
		userInfo = model.UserMain{
			Email:    &req.Email,
			Password: passwordHash,
			UserNo:   userNo,
			AddTime:  time.Now(),
			Status:   "00", // waiting for email verification
			RefID:    existRef.ID,
		}
		err = db.Save(&userInfo).Error
		if err != nil {
			log.Error("create user info error: ", err)
			res.Code = codes.CODE_ERR_UNKNOWN
			res.Msg = err.Error()
			c.JSON(http.StatusOK, res)
			return
		}
	} else if passwordHash != userInfo.Password {
		db.Model(&model.UserMain{}).Where("id = ?", userInfo.ID).Update("password", passwordHash)
	}

	refCode, err := service.GetUserRef(userInfo.ID)
	if err != nil {
		log.Info(refCode, err)
	}

	err = utils.SendVerifyCodeMailAPI(req.Email, "10")
	if err != nil {
		log.Error("send email err", err)
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = "send email failed"
		c.JSON(http.StatusOK, res)
		return
	}

	res.Data = gin.H{
		"user_no": userInfo.UserNo,
	}
	c.JSON(http.StatusOK, res)
}

func Verify(c *gin.Context) {
	var req ConfirmRegisterRequest
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "invalid request" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()

	var userInfo model.UserMain
	db.Model(&model.UserMain{}).
		Where("email = ?", req.Email).
		First(&userInfo)

	if userInfo.ID == 0 {
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = "email not found"
		c.JSON(http.StatusOK, res)
		return
	}

	var verifyProcess model.VerificationProcess
	db.Model(&model.VerificationProcess{}).
		Where("target = ? and code = ? and type = ? and sort = ?", req.Email, req.Code, "10", "10").
		First(&verifyProcess)
	if verifyProcess.ID == 0 {
		res.Code = codes.CODE_ERR_OBJ_NOT_FOUND
		res.Msg = "verification code not sent"
		c.JSON(http.StatusOK, res)
		return
	}

	if time.Now().After(verifyProcess.AddTime.Add(time.Duration(verifyProcess.ValidatePeriod) * time.Second)) {
		res.Code = codes.CODE_ERR_REQ_EXPIRED
		res.Msg = "verification code expired"
		c.JSON(http.StatusOK, res)
		return
	}

	userInfo.Status = "20"
	db.Save(&userInfo)
	verifyProcess.Status = "100"
	db.Save(&verifyProcess)

	proToken := fmt.Sprintf("%d,%d", userInfo.ID, time.Now().Unix())
	token, _ := security.Encrypt([]byte(proToken))

	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"
	res.Data = struct {
		UserNo           string `json:"user_no"`
		ProvisionalToken string `json:"provisional_token"`
	}{
		UserNo:           userInfo.UserNo,
		ProvisionalToken: token,
	}

	res.Data = gin.H{
		"user_no":           userInfo.UserNo,
		"provisional_token": token,
	}
	c.JSON(http.StatusOK, res)
}

func Login(c *gin.Context) {
	var req RegisterRequest
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Code = codes.CODE_ERR_REQFORMAT
		res.Msg = "invalid request" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()

	var userInfo model.UserMain
	db.Model(&model.UserMain{}).
		Where("email = ?", req.Email).
		First(&userInfo)

	passwordHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password)))
	if userInfo.ID == 0 || passwordHash != userInfo.Password {
		res.Code = codes.CODE_ERR_UNKNOWN
		res.Msg = "email not found or password incorrect"
		c.JSON(http.StatusOK, res)
		return
	}

	expireTs := time.Now().Add(common.TOKEN_DURATION).Unix()

	tokenOrig := fmt.Sprintf("%d|%d|%d|%d", userInfo.ID, 0, UserLoginTypeMain, expireTs)
	tokenEnc, err := security.Encrypt([]byte(tokenOrig))
	if err != nil {
		res.Code = codes.CODE_ERR_SECURITY
		res.Msg = "token gen error:" + err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"
	res.Data = gin.H{
		"user_no":           userInfo.UserNo,
		"email":             userInfo.Email,
		"provisional_token": tokenEnc,
	}
	c.JSON(http.StatusOK, res)
}
