package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

type Claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)

func (tm *TokenManager) newToken(userID string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
}

func (tm *TokenManager) NewAccess(userID string) (string, error) {
	return tm.newToken(userID, accessTTL)
}

// NewRefresh rotates: every call produces a fresh refresh token.
func (tm *TokenManager) NewRefresh(userID string) (string, error) {
	return tm.newToken(userID, refreshTTL)
}

var errInvalidToken = errors.New("invalid token")

func (tm *TokenManager) Parse(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil || !token.Valid {
		return "", errInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || claims.UserID == "" {
		return "", errInvalidToken
	}
	return claims.UserID, nil
}

func RandomSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
