package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type ResetTokenHandler struct {
	resetTokenStore store.ResetTokenStore
	userStore       store.UserStore
	logger          *log.Logger
}

func NewResetTokenHandler(resetTokenStore store.ResetTokenStore, userStore store.UserStore, logger *log.Logger) *ResetTokenHandler {
	return &ResetTokenHandler{
		resetTokenStore: resetTokenStore,
		userStore:       userStore,
		logger:          logger,
	}
}

func (rh *ResetTokenHandler) HandleGenerateResetLink(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetUser(r)
	if !ok || u == nil || u.Role != "admin" {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
	}

	var input struct {
		Email string `json:"email"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&input); err != nil {
		rh.logger.Println("error while decoding request", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "email is required"})
		return
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(input.Email) {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid email"})
		return
	}

	user, err := rh.userStore.GetUserByEmail(r.Context(), input.Email)
	if err != nil {
		rh.logger.Println("error while getting user by email", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if user == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user is not found"})
		return
	}

	shortToken, err := generateShortToken(12)
	if err != nil {
		rh.logger.Println("generateShortToken error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	hash := hashToken(shortToken)

	ttl := 10 * time.Minute
	expiresAt := time.Now().Add(ttl)

	token := &store.ResetToken{
		UserId:    user.Id,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	}
	err = rh.resetTokenStore.CreateResetToken(r.Context(), token)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	resetLink := "http://localhost:4000/v1/auth/reset-password/" + shortToken

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"reset_link": resetLink,
		"expires_at": expiresAt.Format(time.RFC3339),
	})

}

func (rh *ResetTokenHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "token is required"})
		return
	}

	var input struct {
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&input); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid json body"})
		return
	}

	newPass := strings.TrimSpace(input.NewPassword)
	confirm := strings.TrimSpace(input.ConfirmPassword)

	if newPass == "" || confirm == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "password fields are required"})
		return
	}

	if newPass != confirm {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "password don't match"})
		return
	}

	tokenHash := hashToken(token)

	userID, err := rh.resetTokenStore.UseResetToken(r.Context(), tokenHash)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid or expired token"})
		return
	}

	err = rh.userStore.UpdatePasswordPlain(userID, input.NewPassword)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "password updated succesfully"})
}

func generateShortToken(length int) (string, error) {
	if length < 8 {
		return "", errors.New("token is too short")
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	s := base64.RawURLEncoding.EncodeToString(bytes)
	if len(s) < length {
		return "", errors.New("could not generate token")
	}

	return s[:length], nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
