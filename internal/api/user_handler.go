package api

import (
	"encoding/json"
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
	jwt       *auth.JWTManager
}

func NewUserHandler(userStore store.UserStore, logger *log.Logger, jwt *auth.JWTManager) *UserHandler {
	return &UserHandler{
		userStore: userStore,
		logger:    logger,
		jwt:       jwt,
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



func (uh *UserHandler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// 1) target id from URL
	targetUserId, err := utils.ReadIdParam(r)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}


	authUser := middleware.GetUser(r)
	if authUser == nil || authUser.Id <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	
	isAdmin := false
	if authUser.Role == "admin" {
		isAdmin = true
	}

	
	if !isAdmin && targetUserId != authUser.Id {
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "forbidden"})
		return
	}


	existingUser, err := uh.userStore.GetUserById(targetUserId)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	if existingUser == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user not found"})
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Role     *string `json:"role"`      // admin-only
		IsActive *bool   `json:"is_active"` // admin-only
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	
	if !isAdmin && (req.Role != nil || req.IsActive != nil) {
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "forbidden"})
		return
	}

	
	if req.Name == nil && req.Email == nil && req.Role == nil && req.IsActive == nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "no fields to update"})
		return
	}

	

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "name cannot be empty"})
			return
		}
		existingUser.Name = name
	}

	if req.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*req.Email))
		if email == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "email cannot be empty"})
			return
		}

		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(email) {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid email"})
			return
		}

		userByEmail, err := uh.userStore.GetUserByEmail(email)
		if err != nil {
			uh.logger.Println("Error getting user by email:", err)
			utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
			return
		}
		if userByEmail != nil && userByEmail.Id != existingUser.Id {
			utils.WriteJson(w, http.StatusConflict, utils.Envelope{"error": "email already exists"})
			return
		}

		existingUser.Email = email
	}

	if req.Role != nil {
		newRole := strings.TrimSpace(strings.ToLower(*req.Role))
		if newRole != "user" && newRole != "admin" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "role must be 'user' or 'admin'"})
			return
		}
		existingUser.Role = newRole
	}

	if req.IsActive != nil {
		existingUser.IsActive = *req.IsActive
	}

	// 10) save
	if err := uh.userStore.UpdateUser(existingUser); err != nil {
		uh.logger.Println("Error updating user:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"user": existingUser})
}




func (uh *UserHandler) HandleListUsers(w http.ResponseWriter, r *http.Request){
	var input store.ListUserInput

	user := middleware.GetUser(r)
	if user == nil || user.Role != "admin"{
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	input.Search = utils.ReadString(r, "search", "")
	input.Filter.Page = utils.ReadInt(r, "page", 1)
	input.Filter.PageSize = utils.ReadInt(r, "page_size", 50)
	input.Filter.Sort = utils.ReadString(r, "sort", "id")
	input.Filter.SortSafeList = []string{"id", "email", "name"}

	is_active, err := utils.ReadBool(r, "is_active")
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "bad request"})
		return
	}

	is_locked, err := utils.ReadBool(r, "is_locked")
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "bad request"})
		return
	}

	input.IsActive = is_active
	input.IsLocked = is_locked

	if string(input.Sort[0]) == string("-") {
		input.Sort = input.Sort[1:] + " DESC"
	}

	users, metadata, err := uh.userStore.GetAllUsers(r.Context(), input)
	if err != nil {
		uh.logger.Println("error getting all users", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if users == nil {
		utils.WriteJson(w, http.StatusOK, utils.Envelope{"result": []any{}, "message": "no user found"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"result": users, "metadata": metadata})
}

func (uh *UserHandler) HandleGenerateResetToken(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetUser(r)
	if u.Role != "admin" {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
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
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
}

func (uh *UserHandler) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token           string `json:"token"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&input); err != nil {
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
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "passwords do not match"})
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
		return
	}

	// extra safety: token email must match DB email
	if strings.ToLower(strings.TrimSpace(user.Email)) != strings.ToLower(strings.TrimSpace(claims.Email)) {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "token doesn't match"})
		return
	}

	if err := user.PasswordHash.Set(input.NewPassword); err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if err := uh.userStore.UpdatePasswordPlain(user.Id, input.NewPassword); err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "password updated"})
}

