package main

import (
	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 用于管理每个房间的连接
var (
	rooms      = make(map[string][]*websocket.Conn)
	roomsMutex sync.RWMutex

	// 用于跟踪用户在线状态
	userConnections = make(map[string]map[string]*websocket.Conn) // map[userID]map[roomID]*websocket.Conn
	userMutex       sync.RWMutex

	// 每个房间的在线人数
	roomOnlineCount = make(map[string]int)
	roomCountMutex  sync.RWMutex
)

type Message struct {
	Type        string `json:"type"`
	Content     string `json:"content"`
	RoomID      string `json:"room_id"`
	Username    string `json:"username"`
	Timestamp   string `json:"timestamp"`
	OnlineCount int    `json:"online_count,omitempty"`
}

func broadcastToRoom(roomID string, message Message) {
	roomsMutex.RLock()
	defer roomsMutex.RUnlock()

	if connections, exists := rooms[roomID]; exists {
		for _, conn := range connections {
			err := conn.WriteJSON(message)
			if err != nil {
				log.Printf("发送消息失败: %v", err)
				continue
			}
		}
	}
}

// 更新房间在线人数
func updateRoomOnlineCount(roomID string) {
	roomCountMutex.Lock()
	defer roomCountMutex.Unlock()

	roomsMutex.RLock()
	count := len(rooms[roomID])
	roomsMutex.RUnlock()

	roomOnlineCount[roomID] = count

	// 广播新的在线人数
	message := Message{
		Type:        "online_count",
		RoomID:      roomID,
		OnlineCount: count,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}
	broadcastToRoom(roomID, message)
}

func setupWebSocket(r *gin.Engine, db *sql.DB) {
	r.GET("/ws/:room_id", func(c *gin.Context) {
		roomID := c.Param("room_id")
		userID := c.Query("user_id")
		username := c.Query("username")

		// 验证用户权限并获取房间信息
		var roomName, roomType string
		var targetUsername sql.NullString
		err := db.QueryRow(`
            SELECT r.name, r.type,
                CASE 
                    WHEN r.type = 'private' THEN (
                        SELECT u.username 
                        FROM users u 
                        JOIN room_members rm ON u.id = rm.user_id 
                        WHERE rm.room_id = r.id AND rm.user_id != ?
                    )
                    ELSE NULL 
                END as target_username
            FROM chat_rooms r 
            JOIN room_members rm ON r.id = rm.room_id 
            WHERE r.id = ? AND rm.user_id = ?`,
			userID, roomID, userID).Scan(&roomName, &roomType, &targetUsername)

		if err != nil {
			log.Printf("查询房间信息失败: %v", err)
			c.String(http.StatusForbidden, "无权访问此聊天室")
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket升级失败: %v", err)
			return
		}

		// 添加用户连接记录
		userMutex.Lock()
		if userConnections[userID] == nil {
			userConnections[userID] = make(map[string]*websocket.Conn)
		}
		userConnections[userID][roomID] = conn
		userMutex.Unlock()

		// 将连接添加到房间
		roomsMutex.Lock()
		if _, exists := rooms[roomID]; !exists {
			rooms[roomID] = make([]*websocket.Conn, 0)
		}
		rooms[roomID] = append(rooms[roomID], conn)
		roomsMutex.Unlock()

		// 更新并广播在线人数
		updateRoomOnlineCount(roomID)

		// 发送房间信息
		if targetUsername.Valid {
			// 私聊房间
			isOnline := false
			userMutex.RLock()
			for uid, conns := range userConnections {
				if uid != userID && len(conns) > 0 {
					isOnline = true
					break
				}
			}
			userMutex.RUnlock()

			roomInfo := Message{
				Type:        "room_info",
				RoomID:      roomID,
				Content:     roomName,
				Username:    targetUsername.String,
				OnlineCount: map[bool]int{true: 2, false: 1}[isOnline],
			}
			conn.WriteJSON(roomInfo)
		} else {
			// 公共房间
			roomInfo := Message{
				Type:        "room_info",
				RoomID:      roomID,
				Content:     roomName,
				OnlineCount: len(rooms[roomID]),
			}
			conn.WriteJSON(roomInfo)
		}

		// 发送房间信息后，加载并发送历史消息
		rows, err := db.Query(`
            SELECT pm.content, u.username, pm.created_at 
            FROM private_messages pm
            JOIN users u ON pm.user_id = u.id
            WHERE pm.room_id = ?
            ORDER BY pm.created_at ASC
            LIMIT 100`, // 限制加载最近的100条消息
			roomID)

		if err != nil {
			log.Printf("加载历史消息失败: %v", err)
		} else {
			defer rows.Close()

			for rows.Next() {
				var content, msgUsername, timestamp string
				if err := rows.Scan(&content, &msgUsername, &timestamp); err != nil {
					log.Printf("读取历史消息失败: %v", err)
					continue
				}

				historyMsg := Message{
					Type:      "message",
					Content:   content,
					RoomID:    roomID,
					Username:  msgUsername,
					Timestamp: timestamp,
				}

				if err := conn.WriteJSON(historyMsg); err != nil {
					log.Printf("发送历史消息失败: %v", err)
				}
			}
		}

		// 消息处理循环
		for {
			var msg Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Printf("读取消息失败: %v", err)
				break
			}

			// 确保消息包含正确的信息
			msg.RoomID = roomID
			msg.Username = username
			msg.Timestamp = time.Now().Format("2006-01-02 15:04:05")
			msg.Type = "message"

			// 存储消息 - 修改为使用 private_messages 表
			_, err = db.Exec(`
                INSERT INTO private_messages (room_id, user_id, content, created_at) 
                VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
				roomID, userID, msg.Content)
			if err != nil {
				log.Printf("存储消息失败: %v", err)
				continue
			}

			// 广播消息给房间内所有用户
			broadcastToRoom(roomID, msg)
		}

		// 清理连接
		defer func() {
			conn.Close()

			// 移除用户连接记录
			userMutex.Lock()
			if conns, exists := userConnections[userID]; exists {
				delete(conns, roomID)
				if len(conns) == 0 {
					delete(userConnections, userID)
				}
			}
			userMutex.Unlock()

			// 从房间移除连接
			roomsMutex.Lock()
			if conns, exists := rooms[roomID]; exists {
				newConns := make([]*websocket.Conn, 0)
				for _, c := range conns {
					if c != conn {
						newConns = append(newConns, c)
					}
				}
				if len(newConns) == 0 {
					delete(rooms, roomID)
				} else {
					rooms[roomID] = newConns
				}
			}
			roomsMutex.Unlock()

			// 更新并广播新的在线人数
			updateRoomOnlineCount(roomID)
		}()
	})
}
