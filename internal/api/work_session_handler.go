package api

import (
	"log"
	"net/http"

	"github.com/htojiddinov77-png/worktime/internal/store"
)

type WorkSessionHandler struct {
	workSessionStore store.WorkSessionStore
	logger           *log.Logger
}

func NewWorkSessionHandler(workSessionStore store.WorkSessionStore, logger *log.Logger) *WorkSessionHandler {
	return &WorkSessionHandler{
		workSessionStore: workSessionStore,
		logger:           logger,
	}
}

func (wh *WorkSessionHandler) HandleStartSession(w http.ResponseWriter, r *http.Request) {
	type sessionRequest struct {
		ProjectID string `json:"project_id"`
		Note      string `json:"note"`
	}
}
