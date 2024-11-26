package main

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
    "log"
)

func initDB() *sql.DB {
    db, err := sql.Open("sqlite3", "chat.db")
    if err != nil {
        log.Fatal(err)
    }

    // 创建用户表
    db.Exec(`CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE,
        password TEXT,
        salt TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`)

    // 修改聊天室表，添加name字段
    db.Exec(`CREATE TABLE IF NOT EXISTS chat_rooms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		type TEXT DEFAULT 'public',  -- 'public' 或 'private'
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	
    // 创建房间成员表
    db.Exec(`CREATE TABLE IF NOT EXISTS room_members (
        room_id INTEGER,
        user_id INTEGER,
        joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (room_id, user_id),
        FOREIGN KEY (room_id) REFERENCES chat_rooms(id),
        FOREIGN KEY (user_id) REFERENCES users(id)
    )`)

    // 创建私聊消息表
    db.Exec(`CREATE TABLE IF NOT EXISTS private_messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        room_id INTEGER,
        user_id INTEGER,
        content TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (room_id) REFERENCES chat_rooms(id),
        FOREIGN KEY (user_id) REFERENCES users(id)
    )`)

    // 检查公共聊天室是否存在
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM chat_rooms WHERE id = 1").Scan(&count)
    if err != nil || count == 0 {
        // 创建公共聊天室
        _, err = db.Exec(`INSERT OR IGNORE INTO chat_rooms (id, name, type) 
                  VALUES (1, '公共大厅', 'public')`)
        if err != nil {
            log.Printf("创建公共聊天室失败: %v", err)
        }
    }

    // 确保所有用户都在公共聊天室中
    rows, err := db.Query("SELECT id FROM users")
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var userID int
            rows.Scan(&userID)
            db.Exec(`INSERT OR IGNORE INTO room_members (room_id, user_id) 
                     VALUES (1, ?)`, userID)
        }
    }

    return db
} 