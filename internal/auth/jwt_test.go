package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
)

const testSecret = "test-secret-that-is-long-enough-1234"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !auth.CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to match")
	}
	if auth.CheckPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestGenerateAndParseTokenWithClaims(t *testing.T) {
	token, err := auth.GenerateTokenWithClaims(testSecret, "user-1", auth.KindUser, "admin", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := auth.ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.ID != "user-1" || claims.Kind != auth.KindUser || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	token, _ := auth.GenerateTokenWithClaims(testSecret, "user-1", auth.KindUser, "admin", time.Hour)
	if _, err := auth.ParseToken("a-different-secret-that-is-long-enough", token); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
}

func TestParseTokenRejectsExpired(t *testing.T) {
	claims := auth.Claims{
		ID:   "user-1",
		Kind: auth.KindUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := auth.ParseToken(testSecret, token); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestParseTokenRejectsNoneAlgorithm(t *testing.T) {
	claims := auth.Claims{
		ID:   "user-1",
		Kind: auth.KindUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := auth.ParseToken(testSecret, token); err == nil {
		t.Fatal("expected none algorithm to be rejected")
	}
}

func TestLegacyTokenHasNoKind(t *testing.T) {
	token, err := auth.GenerateToken(testSecret, "legacy-id", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := auth.ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.ID != "legacy-id" {
		t.Fatalf("unexpected id: %q", claims.ID)
	}
	if claims.Kind != "" {
		t.Fatalf("legacy token should not have a kind, got %q", claims.Kind)
	}
}

func TestGenerateTokenRejectsEmptyInputs(t *testing.T) {
	if _, err := auth.GenerateTokenWithClaims("", "id", auth.KindUser, "", time.Hour); err == nil {
		t.Fatal("expected empty secret to fail")
	}
	if _, err := auth.GenerateTokenWithClaims(testSecret, "", auth.KindUser, "", time.Hour); err == nil {
		t.Fatal("expected empty id to fail")
	}
	if _, err := auth.GenerateTokenWithClaims(testSecret, "id", auth.KindUser, "", 0); err == nil {
		t.Fatal("expected non-positive ttl to fail")
	}
}
