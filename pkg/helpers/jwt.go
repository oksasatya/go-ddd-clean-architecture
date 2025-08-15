package helpers

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager handles generation and validation of JWT tokens
type JWTManager struct {
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

var defaultManager *JWTManager

func NewJWTManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	m := &JWTManager{
		AccessSecret:  []byte(accessSecret),
		RefreshSecret: []byte(refreshSecret),
		AccessTTL:     accessTTL,
		RefreshTTL:    refreshTTL,
	}
	defaultManager = m
	return m
}

// DefaultJWT returns the last constructed JWTManager (used for auto-wiring routes)
func DefaultJWT() *JWTManager { return defaultManager }

type Claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func (m *JWTManager) GenerateAccessToken(userID string) (string, time.Time, error) {
	exp := time.Now().Add(m.AccessTTL)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(m.AccessSecret)
	return s, exp, err
}

func (m *JWTManager) GenerateRefreshToken(userID string) (string, time.Time, error) {
	exp := time.Now().Add(m.RefreshTTL)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(m.RefreshSecret)
	return s, exp, err
}

func (m *JWTManager) ParseAccessToken(tokenStr string) (*Claims, error) {
	return parseToken(tokenStr, m.AccessSecret)
}

func (m *JWTManager) ParseRefreshToken(tokenStr string) (*Claims, error) {
	return parseToken(tokenStr, m.RefreshSecret)
}

func parseToken(tokenStr string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tkn.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
