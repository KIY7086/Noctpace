package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func setupChatRoutes(r *gin.Engine, db *sql.DB) {
	// 获取用户列表
	r.GET("/users", func(c *gin.Context) {
		session := sessions.Default(c)
		currentUserID := session.Get("user_id")
		if currentUserID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		rows, err := db.Query(`
            SELECT id, username 
            FROM users 
            WHERE id != ? 
            ORDER BY username`,
			currentUserID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var users []gin.H
		for rows.Next() {
			var id int
			var username string
			if err := rows.Scan(&id, &username); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			users = append(users, gin.H{
				"id":       id,
				"username": username,
			})
		}

		c.JSON(http.StatusOK, users)
	})

	// 开始私聊
	r.POST("/start-chat", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		targetUserID := c.PostForm("target_user_id")
		if targetUserID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "目标用户ID不能为空"})
			return
		}

		// 检查是否为好友关系
		var exists bool
		err := db.QueryRow(`
            SELECT EXISTS(
                SELECT 1 FROM friendships 
                WHERE status = 'accepted'
                AND ((user_id = ? AND friend_id = ?) 
                    OR (friend_id = ? AND user_id = ?))
            )`,
			userID, targetUserID, userID, targetUserID).Scan(&exists)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证好友关系失败"})
			return
		}

		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能与好友进行私聊"})
			return
		}

		// 查找或创建私聊房间的逻辑保持不变
		var roomID int64
		err = db.QueryRow(`
            SELECT r.id 
            FROM chat_rooms r
            JOIN room_members rm1 ON r.id = rm1.room_id
            JOIN room_members rm2 ON r.id = rm2.room_id
            WHERE r.type = 'private'
            AND ((rm1.user_id = ? AND rm2.user_id = ?) 
                OR (rm1.user_id = ? AND rm2.user_id = ?))
            GROUP BY r.id
            HAVING COUNT(DISTINCT rm1.user_id) = 2`,
			userID, targetUserID, targetUserID, userID).Scan(&roomID)

		if err == sql.ErrNoRows {
			// 创建新的私聊房间
			tx, err := db.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "启动事务失败"})
				return
			}
			defer tx.Rollback()

			result, err := tx.Exec(`
                INSERT INTO chat_rooms (name, type, created_at) 
                VALUES (?, 'private', CURRENT_TIMESTAMP)`,
				"私聊房间")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "创建聊天室失败"})
				return
			}

			roomID, _ = result.LastInsertId()

			// 添加房间成员
			_, err = tx.Exec(`
                INSERT INTO room_members (room_id, user_id) VALUES (?, ?), (?, ?)`,
				roomID, userID, roomID, targetUserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "添加房间成员失败"})
				return
			}

			if err := tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
		})
	})

	// 发送私聊消息
	r.POST("/send-private", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		roomID := c.PostForm("room_id")
		message := c.PostForm("message")

		var exists bool
		err := db.QueryRow("SELECT 1 FROM room_members WHERE room_id = ? AND user_id = ?",
			roomID, userID).Scan(&exists)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此聊天室"})
			return
		}

		_, err = db.Exec(`
            INSERT INTO private_messages (room_id, user_id, content) 
            VALUES (?, ?, ?)`,
			roomID, userID, message)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// 获取私聊消息
	r.GET("/private-messages/:room_id", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		roomID := c.Param("room_id")

		var exists bool
		err := db.QueryRow("SELECT 1 FROM room_members WHERE room_id = ? AND user_id = ?",
			roomID, userID).Scan(&exists)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此聊天室"})
			return
		}

		rows, err := db.Query(`
            SELECT pm.content, u.username, pm.created_at 
            FROM private_messages pm 
            JOIN users u ON pm.user_id = u.id 
            WHERE pm.room_id = ?
            ORDER BY pm.created_at DESC LIMIT 50`,
			roomID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var messages []gin.H
		for rows.Next() {
			var content, username, createdAt string
			if err := rows.Scan(&content, &username, &createdAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			messages = append(messages, gin.H{
				"content":    content,
				"username":   username,
				"created_at": createdAt,
			})
		}

		c.JSON(http.StatusOK, messages)
	})

	// 添加创建聊天室的路由
	r.POST("/create-room", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		roomName := c.PostForm("room_name")
		if roomName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "房间名称不能为空"})
			return
		}

		// 创建新聊天室
		result, err := db.Exec(`
            INSERT INTO chat_rooms (name, created_at) 
            VALUES (?, CURRENT_TIMESTAMP)`,
			roomName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建聊天室失败"})
			return
		}

		roomID, _ := result.LastInsertId()

		// 将创建者加入聊天室
		_, err = db.Exec(`
            INSERT INTO room_members (room_id, user_id) 
            VALUES (?, ?)`,
			roomID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "加入聊天室失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"room_id": roomID,
			"name":    roomName,
		})
	})

	// 添加个人中心路由
	r.GET("/profile", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		var username, createdAt string
		err := db.QueryRow("SELECT username, created_at FROM users WHERE id = ?", userID).
			Scan(&username, &createdAt)
		if err != nil {
			c.Redirect(http.StatusFound, "/logout")
			return
		}

		c.HTML(http.StatusOK, "profile.html", gin.H{
			"active":     "profile",
			"user_id":    userID,
			"username":   username,
			"created_at": createdAt,
		})
	})

	// 好友页面路由
	r.GET("/friends", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "friends.html", gin.H{
			"active":  "friends",
			"user_id": userID,
		})
	})

	// 获取好友列表
	r.GET("/api/friends", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, []gin.H{})
			return
		}

		rows, err := db.Query(`
			SELECT DISTINCT u.id, u.username, f.created_at, COALESCE(u.avatar, '') as avatar
			FROM users u
			JOIN friendships f ON 
				(f.user_id = ? AND f.friend_id = u.id) OR 
				(f.friend_id = ? AND f.user_id = u.id)
			WHERE f.status = 'accepted'
				AND NOT EXISTS (
					SELECT 1 FROM friendships f2
					WHERE ((f2.user_id = ? AND f2.friend_id = u.id) OR 
						  (f2.friend_id = ? AND f2.user_id = u.id))
					AND f2.status = 'rejected'
				)
			ORDER BY u.username`,
			userID, userID, userID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, []gin.H{})
			return
		}
		defer rows.Close()

		friends := []gin.H{}
		for rows.Next() {
			var id int
			var username, createdAt, avatar string
			if err := rows.Scan(&id, &username, &createdAt, &avatar); err != nil {
				c.JSON(http.StatusInternalServerError, []gin.H{})
				return
			}

			friends = append(friends, gin.H{
				"id":         id,
				"username":   username,
				"created_at": createdAt,
				"avatar":     avatar,
			})
		}

		c.JSON(http.StatusOK, friends)
	})

	// 发送好友请求
	r.POST("/api/friend-request", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		targetUserID := c.PostForm("target_user_id")
		if targetUserID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "目标用户ID不能为空"})
			return
		}

		// 检查是否已经存在好友关系或待处理的请求
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM friendships 
				WHERE (user_id = ? AND friend_id = ?) 
				   OR (friend_id = ? AND user_id = ?)
			)`,
			userID, targetUserID, userID, targetUserID).Scan(&exists)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证好友关系失败"})
			return
		}

		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "已经发送过好友请求或已经是好友"})
			return
		}

		// 创建好友请求
		_, err = db.Exec(`
			INSERT INTO friendships (user_id, friend_id, status, created_at)
			VALUES (?, ?, 'pending', CURRENT_TIMESTAMP)`,
			userID, targetUserID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "发送好友请求失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "好友请求已发送"})
	})

	// 处理好友请求
	r.POST("/api/friend-request/:action", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		requestID := c.PostForm("request_id")
		action := c.Param("action")

		if action != "accept" && action != "reject" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的操作"})
			return
		}

		status := "accepted"
		if action == "reject" {
			status = "rejected"
		}

		_, err := db.Exec(`
			UPDATE friendships 
			SET status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND friend_id = ?`,
			status, requestID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "处理好友请求失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "好友请求已处理"})
	})

	// 获取好友请求列表
	r.GET("/api/friend-requests", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		// 查询收到的好友请求
		rows, err := db.Query(`
			SELECT f.id, u.username, f.created_at
			FROM friendships f
			JOIN users u ON f.user_id = u.id
			WHERE f.friend_id = ? AND f.status = 'pending'
			ORDER BY f.created_at DESC`,
			userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, []gin.H{})
			return
		}
		defer rows.Close()

		requests := []gin.H{}
		for rows.Next() {
			var id int
			var username, createdAt string
			if err := rows.Scan(&id, &username, &createdAt); err != nil {
				c.JSON(http.StatusInternalServerError, []gin.H{})
				return
			}
			requests = append(requests, gin.H{
				"id":         id,
				"username":   username,
				"created_at": createdAt,
			})
		}

		c.JSON(http.StatusOK, requests)
	})

	// 搜索用户
	r.GET("/api/search-users", func(c *gin.Context) {
		session := sessions.Default(c)
		currentUserID := session.Get("user_id")
		if currentUserID == nil {
			c.JSON(http.StatusUnauthorized, []gin.H{})
			return
		}

		username := c.Query("username")
		if username == "" {
			c.JSON(http.StatusBadRequest, []gin.H{})
			return
		}

		// 查询用户，排除自己和已经是好友的用户
		rows, err := db.Query(`
			SELECT DISTINCT u.id, u.username
			FROM users u
			LEFT JOIN friendships f ON 
				(f.user_id = ? AND f.friend_id = u.id) OR 
				(f.friend_id = ? AND f.user_id = u.id)
			WHERE u.id != ? 
			AND u.username LIKE ?
			AND (f.id IS NULL OR f.status = 'rejected')
			LIMIT 10`,
			currentUserID, currentUserID, currentUserID, "%"+username+"%")
		if err != nil {
			c.JSON(http.StatusInternalServerError, []gin.H{})
			return
		}
		defer rows.Close()

		var users []gin.H
		for rows.Next() {
			var id int
			var username string
			if err := rows.Scan(&id, &username); err != nil {
				c.JSON(http.StatusInternalServerError, []gin.H{})
				return
			}
			users = append(users, gin.H{
				"id":       id,
				"username": username,
			})
		}

		c.JSON(http.StatusOK, users)
	})

	// 获取用户信息
	r.GET("/api/user/profile", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		var username string
		var email, avatar sql.NullString
		err := db.QueryRow("SELECT username, email, avatar FROM users WHERE id = ?", userID).Scan(&username, &email, &avatar)
		if err != nil {
			log.Printf("数据库查询错误: %v", err)
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取用户信息失败: %v", err)})
			return
		}

		emailValue := ""
		if email.Valid {
			emailValue = email.String
		}

		avatarValue := "" // 空字符串表示使用默认图标
		if avatar.Valid && avatar.String != "" {
			avatarValue = avatar.String
		}

		c.JSON(http.StatusOK, gin.H{
			"username": username,
			"email":    emailValue,
			"avatar":   avatarValue,
		})
	})

	// 更新用户信息
	r.POST("/api/user/profile", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		email := c.PostForm("email")

		// 验证邮箱格式
		if email != "" {
			emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
			if !emailRegex.MatchString(email) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
				return
			}
		}

		// 如果邮箱为空字符串，则设置为 NULL
		var err error
		if email == "" {
			_, err = db.Exec("UPDATE users SET email = NULL WHERE id = ?", userID)
		} else {
			_, err = db.Exec("UPDATE users SET email = ? WHERE id = ?", email, userID)
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户信息失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "用户信息已更新"})
	})

	// 上传头像
	r.POST("/api/user/avatar", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		// 获取上传的文件
		file, err := c.FormFile("avatar")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的图片"})
			return
		}

		// 验证文件类型
		if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "只能上传图片文"})
			return
		}

		// 生成文件名
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("avatar_%v_%d%s", userID, time.Now().Unix(), ext)
		filePath := filepath.Join("static", "uploads", "avatars", filename)

		// 确保目录存在
		os.MkdirAll(filepath.Dir(filePath), 0755)

		// 保存文件
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存头像失败"})
			return
		}

		// 更新数据库中的头像路径
		_, err = db.Exec("UPDATE users SET avatar = ? WHERE id = ?", "/"+filePath, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新头像信息失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "头像已更新",
			"avatar":  "/" + filePath,
		})
	})

	// 获取聊天列表
	r.GET("/api/chat-list", func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}

		// 修改 SQL 查询，只获取与好友的私聊
		rows, err := db.Query(`
			SELECT DISTINCT r.id, r.name, r.type, r.created_at,
				CASE 
					WHEN r.type = 'private' THEN (
						SELECT username 
						FROM users u2 
						WHERE u2.id IN (
							SELECT user_id 
							FROM room_members 
							WHERE room_id = r.id AND user_id != ?
						)
					)
					ELSE r.name 
				END as display_name
			FROM rooms r
			JOIN room_members rm ON r.id = rm.room_id
			LEFT JOIN friendships f ON 
				(f.user_id = ? AND f.friend_id = (
					SELECT user_id 
					FROM room_members 
					WHERE room_id = r.id AND user_id != ?
				))
				OR 
				(f.friend_id = ? AND f.user_id = (
					SELECT user_id 
					FROM room_members 
					WHERE room_id = r.id AND user_id != ?
				))
			WHERE rm.user_id = ? 
			AND (r.type = 'public' OR (r.type = 'private' AND f.status = 'accepted'))
			ORDER BY r.updated_at DESC`,
			userID, userID, userID, userID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取聊天列表失败"})
			return
		}
		defer rows.Close()

		var rooms []gin.H
		for rows.Next() {
			var id int
			var name, roomType, createdAt, displayName string
			if err := rows.Scan(&id, &name, &roomType, &createdAt, &displayName); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取聊天列表失败"})
				return
			}
			rooms = append(rooms, gin.H{
				"id":           id,
				"name":         name,
				"type":         roomType,
				"created_at":   createdAt,
				"display_name": displayName,
			})
		}

		c.JSON(http.StatusOK, rooms)
	})
}
