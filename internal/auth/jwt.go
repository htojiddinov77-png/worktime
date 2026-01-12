package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type UserClaims struct {
	Id     int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Expiry int64  `json:"exp"`
	jwt.RegisteredClaims
}
type ResetClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

var AnonymousUser = &UserClaims{}

type JWTManager struct {
	secretKey []byte
	ttl       time.Duration
}

func NewJWTManager() *JWTManager {
	secret := os.Getenv("WORKTIME_JWT_SECRET")
	if secret == "" {
		secret = "change-me-in-env-worktime-jwt-secret"
	}

	return &JWTManager{
		secretKey: []byte(secret),
		ttl:       24 * time.Hour,
	}
}

func (j *JWTManager) CreateToken(userID int64, email string, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

func (j *JWTManager) VerifyToken(tokenString string) (*UserClaims, error) {
	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return (j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (j *JWTManager) CreateResetToken(userID int64, email, role string, isActive bool, ttl time.Duration) (string, time.Time, error) {

	expiresAt := time.Now().Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":   userID,
		"email":     email,
		"role":      role,
		"is_active": isActive,
		"exp":       expiresAt.Unix(),
	})

	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func (j *JWTManager) ParseResetToken(tokenString string) (*ResetClaims, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, errors.New("missing token")
	}

	claims := &ResetClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return j.secretKey, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.UserID <= 0 || strings.TrimSpace(claims.Email) == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (j *JWTManager) ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("No authorization found in header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("Invalid authorization header")
	}

	return parts[1], nil
}
