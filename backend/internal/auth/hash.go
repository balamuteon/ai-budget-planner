package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HashToken возвращает SHA-256 хэш токена в hex-представлении.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CompareTokenHash сравнивает хэш с токеном в константное время.
func CompareTokenHash(hash, token string) bool {
	computed := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(computed)) == 1
}
