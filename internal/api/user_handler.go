package api

import (
	"encoding/json"
	"log"
	"net/http"

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
