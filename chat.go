package main

import (
	"database/sql"
	"net/http"

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

		// 先查找这两个用户之间是否已经有私聊房间
		var roomID int64
		err := db.QueryRow(`
            SELECT r.id 
            FROM chat_rooms r
            JOIN room_members rm1 ON r.id = rm1.room_id
            JOIN room_members rm2 ON r.id = rm2.room_id
            WHERE r.type = 'private'  -- 添加房间类型区分
            AND ((rm1.user_id = ? AND rm2.user_id = ?) OR (rm1.user_id = ? AND rm2.user_id = ?))
            GROUP BY r.id
            HAVING COUNT(DISTINCT rm1.user_id) = 2`,
			userID, targetUserID, targetUserID, userID).Scan(&roomID)

		if err == sql.ErrNoRows {
			// 如果没有找到，创建新的私聊房间
			tx, err := db.Begin()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "启动事务失败"})
				return
			}
			defer tx.Rollback()

			// 创建新房间，添加类型标记
			result, err := tx.Exec(`
                INSERT INTO chat_rooms (name, type, created_at) 
                VALUES (?, 'private', CURRENT_TIMESTAMP)`,
				"私聊房间")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "创建聊天室失败"})
				return
			}

			roomID, _ = result.LastInsertId()

			// 将两个用户加入房间
			_, err = tx.Exec(`
                INSERT INTO room_members (room_id, user_id) 
                VALUES (?, ?), (?, ?)`,
				roomID, userID, roomID, targetUserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "加入聊天室失败"})
				return
			}

			if err = tx.Commit(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败"})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询聊天室失败"})
			return
		}

		// 返回房间ID（无论是新创建的还是已存在的）
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
}
