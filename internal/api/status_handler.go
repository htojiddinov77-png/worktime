package api

import (
	"net/http"

	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type StatusHandler struct {
	StatusStore store.StatusStore
}

func NewStatusHandler(statusStore store.StatusStore) *StatusHandler {
	return &StatusHandler{StatusStore: statusStore}
}

func (h *StatusHandler) HandleGetAllStatuses(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.StatusStore.GetAllStatuses(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{
			"error": "internal server error",
		})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"statuses": statuses,
	})
}
