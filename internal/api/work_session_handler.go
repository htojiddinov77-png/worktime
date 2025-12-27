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

func NewWorkSessionHandler(
	workSessionStore store.WorkSessionStore,
	userStore store.UserStore,
	logger *log.Logger,
	middleware middleware.Middleware,
) *WorkSessionHandler {
	return &WorkSessionHandler{
		workSessionStore: workSessionStore,
		userStore:        userStore,
		logger:           logger,
		Middleware:       middleware,
	}
}

// --- metadata (book-style) ---

type metadata struct {
	CurrentPage  int `json:"current_page"`
	PageSize     int `json:"page_size"`
	FirstPage    int `json:"first_page"`
	LastPage     int `json:"last_page"`
	TotalRecords int `json:"total_records"`
}

func calculateMetadata(totalRecords, page, pageSize int) metadata {
	if totalRecords == 0 {
		return metadata{}
	}
	lastPage := (totalRecords + pageSize - 1) / pageSize
	return metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     lastPage,
		TotalRecords: totalRecords,
	}
}

// parseTimeParam accepts either RFC3339 (full timestamp) OR "2006-01-02" (date only).
func parseTimeParam(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}

	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try YYYY-MM-DD (treat as start of day UTC)
	if d, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	return time.Time{}, errors.New("invalid time format")
}

// ---------------- Handlers ----------------

func (wh *WorkSessionHandler) HandleStartSession(w http.ResponseWriter, r *http.Request) {
	type sessionRequest struct {
		ProjectID int64  `json:"project_id"`
		Note      string `json:"note"`
	}

	var req sessionRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		wh.logger.Println("Error decoding request:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	if req.ProjectID <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "project_id must be positive"})
		return
	}
	req.Note = strings.TrimSpace(req.Note)

	userID, ok := middleware.GetUserID(r)
	if !ok || userID <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	ws := &store.WorkSession{
		UserId:    userID,
		ProjectId: req.ProjectID,
		Note:      req.Note,
	}

	if err := wh.workSessionStore.StartSession(ws); err != nil {
		wh.logger.Printf("Error starting session: %v", err)
		if strings.Contains(err.Error(), "one_active_session_per_user") {
			utils.WriteJson(w, http.StatusConflict, utils.Envelope{
				"error": "you already have one active session.Stop it before starting a new sessions",
			})
		}
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	// new session is active because EndAt is nil
	utils.WriteJson(w, http.StatusCreated, utils.Envelope{
		"session": ws,
		"status":  "active",
	})
}

func (wh *WorkSessionHandler) HandleStopSession(w http.ResponseWriter, r *http.Request) {
	sessionId, err := utils.ReadIdParam(r)
	if err != nil || sessionId <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok || userID <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	err = wh.workSessionStore.StopSession(sessionId, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "no active session"})
			return
		}
		wh.logger.Println("Error stopping session:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"message":    "session stopped",
		"session_id": sessionId,
	})
}

func (wh *WorkSessionHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := middleware.GetUserID(r)
	if !ok || authUserID <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	role, _ := middleware.GetUserRole(r) // if missing => non-admin
	q := r.URL.Query()

	var (
		userIDPtr    *int64
		projectIDPtr *int64
		activePtr    *bool
		startFromPtr *time.Time
		startToPtr   *time.Time
		searchPtr    *string
	)

	// Admin can filter by user_id; non-admin forced to self
	if s := strings.TrimSpace(q.Get("user_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
			return
		}
		userIDPtr = &v
	}

	// project_id
	if s := strings.TrimSpace(q.Get("project_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
			return
		}
		projectIDPtr = &v
	}

	// status: active|inactive OR active=true/false
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
	} else if s := strings.TrimSpace(q.Get("active")); s != "" {
		v, err := strconv.ParseBool(s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid active (use true/false)"})
			return
		}
		activePtr = &v
	}

	// start_from/start_to: RFC3339 OR YYYY-MM-DD
	if s := strings.TrimSpace(q.Get("start_from")); s != "" {
		t, err := parseTimeParam(s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid start_from (use RFC3339 or YYYY-MM-DD)"})
			return
		}
		startFromPtr = &t
	}
	if s := strings.TrimSpace(q.Get("start_to")); s != "" {
		t, err := parseTimeParam(s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid start_to (use RFC3339 or YYYY-MM-DD)"})
			return
		}
		startToPtr = &t
	}

	// search
	if s := strings.TrimSpace(q.Get("search")); s != "" {
		searchPtr = &s
	}

	// page/page_size (book)
	page := 1
	if s := strings.TrimSpace(q.Get("page")); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid page (>=1)"})
			return
		}
		page = v
	}

	pageSize := 50
	if s := strings.TrimSpace(q.Get("page_size")); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > 200 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid page_size (1..200)"})
			return
		}
		pageSize = v
	}

	// sort (store enforces allowlist; we just provide default)
	sort := strings.TrimSpace(q.Get("sort"))
	if sort == "" {
		sort = "-start_at"
	}

	// Ownership enforcement
	if role != "admin" {
		userIDPtr = &authUserID
	}

	filter := store.WorkSessionFilter{
		UserID:    userIDPtr,
		ProjectID: projectIDPtr,
		Active:    activePtr,
		StartFrom: startFromPtr,
		StartTo:   startToPtr,
		Search:   searchPtr,

		Page:     page,
		PageSize: pageSize,
		Sort:     sort,
	}

	rows, total, err := wh.workSessionStore.ListSessions(filter)
	if err != nil {
		wh.logger.Println("ListSessions error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"sessions": rows,
		"metadata": calculateMetadata(total, page, pageSize),
	})
}

func (wh *WorkSessionHandler) HandleSummaryReport(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := middleware.GetUserID(r)
	if !ok || authUserID <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	role, _ := middleware.GetUserRole(r)

	fromStr := strings.TrimSpace(r.URL.Query().Get("from"))
	toStr := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromStr == "" || toStr == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "missing from/to (YYYY-MM-DD)"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid from (YYYY-MM-DD)"})
		return
	}
	toDate, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid to (YYYY-MM-DD)"})
		return
	}

	// user_id: non-admin forced to auth user, admin optional
	var uidPtr *int64
	if role != "admin" {
		uidPtr = &authUserID
	} else {
		if s := strings.TrimSpace(r.URL.Query().Get("user_id")); s != "" {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v <= 0 {
				utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
				return
			}
			uidPtr = &v
		}
	}

	// project_id optional
	var pidPtr *int64
	if s := strings.TrimSpace(r.URL.Query().Get("project_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
			return
		}
		pidPtr = &v
	}

	filter := store.SummaryRangeFilter{
		UserID:    uidPtr,
		ProjectID: pidPtr,
		FromDate:  fromDate,
		ToDate:    toDate,
	}

	rep, err := wh.workSessionStore.GetSummaryReport(filter)
	if err != nil {
		wh.logger.Println("GetSummaryReport error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	// You can return rep directly too, but keeping your response shape:
	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"from":       rep.From,
		"to":         rep.To,
		"filters":    rep.Filters,
		"user":       rep.User,
		"overall":    rep.Overall,
		"by_project": rep.ByProject,
	})
}
