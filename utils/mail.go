package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chaos/api/system"

	"chaos/api/model"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"gopkg.in/gomail.v2"
)

// 生成6位随机数字验证码
func Generate6DigitCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func SendVerifyCodeMail(toEmail, sort string) error {
	from := os.Getenv("GMAIL_APP_ACC")
	password := os.Getenv("GMAIL_APP_PWD")

	if len(from) == 0 || len(password) == 0 {
		return fmt.Errorf("unable to send email for missing configuration")
	}

	code := Generate6DigitCode()

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "[TradeVerse] Your Email Verification Code")
	m.SetBody("text/html", fmt.Sprintf(`
        <div style="font-family:Arial,sans-serif;font-size:16px;color:#222;max-width:420px;margin:auto;border:1px solid #e5e7eb;border-radius:12px;padding:32px 24px;background:#f9fafb;">
            <div style="text-align:center;margin-bottom:18px;">
                <span style="display:inline-block;font-size:22px;font-weight:bold;color:#2563eb;letter-spacing:1px;">TradeVerse</span>
            </div>
            <p style="margin-bottom:18px;">Dear user,</p>
            <p style="margin-bottom:18px;">You are receiving this email because you (or someone else) requested an email verification code for your TradeVerse account.</p>
            <div style="text-align:center;margin:24px 0;">
                <span style="display:inline-block;font-size:32px;font-weight:bold;letter-spacing:4px;color:#2563eb;background:#fff;padding:12px 32px;border-radius:8px;border:1px solid #dbeafe;">%s</span>
            </div>
            <p style="margin-bottom:18px;">This code is valid for <b>10 minutes</b>. Please do not share it with anyone.</p>
            <p style="margin-bottom:18px;">If you did not request this code, you can safely ignore this email.</p>
            <div style="margin-top:32px;text-align:center;color:#888;font-size:13px;">This is an official email from TradeVerse. <br/>If you have any questions, please contact us at <a href=\"mailto:nnnnngamefi@gmail.com\" style=\"color:#2563eb;\">nnnnngamefi@gmail.com</a>.</div>
        </div>
    `, code))

	d := gomail.NewDialer("smtp.gmail.com", 587, from, password)

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	vp := model.VerificationProcess{
		Target:         toEmail,
		Type:           "10",
		Code:           code,
		AddTime:        time.Now(),
		ValidatePeriod: 600,
		Sort:           sort,
		Status:         "000",
	}
	db := system.GetDb()
	db.Save(&vp)
	return nil
}

// 读取 Gmail API 服务（基于 credentials.json + token.json）
func gmailService(ctx context.Context) (*gmail.Service, error) {
	credPath := os.Getenv("GMAIL_CREDENTIAL")
	if credPath == "" {
		return nil, fmt.Errorf("GMAIL_CREDENTIAL not set")
	}
	credBytes, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(credBytes, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	// 默认 token.json 放在 credentials 同目录
	tokenPath := os.Getenv("GMAIL_TOKEN")
	if tokenPath == "" {
		tokenPath = filepath.Join(filepath.Dir(credPath), "token.json")
	}
	tb, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read token.json: %w (generate it once via OAuth)", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(tb, &tok); err != nil {
		return nil, fmt.Errorf("parse token.json: %w", err)
	}
	ts := cfg.TokenSource(ctx, &tok)

	return gmail.NewService(ctx, option.WithTokenSource(ts))
}

