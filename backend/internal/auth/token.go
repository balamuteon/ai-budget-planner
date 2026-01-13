package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type TokenManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenManager инициализирует менеджер JWT токенов.
func NewTokenManager(secret string, issuer string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// NewTokenPair создает пару access/refresh токенов для пользователя.
func (m *TokenManager) NewTokenPair(userID uuid.UUID, refreshTokenID uuid.UUID) (TokenPair, error) {
	accessToken, accessExp, err := m.newToken(userID, uuid.New(), TokenTypeAccess, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, refreshExp, err := m.newToken(userID, refreshTokenID, TokenTypeRefresh, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}, nil
}

// ParseAccessToken валидирует access-токен и возвращает claims.
func (m *TokenManager) ParseAccessToken(tokenString string) (*Claims, error) {
	return m.parseToken(tokenString, TokenTypeAccess)
}

// ParseRefreshToken валидирует refresh-токен и возвращает claims.
func (m *TokenManager) ParseRefreshToken(tokenString string) (*Claims, error) {
	return m.parseToken(tokenString, TokenTypeRefresh)
}

func (m *TokenManager) newToken(userID uuid.UUID, tokenID uuid.UUID, tokenType TokenType, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	claims := Claims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ID:        tokenID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (m *TokenManager) parseToken(tokenString string, tokenType TokenType) (*Claims, error) {
	claims := &Claims{}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}), jwt.WithIssuer(m.issuer))
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	if claims.TokenType != tokenType {
		return nil, errors.New("token type mismatch")
	}

	return claims, nil
}
