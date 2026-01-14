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

func NewProjectHandler(projectStore store.ProjectStore, userStore store.UserStore, logger *log.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectStore: projectStore,
		userStore:    userStore,
		logger:       logger,
	}
}

func (ph *ProjectHandler) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	type projectRequest struct {
		Name     string `json:"name"`
		StatusId int64  `json:"status_id"`
	}

	var req projectRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		ph.logger.Println("Error decoding request:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "name can't be empty"})
		return
	}

	if req.StatusId <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "status_id must be positive"})
		return
	}

	u := middleware.GetUser(r)
	if u.Id <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	user, err := ph.userStore.GetUserById(r.Context(), u.Id)
	if err != nil {
		ph.logger.Println("error while getting user by id:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	if user == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	if user.Role != "admin" {
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "only admin can create a project"})
		return
	}

	pj := &store.Project{
		ProjectName: req.Name,
	}

	if err := ph.projectStore.CreateProject(r.Context(), pj); err != nil {
		ph.logger.Println("error while creating project:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusCreated, utils.Envelope{
		"message": "project created successfully",
		"project": pj,
	})
}

func (ph *ProjectHandler) HandleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := ph.projectStore.ListProjects(r.Context())
	if err != nil {
		ph.logger.Println("ListProjects error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{
			"error": "internal server error",
		})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"count":    len(projects),
		"projects": projects,
	})
}

func (ph *ProjectHandler) HandleUpdateProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || user.Role != "admin" {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		StatusId int64  `json:"status_id"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	projectId, err := utils.ReadIdParam(r)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}

	err = ph.projectStore.UpdateProject(r.Context(), projectId)
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "project updated succesfully"})
}





