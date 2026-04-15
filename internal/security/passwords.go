package security

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("password must not be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
func CheckPassword(hash, password string) bool {
	if strings.TrimSpace(hash) == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
