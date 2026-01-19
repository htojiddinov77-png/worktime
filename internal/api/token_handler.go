package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/auth"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type TokenHandler struct {
	UserStore store.UserStore
	Logger    *log.Logger
	JWT       *auth.JWTManager
}

func NewTokenHandler(userStore store.UserStore, jwtManager *auth.JWTManager, logger *log.Logger) *TokenHandler {
	return &TokenHandler{
		UserStore: userStore,
		JWT:       jwtManager,
		Logger:    logger,
	}
}

func (th *TokenHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "email or password is empty"})
		return
	}

	existingUser, err := th.UserStore.GetUserByEmail(r.Context(),req.Email)
	if err != nil {
		th.Logger.Println("GetUserByEmail error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if existingUser == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	if !existingUser.IsActive {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "user is inactive"})
		return
	}

	if existingUser.IsLocked && time.Since(existingUser.LastFailedLogin.Time) < time.Hour*24 {
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
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "password is incorrect"})
		th.UserStore.LoginFail(r.Context(), existingUser.Email)

		if existingUser.FailedAttempts+1 > 4 {
			th.UserStore.Lockout(r.Context(), existingUser.Email)
		}
		return
	} else {
		th.UserStore.Unlock(r.Context(), existingUser.Email)
	}

	tokenString, err := th.JWT.CreateToken(existingUser.Id, existingUser.Email, existingUser.Role)
	if err != nil {
		th.Logger.Println("CreateToken error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"token": tokenString,
		"name": existingUser.Name,
		"role":  existingUser.Role,
	})
}
