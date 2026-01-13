package router

import (
	"net/http"
	"os"

	general "chaos/api/api/http"
	"chaos/api/api/http/controller/auth"
	"chaos/api/api/interceptor"
	"chaos/api/api/oauth"
	"chaos/api/api/ws"
	"chaos/api/log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Option func(*gin.RouterGroup)

var options = []Option{}

func Include(opts ...Option) {
	options = append(options, opts...)
}

func Init() *gin.Engine {
	Include(general.Routers)

	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	env := os.Getenv("ENV")

	log.Info(env)
	if env == "dev" {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "APPID", "SIG", "TS", "VER", "REQUESTID", "XAUTH", "DAUTH"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
		}))
	}

	secure := true
	if env == "dev" {
		secure = false
	}
	r.Use(oauth.Sessions(secure))

	r.GET("/index", helloHandler) //Default welcome api

	wsGroup := r.Group("/ws", interceptor.WSInterceptor())
	wsGroup.GET("chat", ws.Chat)

	// Twitter OAuth login endpoint - no authentication required
	r.GET("/spwapi/preauth/thirdpart/x/login", auth.XLogin)

	apiGroup := r.Group("/spwapi", interceptor.HttpInterceptor()) // total interceptor stack
	for _, opt := range options {
		opt(apiGroup)
	}

	oauthGroup := r.Group("/oauth")
	{
		oauthGroup.GET("/login", oauth.LoginPage)
		oauthGroup.POST("/login", oauth.LoginPost)
		oauthGroup.POST("/logout", oauth.Logout) // 建议POST避免CSRF

		// 用户浏览器跳转，展示登录/授权页面
		oauthGroup.GET("/authorize", oauth.AuthorizeHandler)
		// 第三方应用用 code 换 token
		oauthGroup.POST("/token", oauth.TokenHandler)
	}
	apiGroup2 := r.Group("/oapi", oauth.AuthzMiddleware(oauth.GetJWTSecret()))
	{
		apiGroup2.GET("/me", oauth.MeHandler)
		apiGroup2.POST("/game/session/init", oauth.GameSessionInitHandler)
		apiGroup2.POST("/game/start", oauth.GameStartHandler)
		apiGroup2.POST("/game/end", oauth.GameEndHandler)

		// 高级接口 - 添加额外中间件
		advancedGroup := apiGroup2.Group("", oauth.AdvancedAuthMiddleware())
		{
			advancedGroup.POST("/trans/freeze", oauth.FreezeHandler)
			advancedGroup.POST("/trans/unfreeze", oauth.UnFreezeHandler)
			advancedGroup.POST("/trans/spend", oauth.SpendHandler)
		}
		//this is directly money control api, need special advanced auth
		// apiGroup2.POST("/trans/freeze", oauth.FreezeHandler)
		// apiGroup2.POST("/trans/unfreeze", oauth.UnFreezeHandler)
		// apiGroup2.POST("/trans/spend", oauth.SpendHandler)
	}
	return r
}

func helloHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Hello Tradeverse",
	})
}
