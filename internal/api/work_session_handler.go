package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type WorkSessionHandler struct {
	workSessionStore store.WorkSessionStore
	userStore        store.UserStore
	logger           *log.Logger
	middleware       middleware.Middleware
}

func NewWorkSessionHandler(workSessionStore store.WorkSessionStore, userStore store.UserStore, logger *log.Logger, middleware middleware.Middleware) *WorkSessionHandler {
	return &WorkSessionHandler{
		workSessionStore: workSessionStore,
		userStore:        userStore,
		logger:           logger,
		middleware:       middleware,
	}
}

func (wh *WorkSessionHandler) HandleStartSession(w http.ResponseWriter, r *http.Request) {
	type sessionRequest struct {
		ProjectID int64  `json:"project_id"`
		Note      string `json:"note"`
	}

	var req sessionRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		wh.logger.Println("Error decoding request")
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "decoding json"})
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unathorized"})
		return
	}
	ws := &store.WorkSession{
		UserId: userID,
		ProjectId: req.ProjectID,
		Note: req.Note,
	}

	err = wh.workSessionStore.StartSession(ws)
	if err != nil {
		wh.logger.Println("Error starting session:")
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	status := "inactive"
	if ws.EndAt == nil {
		status = "active"
	}


	utils.WriteJson(w, http.StatusCreated, utils.Envelope{
		"session": ws,
		"status": status,
	})
}

func (wh *WorkSessionHandler) HandleStopSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{
			"error": "unauthorized",
		})
		return
	}

	err := wh.workSessionStore.StopSession(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{
				"error": "no active session",
			})
			return
		}
		wh.logger.Println("Error stopping session: ", err)
	utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{
		"error": "internal server error",
	})
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"message": "session stopped",
	})
}
