package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"

	redis "github.com/redis/go-redis/v9"
)

type WorkSessionHandler struct {
	workSessionStore store.WorkSessionStore
	userStore        store.UserStore
	logger           *log.Logger
	Middleware       middleware.Middleware

	rds *redis.Client
}

func NewWorkSessionHandler(workSessionStore store.WorkSessionStore, userStore store.UserStore, logger *log.Logger, middleware middleware.Middleware, rds *redis.Client) *WorkSessionHandler {
	return &WorkSessionHandler{
		workSessionStore: workSessionStore,
		userStore:        userStore,
		logger:           logger,
		Middleware:       middleware,
		rds:              rds,
	}
}

type event struct {
	Type      string `json:"type"`
	SessionID int64  `json:"session_id"`
	UserID    int64  `json:"user_id"`
}

var redisEventsChannel = "worktime_events"

func (wh *WorkSessionHandler) publishEvent(ctx context.Context, evt event) {
	wh.logger.Printf("Redis publishing event type=%s session_id=%d user_id=%d",
		evt.Type, evt.SessionID, evt.UserID)
	id, err := wh.rds.XAdd(ctx, &redis.XAddArgs{ // xadd appends new entry to redis stream
		Stream: redisEventsChannel,
		Values: map[string]interface{}{
			"type":       evt.Type,
			"session_id": evt.SessionID,
			"user_id":    evt.UserID,
		},
	}).Result()
	if err != nil {
		wh.logger.Printf("failed to XADD event: %v", err)
	}

	wh.logger.Printf("redis: event stored id=%s", id)
}

func (wh *WorkSessionHandler) ServeSSE(w http.ResponseWriter, r *http.Request) {
	wh.logger.Println("SSE: new client connected")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		wh.logger.Println("SSE: flusher not supported")
		http.Error(w, "streaming failed to connect", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()


	lastID := "$"
	wh.logger.Println("SSE: starting XREAD loop with lastID = ", lastID)

	for {
		select {
		case <-ctx.Done():
			wh.logger.Println("SSE: client disconnected")
			return
		default:
			wh.logger.Println("SSE: waiting for Redis events...")

			streams, err := wh.rds.XRead(ctx, &redis.XReadArgs{
				Streams: []string{redisEventsChannel, lastID},
				Block:   10 * time.Second,
				Count:   10,
			}).Result()
			if err != nil {
				wh.logger.Println("XREAD failed: ", err)
				continue
			}

			for _, s := range streams {
				for _, msg := range s.Messages {


					lastID = msg.ID
					wh.logger.Printf("SSE: event received id=%s type=%v user=%v session=%v",msg.ID,msg.Values["type"],
					msg.Values["user_id"],msg.Values["session_id"])

					
					jsonBytes, err := json.Marshal(msg.Values)
					if err != nil {
						wh.logger.Println("marshal stream failed: ", err)
						continue
					}

					fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
					flusher.Flush()
				}
			}
		}
	}
}

func parseTimeParam(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	if d, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	return time.Time{}, errors.New("invalid time format")
}

func (wh *WorkSessionHandler) HandleStartSession(w http.ResponseWriter, r *http.Request) {
	type sessionRequest struct {
		ProjectID int64  `json:"project_id"`
		Note      string `json:"note"`
	}

	var req sessionRequest

	dec := json.NewDecoder(r.Body)

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

	user, ok := middleware.GetUser(r)
	if !ok || user == nil {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthorized"})
		return
	}

	ws := &store.WorkSession{
		UserId:    user.Id,
		ProjectId: req.ProjectID,
		Note:      req.Note,
	}

	if err := wh.workSessionStore.StartSession(r.Context(), ws); err != nil {
		wh.logger.Printf("Error starting session: %v", err)
		if strings.Contains(err.Error(), "one_active_session_per_user") {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{
				"error": "you already have one active session.Stop it before starting a new sessions",
			})
			return
		}
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}
	evt := event{
		Type:      "session_started",
		SessionID: ws.Id,
		UserID:    user.Id,
	}
	wh.publishEvent(r.Context(), evt)

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

	user, ok := middleware.GetUser(r)
	if !ok || user == nil || user.Id == 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthorized"})
		return
	}

	err = wh.workSessionStore.StopSession(r.Context(), sessionId, user.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "no active session"})
			return
		}
		wh.logger.Println("Error stopping session:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	evt := event{
		Type:      "session_stopped",
		SessionID: sessionId,
		UserID:    user.Id,
	}

	wh.publishEvent(r.Context(), evt)

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"message":    "session stopped",
		"session_id": sessionId,
	})
}

