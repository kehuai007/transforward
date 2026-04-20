package auth

import (
	"crypto/rand"
	"encoding/hex"
	"transforward/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func SetPassword(password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	config.Update(func(c *config.Config) {
		c.PasswordHash = hash
	})
	return config.Save(config.GetConfigPath())
}

func NeedInit() bool {
	cfg := config.Get()
	return cfg.PasswordHash == ""
}
