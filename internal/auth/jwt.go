package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	KindUser     = "user"
	KindCustomer = "customer"
)

type Claims struct {
	ID   string `json:"id"`
	Kind string `json:"kind,omitempty"`
	Role string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken is kept for compatibility with older callers. New code should
// use GenerateTokenWithClaims so the token can be scoped to a user or customer.
func GenerateToken(secret, id string, ttl time.Duration) (string, error) {
	return GenerateTokenWithClaims(secret, id, "", "", ttl)
}

func GenerateTokenWithClaims(secret, id, kind, role string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("jwt secret is empty")
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New("jwt subject is empty")
	}
	if ttl <= 0 {
		return "", errors.New("jwt ttl must be positive")
	}
	now := time.Now()
	claims := Claims{
		ID:   id,
		Kind: kind,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func ParseToken(secret, tokenStr string) (*Claims, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("jwt secret is empty")
	}
	if strings.TrimSpace(tokenStr) == "" {
		return nil, errors.New("token is empty")
	}
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.ID == "" {
		return nil, errors.New("id not found")
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.After(time.Now()) {
		return nil, errors.New("token expired")
	}
	return claims, nil
}