func (wh *WorkSessionHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	u, ok := middleware.GetUser(r)
	if u == nil || u.Id <= 0 || !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	isAdmin := false
	if u.Role == "admin" {
		isAdmin = true
	}

	var filter store.WorkSessionFilter

	filter.Page = utils.ReadInt(r, "page", 1)
	filter.PageSize = utils.ReadInt(r, "page_size", 50)

	if s := strings.TrimSpace(q.Get("search")); s != "" {
		filter.Search = &s
	}

	if s := strings.TrimSpace(q.Get("active")); s != "" {
		v, err := strconv.ParseBool(s)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "active must be true or false"})
			return
		}
		filter.Active = &v
	}

	if s := strings.TrimSpace(q.Get("project_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
			return
		}
		filter.ProjectID = &v
	}

	if isAdmin {
		if s := strings.TrimSpace(q.Get("user_id")); s != "" {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil || v <= 0 {
				utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
				return
			}
			filter.UserID = &v
		}
	} else {
		uid := u.Id
		filter.UserID = &uid
	}

	if err := filter.Validate(); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": err.Error()})
		return
	}

	rows, total, err := wh.workSessionStore.ListSessions(r.Context(), filter)
	if err != nil {
		wh.logger.Println("Error listing sessions:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	meta := store.CalculateMetadata(total, filter.Page, filter.PageSize)

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"result":   rows,
		"metadata": meta,
	})
}

func (wh *WorkSessionHandler) HandleGetSummaryReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// 1) Auth
	authUser, ok := middleware.GetUser(r)
	if !ok || authUser == nil || authUser.Id <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return
	}

	isAdmin := false
	if authUser.Role == "admin" {
		isAdmin = true
	}

	// 2) Required dates: from, to
	fromStr := strings.TrimSpace(q.Get("from"))
	toStr := strings.TrimSpace(q.Get("to"))

	if fromStr == "" || toStr == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "from and to are required"})
		return
	}

	fromDate, err := parseTimeParam(fromStr) //  helper supports YYYY-MM-DD
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid from"})
		return
	}

	toDate, err := parseTimeParam(toStr)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid to"})
		return
	}

	//  Optional project_id
	var requestedProjectID *int64
	if s := strings.TrimSpace(q.Get("project_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
			return
		}
		requestedProjectID = &v
	}

	//  Optional user_id (admin only)
	var requestedUserID *int64
	if s := strings.TrimSpace(q.Get("user_id")); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid user_id"})
			return
		}
		requestedUserID = &v
	}

	// 5)  which user_id i actually allow for the report
	var allowedUserID *int64
	if isAdmin {
		// admin:
		// - if user_id is provided => report for that user
		// - if user_id is missing  => report for all users (nil)
		allowedUserID = requestedUserID
	} else {
		// normal user: force to self no matter what query says
		myID := authUser.Id
		allowedUserID = &myID
	}

	// 6) Build store filter (store layer doesn't care about roles)
	filter := store.SummaryRangeFilter{
		UserID:    allowedUserID,
		ProjectID: requestedProjectID,
		FromDate:  fromDate,
		ToDate:    toDate,
	}

	// 7) Fetch report
	report, err := wh.workSessionStore.GetSummaryReport(r.Context(), filter)
	if err != nil {
		wh.logger.Println("GetSummaryReport error:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"report": report})
}
