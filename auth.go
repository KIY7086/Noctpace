package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func setupAuthRoutes(r *gin.Engine, db *sql.DB) {
	// 统一认证页面路由
	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "",
			})
			return
		}

		// 获取用户信息
		var username string
		err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
		if err != nil {
			c.Redirect(http.StatusFound, "/logout")
			return
		}

		c.HTML(http.StatusOK, "chat.html", gin.H{
			"active":   "chat",
			"user_id":  userID,
			"username": username,
		})
	})

	// 登录路由
	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		var (
			userID         int
			hashedPassword string
			salt           string
		)
		err := db.QueryRow("SELECT id, password, salt FROM users WHERE username = ?",
			username).Scan(&userID, &hashedPassword, &salt)

		if err != nil || !verifyPassword(hashedPassword, password, salt) {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "用户名或密码错误",
			})
			return
		}

		session := sessions.Default(c)
		session.Set("user_id", userID)

		session.Save()

		c.Redirect(http.StatusFound, "/")
	})

	// 注册路由
	r.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		confirmPassword := c.PostForm("confirm_password")

		if password != confirmPassword {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "两次输入的密码不一致",
			})
			return
		}

		if len(password) < 12 {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "密码长度必须至少12位",
			})
			return
		}

		salt, err := generateSalt()
		if err != nil {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "注册失败，请重试",
			})
			return
		}

		hashedPassword := hashPassword(password, salt)

		result, err := db.Exec(`
            INSERT INTO users (username, password, salt, created_at) 
            VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			username, hashedPassword, salt)

		if err != nil {
			c.HTML(http.StatusOK, "auth.html", gin.H{
				"error": "注册失败，用户名可能已存在",
			})
			return
		}

		userID, _ := result.LastInsertId()

		// 注册成功后，将用户添加到公共聊天室
		_, err = db.Exec("INSERT INTO room_members (room_id, user_id) VALUES (1, ?)", userID)
		if err != nil {
			log.Printf("添加用户到公共聊天室失败: %v", err)
		}

		session := sessions.Default(c)
		session.Set("user_id", userID)
		session.Save()

		c.Redirect(http.StatusFound, "/")
	})

	// 登出路由
	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.Redirect(http.StatusFound, "/")
	})
}
