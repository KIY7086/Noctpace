package main

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	db := initDB()
	defer db.Close()

	r := gin.Default()

	// 添加静态文件支持
	r.Static("/static", "./static")
	r.StaticFile("/favicon.ico", "./static/images/favicon.ico")

	// 设置 session
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	// 设置模板
	r.LoadHTMLFiles(
		"templates/auth.html",
		"templates/chat.html",
		"templates/profile.html",
		"templates/components/navbar.html",
	)

	// 安全头部中间件
	r.Use(securityHeaders())

	// 注册路由
	setupAuthRoutes(r, db) // 认证相关路由
	setupChatRoutes(r, db) // 聊天相关路由

	// 添加 WebSocket 支持
	setupWebSocket(r, db)

	r.Run(":8080")
}

// 安全头部中间件
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}
