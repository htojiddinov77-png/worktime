package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

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
		// same message as wrong password prevents user enumeration
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	if !existingUser.IsActive{
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "user is inactive"})
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

	// IMPORTANT: role comes from DB, not request
	tokenString, err := th.JWT.CreateToken(existingUser.Id, existingUser.Email, existingUser.Role)
	if err != nil {
		th.Logger.Println("CreateToken error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"token": tokenString})
}
