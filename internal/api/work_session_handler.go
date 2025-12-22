package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type WorkSessionHandler struct {
	workSessionStore store.WorkSessionStore
	userStore        store.UserStore
	logger           *log.Logger
	Middleware       middleware.Middleware
}

func NewWorkSessionHandler(workSessionStore store.WorkSessionStore, userStore store.UserStore, logger *log.Logger, middleware middleware.Middleware) *WorkSessionHandler {
	return &WorkSessionHandler{
		workSessionStore: workSessionStore,
		userStore:        userStore,
		logger:           logger,
		Middleware:       middleware,
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
		wh.logger.Printf("Error starting session: %v", err)
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

func (wh *WorkSessionHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	// Must be authenticated
	authUserID, ok := middleware.GetUserID(r)
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	role, _ := middleware.GetUserRole(r) // if missing, treat as non-admin

	q := r.URL.Query()

	var (
		userIDPtr    *int64
		projectIDPtr *int64
		activePtr    *bool
		startFromPtr *time.Time
		startToPtr   *time.Time
		searchPtr    *string
	)

	// user_id: admin can filter by it, non-admin must be self
	if userIDStr := strings.TrimSpace(q.Get("user_id")); userIDStr != "" {
		parsed, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || parsed <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
			return
		}
		userIDPtr = &parsed
	}

	// project_id
	if projectIDStr := strings.TrimSpace(q.Get("project_id")); projectIDStr != "" {
		parsed, err := strconv.ParseInt(projectIDStr, 10, 64)
		if err != nil || parsed <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
			return
		}
		projectIDPtr = &parsed
	}

	// status: active|inactive (maps to end_at null/not null)
	// allow both `status=active` or `active=true/false` (optional)
	if status := strings.ToLower(strings.TrimSpace(q.Get("status"))); status != "" {
		switch status {
		case "active":
			v := true
			activePtr = &v
		case "inactive":
			v := false
			activePtr = &v
		default:
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid status (use active or inactive)"})
			return
		}
	} else if activeStr := strings.TrimSpace(q.Get("active")); activeStr != "" {
		parsed, err := strconv.ParseBool(activeStr)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid active (use true/false)"})
			return
		}
		activePtr = &parsed
	}

	// start_from / start_to: RFC3339 timestamps
	if s := strings.TrimSpace(q.Get("start_from")); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid start_from (use RFC3339)"})
			return
		}
		startFromPtr = &t
	}

	if s := strings.TrimSpace(q.Get("start_to")); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid start_to (use RFC3339)"})
			return
		}
		startToPtr = &t
	}

	// search
	if s := strings.TrimSpace(q.Get("search")); s != "" {
		searchPtr = &s
	}

	// pagination
	limit := 50
	if s := strings.TrimSpace(q.Get("limit")); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > 200 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid limit (1..200)"})
			return
		}
		limit = v
	}

	offset := 0
	if s := strings.TrimSpace(q.Get("offset")); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid offset (>=0)"})
			return
		}
		offset = v
	}

	
	if role != "admin" {
		userIDPtr = &authUserID
	} else {
	
	}

	filter := store.WorkSessionFilter{
		UserID:     userIDPtr,
		ProjectID:  projectIDPtr,
		Active:     activePtr,
		StartFrom:  startFromPtr,
		StartTo:    startToPtr,
		Search:     searchPtr,
		Limit:      limit,
		Offset:     offset,
	}

	rows, err := wh.workSessionStore.ListSessions(filter)
	if err != nil {
		wh.logger.Println("ListSessions error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	
	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"filters": utils.Envelope{
			"user_id":     userIDPtr,
			"project_id":  projectIDPtr,
			"status":      q.Get("status"),
			"start_from":  q.Get("start_from"),
			"start_to":    q.Get("start_to"),
			"search":      q.Get("search"),
			"limit":       limit,
			"offset":      offset,
		},
		"sessions": rows,
		"count":    len(rows),
	})
}

func (rh *WorkSessionHandler) HandleDailyReport(w http.ResponseWriter, r *http.Request) {
    authUserID, ok := middleware.GetUserID(r)
    if !ok {
        utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
        return
    }
    role, _ := middleware.GetUserRole(r)

    // date=YYYY-MM-DD (simple for frontend)
    dateStr := strings.TrimSpace(r.URL.Query().Get("date"))
    if dateStr == "" {
        utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "missing date (YYYY-MM-DD)"})
        return
    }

    date, err := time.Parse("2006-01-02", dateStr)
    if err != nil {
        utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid date (YYYY-MM-DD)"})
        return
    }

    // admin can optionally pass user_id; non-admin always self
    var uidPtr *int64
    if role != "admin" {
        uidPtr = &authUserID
    } else {
        // optional: ?user_id=123
        if s := strings.TrimSpace(r.URL.Query().Get("user_id")); s != "" {
            v, err := strconv.ParseInt(s, 10, 64)
            if err != nil || v <= 0 {
                utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
                return
            }
            uidPtr = &v
        }
    }

    summary, err := rh.workSessionStore.GetDailySummary(uidPtr, date)
    if err != nil {
        rh.logger.Println("GetDailySummary error:", err)
        utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
        return
    }

    utils.WriteJson(w, http.StatusOK, utils.Envelope{
        "date":    dateStr,
        "summary": summary,
        "count":   len(summary),
    })
}



