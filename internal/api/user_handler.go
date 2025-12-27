package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/auth"
	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type UserHandler struct {
	userStore store.UserStore
	logger    *log.Logger
	jwt *auth.JWTManager
}

func NewUserHandler(userStore store.UserStore, logger *log.Logger, jwt *auth.JWTManager) *UserHandler {
	return &UserHandler{
		userStore: userStore,
		logger:    logger,
		jwt: jwt,
	}
}

func (uh *UserHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {

	type registerUserRequest struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req registerUserRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		uh.logger.Println("Error while decoding")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid email"})
		return
	}

	existingUser, err := uh.userStore.GetUserByEmail(req.Email)
	if err != nil {
		uh.logger.Println("Error while getting user:")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	if existingUser != nil {
		uh.logger.Println("Error duplication Entry")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Email is already exists"})
		return
	}

	user := &store.User{
		Name:     req.Name,
		Email:    req.Email,
		IsActive: true,
	}

	err = user.PasswordHash.Set(req.Password)
	if err != nil {
		uh.logger.Println("Error: hashing password")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	err = uh.userStore.CreateUser(user)
	if err != nil {
		uh.logger.Println("Error: while creating user")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
	}

	utils.WriteJson(w, http.StatusCreated, utils.Envelope{"user": user})

}

func (uh *UserHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	existingUser, err := uh.userStore.GetUserById(userID)
	if err != nil {
		uh.logger.Println("GetUserById error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if existingUser == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	type changePasswordRequest struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	var req changePasswordRequest

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		uh.logger.Println("Error while decoding")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "password is empty"})
		return
	}

	oldUserPassword, err := uh.userStore.GetUserById(userID)
	if err != nil {
		uh.logger.Println("GetUserByEmail error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if oldUserPassword == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	passwordsDomatch, err := oldUserPassword.PasswordHash.Matches(req.OldPassword)
	if err != nil {
		uh.logger.Println("Error while comparing passwords")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if !passwordsDomatch {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "unauthorized"})
		return
	}

	err = oldUserPassword.PasswordHash.Set(req.NewPassword)
	if err != nil {
		uh.logger.Println("Error: hashing password")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	err = uh.userStore.UpdateUser(oldUserPassword)
	if err != nil {
		uh.logger.Println("Error: while updating user")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "password changed successfully"})
}

func (uh *UserHandler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthorized"})
		return
	}
	

	existingUser, err := uh.userStore.GetUserById(userId)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if existingUser == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "User not found"})
		return
	}

	var updateUserReqeust struct {
		Name *string `json:"name"`
		Email *string `json:"email"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateUserReqeust)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	if updateUserReqeust.Name != nil {
		if strings.TrimSpace(*updateUserReqeust.Name) == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "user can't be empty"})
			return
		}
		existingUser.Name = *updateUserReqeust.Name
	}

	if updateUserReqeust.Email != nil {
		if strings.TrimSpace(*updateUserReqeust.Email) == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Email cannot be empty"})
			return
		}

		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(*updateUserReqeust.Email) {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Invalid email"})
			return
		}

		userByEmail, err := uh.userStore.GetUserByEmail(*updateUserReqeust.Email)
		if err != nil {
			uh.logger.Println("Error geting userbyemail",err)
			utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
			return
		}
		if userByEmail != nil && existingUser.Id != userByEmail.Id {
			utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Email already exists"})
			return
		}
		existingUser.Email = *updateUserReqeust.Email
	}

	err = uh.userStore.UpdateUser(existingUser)
	if err != nil {
		uh.logger.Println("Error updating user:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return 
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"user": existingUser})
}

func (uh *UserHandler) HandleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	userId, err := utils.ReadIdParam(r)
	if err != nil || userId < 0 {
		uh.logger.Printf("Error getting user id %d", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
	}

	var input struct {
		isActive *bool   `json:"is_active"`
		Role     *string `json:"role"`
	} 

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err = dec.Decode(&input)
	if err != nil {
		uh.logger.Println("Error decoding admin update user:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Invalid JSON body"})
		return
	}

	if input.isActive == nil && input.Role == nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "provie at least one field"})
		return
	}

	if input.Role != nil {
		role := strings.TrimSpace(*input.Role)
		if role == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "role can't be empty"})
			return
		}

		if role != "admin" && role != "user" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid role"})
			return
		}
		input.Role = &role
	}

	err = uh.userStore.AdminUserUpdate(userId ,store.AdminUserUpdate{
		IsActive: input.isActive,
		Role: input.Role,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user not found"})
			return
		}

		if err.Error() == "no fields to update" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "no fields to update"})
			return
		}

		uh.logger.Println("AdminUpdateUser error", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "intenal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK,utils.Envelope{
		"message": "user updated",
		"user_id": userId,
	})
}

func (uh *UserHandler) HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	// Route will be protected by RequireAdmin middleware, so we don’t re-check here.
	users, err := uh.userStore.ListUsers()
	if err != nil {
		uh.logger.Println("ListUsers error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"users": users,
		"count": len(users),
	})
}

func (uh *UserHandler) HandleGenerateResetToekn(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetUserRole(r)
	if role != "admin" {
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "forbidden"})
		return
	}

	var input struct {
		Email string `json:"email"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&input)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if input.Email == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "email is required"})
		return
	}

	user, err := uh.userStore.GetUserByEmail(input.Email)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if user == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user not found"})
		return
	}

	ttl := 10 * time.Minute

	resetToken, expiresAt, err := uh.jwt.CreateResetToken(
		user.Id,
		user.Email,
		user.Role,
		user.IsActive,
		ttl,
	)

	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"reset_token": resetToken,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func (uh *UserHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
		NewPassword string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&input)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	input.Token = strings.TrimSpace(input.Token)
	input.NewPassword = strings.TrimSpace(input.NewPassword)
	input.ConfirmPassword = strings.TrimSpace(input.ConfirmPassword)

	if input.Token == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "token is required"})
		return
	}

	if input.NewPassword == "" || input.ConfirmPassword == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "password fields are required"})
		return
	}

	if input.NewPassword != input.ConfirmPassword {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "password don't match"})
		return
	}

	claims, err := uh.jwt.ParseResetToken(input.Token)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid or expired token"})
		return
	}

	user, err := uh.userStore.GetUserById(claims.UserID)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if user == nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid token user"})
	}

	if !strings.EqualFold(user.Email, claims.Email) {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "token doesn't match"})
		return
	}

	err = user.PasswordHash.Set(input.NewPassword)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	err = uh.userStore.UpdatePasswordPlain(user.Id, input.NewPassword)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "password updated"})
}
