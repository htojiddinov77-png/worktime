package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
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
		// dev fallback only — set WORKTIME_JWT_SECRET in real env
		secret = "change-me-in-env-worktime-jwt-secret"
	}

	return &JWTManager{
		secretKey: []byte(secret),
		issuer:    "worktime-api",
		ttl:       24 * time.Hour,
	}
}

// Optional helper if you want to configure issuer/ttl explicitly.


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
