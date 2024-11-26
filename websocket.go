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

	// 新增：用户在线状态管理
	onlineUsers      = make(map[string]bool)
	onlineUsersMutex sync.RWMutex
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
			}
		}
	}
}

// 新增：获取在线用户数量
func getOnlineUsersCount() int {
	onlineUsersMutex.RLock()
	defer onlineUsersMutex.RUnlock()
	return len(onlineUsers)
}

func setupWebSocket(r *gin.Engine, db *sql.DB) {
	r.GET("/ws/:room_id", func(c *gin.Context) {
		roomID := c.Param("room_id")
		userID := c.Query("user_id")
		username := c.Query("username")

		// 验证用户权限并获取房间信息
		var roomName string
		err := db.QueryRow(`
            SELECT r.name 
            FROM chat_rooms r 
            JOIN room_members rm ON r.id = rm.room_id 
            WHERE r.id = ? AND rm.user_id = ?`,
			roomID, userID).Scan(&roomName)
		if err != nil {
			c.String(http.StatusForbidden, "无权访问此聊天室")
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket升级失败: %v", err)
			return
		}

		// 将连接添加到房间
		roomsMutex.Lock()
		if _, exists := rooms[roomID]; !exists {
			rooms[roomID] = make([]*websocket.Conn, 0)
		}
		rooms[roomID] = append(rooms[roomID], conn)
		roomsMutex.Unlock()

		// 用户上线
		onlineUsersMutex.Lock()
		onlineUsers[userID] = true
		onlineUsersMutex.Unlock()
		broadcastOnlineCount()

		// 在函数返回时清理连接
		defer func() {
			conn.Close()
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
			onlineUsersMutex.Lock()
			delete(onlineUsers, userID)
			onlineUsersMutex.Unlock()
			broadcastOnlineCount()
		}()

		log.Printf("新WebSocket连接: 房间=%s(%s), 用户=%s", roomName, roomID, username)

		// 发送历史消息，按时间正序排列
		rows, err := db.Query(`
            SELECT pm.content, u.username, pm.created_at 
            FROM private_messages pm 
            JOIN users u ON pm.user_id = u.id 
            WHERE pm.room_id = ?
            ORDER BY pm.created_at ASC
            LIMIT 50`,
			roomID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var content, msgUsername, createdAt string
				if err := rows.Scan(&content, &msgUsername, &createdAt); err == nil {
					msg := Message{
						Type:      "message",
						Content:   content,
						Username:  msgUsername,
						Timestamp: createdAt,
						RoomID:    roomID, // 添加房间ID
					}
					conn.WriteJSON(msg)
				}
			}
		}

		// 消息处理循环
		for {
			var msg Message
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Printf("读取消息错误: %v", err)
				break
			}

			// 确保消息发送到正确的房间
			msg.RoomID = roomID

			// 存储消息
			_, err = db.Exec(`
                INSERT INTO private_messages (room_id, user_id, content, created_at) 
                VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
				roomID, userID, msg.Content)
			if err != nil {
				log.Printf("存储消息失败: %v", err)
				continue
			}

			// 广播消息给房间内所有用户
			response := Message{
				Type:      "message",
				Content:   msg.Content,
				Username:  username,
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				RoomID:    roomID, // 添加房间ID
			}

			broadcastToRoom(roomID, response)
		}
	})
}

// 新增：广播在线人数
func broadcastOnlineCount() {
	count := getOnlineUsersCount()
	message := Message{
		Type:        "online_count",
		OnlineCount: count,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	roomsMutex.RLock()
	defer roomsMutex.RUnlock()

	// 向所有连接的客户端广播
	for _, connections := range rooms {
		for _, conn := range connections {
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("广播在线人数失败: %v", err)
			}
		}
	}
}
