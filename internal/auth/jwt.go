package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}
type ResetTokenClaims struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
	Purpose  string `json:"purpose"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey []byte
	issuer    string
	ttl       time.Duration
}

func NewJWTManager() *JWTManager {
	secret := os.Getenv("WORKTIME_JWT_SECRET")
	if secret == "" {
		secret = "change-me-in-env-worktime-jwt-secret"
	}

	return &JWTManager{
		secretKey: []byte(secret),
		issuer:    "worktime-api",
		ttl:       24 * time.Hour,
	}
}

func (j *JWTManager) CreateToken(userID int64, email, role string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

func (j *JWTManager) VerifyToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Strictly enforce HS256
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims.Issuer != j.issuer {
		return nil, errors.New("invalid token issuer")
	}

	return claims, nil
}

func (j *JWTManager) CreateResetToken(userID int64, email, role string, isActive bool, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(ttl)

	claims := ResetTokenClaims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		IsActive: isActive,
		Purpose:  "password_reset",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(j.secretKey)
	return s, exp, err
}

func (j *JWTManager) ParseResetToken(tokenString string) (*ResetTokenClaims, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}

	tok, err := jwt.ParseWithClaims(tokenString, &ResetTokenClaims{}, func(t *jwt.Token) (any, error) {
		// enforce HS256, same as VerifyToken
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secretKey, nil
	})
	if err != nil || tok == nil || !tok.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := tok.Claims.(*ResetTokenClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	if claims.Issuer != j.issuer {
		return nil, errors.New("invalid token issuer")
	}
	if claims.Purpose != "password_reset" {
		return nil, ErrInvalidToken
	}
	if claims.UserID <= 0 || claims.Email == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}



