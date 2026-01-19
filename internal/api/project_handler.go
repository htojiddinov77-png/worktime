package api

import (
	"encoding/json"
	"fmt"
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

	u, ok := middleware.GetUser(r)
	if !ok || u.Id <= 0 {
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
		StatusId:    req.StatusId,
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
	ctx := r.Context()

	projects, err := ph.projectStore.ListProjects(ctx)
	if err != nil {
		ph.logger.Println("ListProjects error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	u, ok := middleware.GetUser(r)

	isAdmin := false
	if ok && u != nil && u.Role == "admin" {
		isAdmin = true
	}

	activeByProject := map[int64][]store.ActiveSessionRow{}

	if isAdmin {
		active, err := ph.projectStore.ListActiveSessions(ctx)
		if err != nil {
			ph.logger.Println("ListActiveSessions error:", err)
			utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
			return
		}

		for _, a := range active {
			a.ActiveMinutes = a.ActiveSeconds / 60
			activeByProject[a.ProjectId] = append(activeByProject[a.ProjectId], a)
		}
	}

	for i := range projects {
		projects[i].TotalDurations = formatDuration(projects[i].TotalSeconds)

		if isAdmin {
			projects[i].ActiveSessions = activeByProject[projects[i].Id]
			if projects[i].ActiveSessions == nil {
				projects[i].ActiveSessions = []store.ActiveSessionRow{}
			}
		} else {
			projects[i].ActiveSessions = []store.ActiveSessionRow{}
		}
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"count": len(projects), "projects": projects})
}

// func (ph *ProjectHandler) HandleListProjects(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()

// 	projects, err := ph.projectStore.ListProjects(ctx)
// 	if err != nil {
// 		ph.logger.Println("ListProjects error:", err)
// 		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
// 		return
// 	}

// 	active, err := ph.projectStore.ListActiveSessions(ctx)
// 	if err != nil {
// 		ph.logger.Println("ListActiveSessions error:", err)
// 		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
// 		return
// 	}

// 	activeByProject := map[int64][]store.ActiveSessionRow{}
// 	for _, a := range active {
// 		a.ActiveMinutes = a.ActiveSeconds / 60
// 		activeByProject[a.ProjectId] = append(activeByProject[a.ProjectId], a)
// 	}

// 	for i := range projects {
// 		projects[i].TotalDurations = formatDuration(projects[i].TotalSeconds)
// 		projects[i].ActiveSessions = activeByProject[projects[i].Id] // nil -> omitted? see note below
// 		if projects[i].ActiveSessions == nil {
// 			projects[i].ActiveSessions = []store.ActiveSessionRow{}
// 		}
// 	}

// 	utils.WriteJson(w, http.StatusOK, utils.Envelope{"count": len(projects), "projects": projects})
// }



func (ph *ProjectHandler) HandleUpdateProject(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r)
	if !ok || user == nil || user.Role != "admin" {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	var req struct {
		Name     *string `json:"name"`
		StatusId *int64  `json:"status_id"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	if req.Name == nil && req.StatusId == nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "at least one field is required: name or status_id"})
		return
	}

	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "name cannot be empty"})
			return
		}
		req.Name = &n
	}

	if req.StatusId != nil && *req.StatusId <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "status_id must be positive"})
		return
	}

	projectId, err := utils.ReadIdParam(r)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}

	err = ph.projectStore.UpdateProject(r.Context(), projectId, req.Name, req.StatusId)
	if err != nil {
		ph.logger.Println("error updating project:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "project updated successfully"})
}

func formatDuration(totalSeconds int64) string {
	//days := totalSeconds / 86400
	//hours := (totalSeconds % 86400) / 3600
	minutes := totalSeconds / 60
	//secs := totalSeconds % 60

	return fmt.Sprintf("%02d minutes", minutes)
}
