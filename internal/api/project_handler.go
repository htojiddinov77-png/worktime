package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type ProjectHandler struct {
	projectStore store.ProjectStore
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
		StatusId string `json:"status_id"`
	}
	var req projectRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		ph.logger.Println("Error decoding request")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Status bad request"})
		return
	}


}
