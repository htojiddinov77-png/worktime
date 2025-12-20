package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type TokenHandler struct {
	UserStore store.UserStore
	Logger    *log.Logger

	secretKey []byte
	issuer    string
}

// NewTokenHandler creates a TokenHandler.
// It reads JWT secret from env var WORKTIME_JWT_SECRET (recommended).
// For local dev, you can pass a fallback secret string.
// NOTE: Keep the secret long + random in real envs.
func NewTokenHandler(userStore store.UserStore, logger *log.Logger) *TokenHandler {
	secret := os.Getenv("WORKTIME_JWT_SECRET")
	if secret == "" {
		// Fallback for local/dev. Replace in real environment.
		secret = "change-me-in-env-worktime-jwt-secret"
	}

	return &TokenHandler{
		UserStore:  userStore,
		Logger:     logger,
		secretKey:  []byte(secret),
		issuer:     "worktime-api",
	}
}


type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}


type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (th *TokenHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "email or password is empty"})
		return
	}


	existingUser, err := th.UserStore.GetUserByEmail(req.Email)
	if err != nil {
		th.Logger.Println("GetUserByEmail error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}


	if existingUser == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	matches, err := existingUser.PasswordHash.Matches(req.Password)
	if err != nil {
		th.Logger.Println("password match error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if !matches {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	
	tokenString, err := th.CreateToken(existingUser.Id, existingUser.Email, existingUser.Role)
	if err != nil {
		th.Logger.Println("CreateToken error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"token": tokenString})
}

func (th *TokenHandler) CreateToken(userID int64, email string, role string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			// Common standard fields.
			Issuer:    th.issuer,
			Subject:   fmt.Sprintf("%d", userID), 
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(th.secretKey)
}


func (th *TokenHandler) VerifyToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return th.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Optional: issuer check (extra safety)
	if claims.Issuer != th.issuer {
		return nil, errors.New("invalid token issuer")
	}

	return claims, nil
}
