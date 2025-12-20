package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type ProjectHandler struct {
	projectStore store.ProjectStore
	userStore    store.UserStore
	logger       *log.Logger
}

func NewProjectHandler(projectStore store.ProjectStore, logger *log.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectStore: projectStore,
		logger:       logger,
	}
}

func (ph *ProjectHandler) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	type projectRequest struct {
		Name     string `json:"name"`
		StatusId int64  `json:"status_id"`
	}

	var req projectRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		ph.logger.Println("Error decoding request")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Bad request"})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "name can't be empty"})
		return
	}

	if req.StatusId <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "bad request"})
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unathorized"})
		return
	}
	user, err := ph.userStore.GetUserById(userID)
	if err != nil {
		ph.logger.Println("error while getting user by id")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if user == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "user doesn't exist"})
		return
	}

	if user.Role != "admin" {
		ph.logger.Println("Status forbidden")
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "only admin can create a project"})
		return
	}
	pj := &store.Project{
		Name:     req.Name,
		StatusId: req.StatusId,
	}

	err = ph.projectStore.CreateProject(pj)
	if err != nil {
		ph.logger.Println("error while creating project")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusCreated, utils.Envelope{
		"message": "project created succesfully",
		"project": pj,
	})

}
