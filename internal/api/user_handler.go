package api

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type UserHandler struct {
	userStore store.UserStore
	logger    *log.Logger
}

func NewUserHandler(userStore store.UserStore, logger *log.Logger) *UserHandler {
	return &UserHandler{
		userStore: userStore,
		logger:    logger,
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
		Name: req.Name,
		Email: req.Email,
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

func (uh *UserHandler) HandleDisableUser(w http.ResponseWriter, r *http.Request) {
	userID, err := utils.ReadIdParam(r)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "bad request"})
		return
	}

	existingUser, err := uh.userStore.GetUserById(userID)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if existingUser == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user doesn't exist"})
		return
	}

	err = uh.userStore.DisableUser(existingUser.Id)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "user disabled successfully"})
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





