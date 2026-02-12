package mycache

import (
	"time"

	"chaos/api/model"

	"github.com/dgraph-io/ristretto/v2"
)

const companyCacheTTL = 5 * time.Minute

const companyListCacheKey = "companies"

var CompanyCache *ristretto.Cache[string, []model.Company]

func init() {
	cache, err := ristretto.NewCache[string, []model.Company](&ristretto.Config[string, []model.Company]{
		NumCounters: 1000,
		MaxCost:     10 * 1024 * 1024, // 10MB
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	CompanyCache = cache
}

// GetCompanyList 从缓存读取公司列表，ok 表示命中
func GetCompanyList() ([]model.Company, bool) {
	CompanyCache.Wait()
	return CompanyCache.Get(companyListCacheKey)
}

// SetCompanyList 写入公司列表到缓存，TTL 5 分钟
func SetCompanyList(list []model.Company) {
	if list == nil {
		return
	}
	cost := int64(1)
	if len(list) > 0 {
		cost = int64(len(list))
	}
	CompanyCache.SetWithTTL(companyListCacheKey, list, cost, companyCacheTTL)
	CompanyCache.Wait()
}
