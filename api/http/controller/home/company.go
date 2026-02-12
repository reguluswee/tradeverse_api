package home

import (
	mycache "chaos/api/cache"
	"net/http"
	"time"

	"chaos/api/api/common"
	"chaos/api/codes"
	"chaos/api/log"
	"chaos/api/model"
	"chaos/api/system"
	"chaos/api/thirdpart"

	"github.com/gin-gonic/gin"
)

// CompanyList 获取 Company 列表（缓存 5 分钟）
func CompanyList(c *gin.Context) {
	res := common.Response{}
	res.Timestamp = time.Now().Unix()
	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"

	if list, ok := mycache.GetCompanyList(); ok {
		res.Data = gin.H{"list": list}
		c.JSON(http.StatusOK, res)
		return
	}

	db := system.GetDb()
	var list []model.Company
	err := db.Order("id ASC").Find(&list).Error
	if err != nil {
		log.Error("load companies error", err)
		res.Code = codes.CODE_ERR_REMOTE
		res.Msg = "query failed"
		c.JSON(http.StatusOK, res)
		return
	}

	mycache.SetCompanyList(list)
	res.Data = gin.H{"list": list}
	c.JSON(http.StatusOK, res)
}

// ChartBySymbol 通过 symbol 获取 K 线（Yahoo Finance）
// Query: interval（可选，默认 1h）、range（可选，默认 1mo）
func ChartBySymbol(c *gin.Context) {
	res := common.Response{}
	res.Timestamp = time.Now().Unix()

	symbol := c.Param("symbol")
	if symbol == "" {
		res.Code = codes.CODE_ERR_BAD_PARAMS
		res.Msg = "symbol is required"
		c.JSON(http.StatusOK, res)
		return
	}

	interval := c.DefaultQuery("interval", "1h")
	rangeParam := c.DefaultQuery("range", "1mo")

	if chart, ok := mycache.GetChart(symbol, interval, rangeParam); ok {
		res.Code = codes.CODE_SUCCESS
		res.Msg = "success"
		res.Data = chart
		c.JSON(http.StatusOK, res)
		return
	}

	chart, err := thirdpart.GetChart(symbol, interval, rangeParam)
	if err != nil {
		log.Error("yahoo chart error", err)
		res.Code = codes.CODE_ERR_REMOTE
		res.Msg = err.Error()
		c.JSON(http.StatusOK, res)
		return
	}

	mycache.SetChart(symbol, interval, rangeParam, chart)

	res.Code = codes.CODE_SUCCESS
	res.Msg = "success"
	res.Data = chart
	c.JSON(http.StatusOK, res)
}