// 用 Gmail API 发送验证码邮件（替代 SMTP 版本）
func SendVerifyCodeMailAPI(toEmail, sort string) error {
	from := os.Getenv("GMAIL_SENDER")
	if from == "" {
		return fmt.Errorf("GMAIL_SENDER not set")
	}

	code := Generate6DigitCode()

	html := fmt.Sprintf(`
        <div style="font-family:Arial,sans-serif;font-size:16px;color:#222;max-width:420px;margin:auto;border:1px solid #e5e7eb;border-radius:12px;padding:32px 24px;background:#f9fafb;">
            <div style="text-align:center;margin-bottom:18px;">
                <span style="display:inline-block;font-size:22px;font-weight:bold;color:#2563eb;letter-spacing:1px;">KAIVO</span>
            </div>
            <p style="margin-bottom:18px;">Dear user,</p>
            <p style="margin-bottom:18px;">You are receiving this email because you (or someone else) requested an email verification code for your KAIVO account.</p>
            <div style="text-align:center;margin:24px 0;">
                <span style="display:inline-block;font-size:32px;font-weight:bold;letter-spacing:4px;color:#2563eb;background:#fff;padding:12px 32px;border-radius:8px;border:1px solid #dbeafe;">%s</span>
            </div>
            <p style="margin-bottom:18px;">This code is valid for <b>10 minutes</b>. Please do not share it with anyone.</p>
            <p style="margin-bottom:18px;">If you did not request this code, you can safely ignore this email.</p>
            <div style="margin-top:32px;text-align:center;color:#888;font-size:13px;">This is an official email from KAIVO. <br/>If you have any questions, please contact us at <a href="mailto:nnnnngamefi@gmail.com" style="color:#2563eb;">nnnnngamefi@gmail.com</a>.</div>
        </div>
    `, code)

	subject := "Your verification code for KAIVO Platform"

	// 组装 RFC822 报文
	raw := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, toEmail, subject, html,
	)

	ctx := context.Background()
	svc, err := gmailService(ctx)
	if err != nil {
		return err
	}

	_, err = svc.Users.Messages.Send("me", &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}).Do()
	if err != nil {
		return fmt.Errorf("gmail send: %w", err)
	}

	// 持久化与原逻辑一致
	vp := model.VerificationProcess{
		Target:         toEmail,
		Type:           "10",
		Code:           code,
		AddTime:        time.Now(),
		ValidatePeriod: 600,
		Sort:           sort,
		Status:         "000",
		MainID:         0,
	}
	db := system.GetDb()
	db.Save(&vp)
	return nil
}

func SendVerifyCodeMailAPIWithUserMainId(toEmail, sort string, userMainId uint64) error {
	from := os.Getenv("GMAIL_SENDER")
	if from == "" {
		return fmt.Errorf("GMAIL_SENDER not set")
	}

	code := Generate6DigitCode()

	html := fmt.Sprintf(`
        <div style="font-family:Arial,sans-serif;font-size:16px;color:#222;max-width:420px;margin:auto;border:1px solid #e5e7eb;border-radius:12px;padding:32px 24px;background:#f9fafb;">
            <div style="text-align:center;margin-bottom:18px;">
                <span style="display:inline-block;font-size:22px;font-weight:bold;color:#2563eb;letter-spacing:1px;">KAIVO</span>
            </div>
            <p style="margin-bottom:18px;">Dear user,</p>
            <p style="margin-bottom:18px;">You are receiving this email because you (or someone else) requested an email verification code for your KAIVO account.</p>
            <div style="text-align:center;margin:24px 0;">
                <span style="display:inline-block;font-size:32px;font-weight:bold;letter-spacing:4px;color:#2563eb;background:#fff;padding:12px 32px;border-radius:8px;border:1px solid #dbeafe;">%s</span>
            </div>
            <p style="margin-bottom:18px;">This code is valid for <b>10 minutes</b>. Please do not share it with anyone.</p>
            <p style="margin-bottom:18px;">If you did not request this code, you can safely ignore this email.</p>
            <div style="margin-top:32px;text-align:center;color:#888;font-size:13px;">This is an official email from KAIVO. <br/>If you have any questions, please contact us at <a href="mailto:nnnnngamefi@gmail.com" style="color:#2563eb;">nnnnngamefi@gmail.com</a>.</div>
        </div>
    `, code)

	subject := "Your verification code for KAIVO Platform"

	// 组装 RFC822 报文
	raw := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, toEmail, subject, html,
	)

	ctx := context.Background()
	svc, err := gmailService(ctx)
	if err != nil {
		return err
	}

	_, err = svc.Users.Messages.Send("me", &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}).Do()
	if err != nil {
		return fmt.Errorf("gmail send: %w", err)
	}

	// 持久化与原逻辑一致
	vp := model.VerificationProcess{
		Target:         toEmail,
		Type:           "10",
		Code:           code,
		AddTime:        time.Now(),
		ValidatePeriod: 600,
		Sort:           sort,
		Status:         "000",
		MainID:         userMainId,
	}
	db := system.GetDb()
	db.Save(&vp)
	return nil
}

func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 { // 没有域名或格式不对，原样返回
		return email
	}

	local, domain := email[:at], email[at+1:]
	n := len(local)

	switch {
	case n <= 0:
		return "*" + "@" + domain
	case n == 1:
		return "*" + "@" + domain
	default:
		prefix := local[:1]
		suffix := local[n-1:]
		return prefix + "***" + suffix + "@" + domain
	}
}

func MaskUserNo(userNo string) string {
	userNo = strings.TrimSpace(userNo)
	if len(userNo) <= 8 {
		return userNo
	}
	return userNo[:4] + "***" + userNo[len(userNo)-4:]
}
