package security

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateStrongPassword returns a URL-safe random password of the given length.
func GenerateStrongPassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}

	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	password := base64.RawURLEncoding.EncodeToString(randomBytes)
	if len(password) > length {
		password = password[:length]
	}
	return password, nil
}
