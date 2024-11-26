package main

import (
	"crypto/rand"
	"encoding/base64"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// 生成随机盐值
func generateSalt() (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// 使用 bcrypt 加密密码
func hashPassword(password string, salt string) string {
	// 组合密码和盐值
	combined := password + salt
	hash, _ := bcrypt.GenerateFromPassword([]byte(combined), bcrypt.DefaultCost)
	return string(hash)
}

// 验证密码
func verifyPassword(hashedPassword, password, salt string) bool {
	combined := password + salt
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(combined))
	return err == nil
}
