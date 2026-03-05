package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type BatchHandler struct {
	batchStore store.BatchStore
	logger     *log.Logger
	Middleware middleware.Middleware
}

func NewBatchHandler(batchStore store.BatchStore, logger *log.Logger, middleware middleware.Middleware) *BatchHandler {
	return &BatchHandler{
		batchStore: batchStore,
		logger:     logger,
		Middleware: middleware,
	}
}

// -------- DTOs (avoid sql.NullTime JSON noise) --------

type BatchDTO struct {
	ID               int64      `json:"id"`
	ProjectID        int64      `json:"project_id"`
	CreatedByAdminID int64      `json:"created_by"`
	Description      string     `json:"description"`
	Amount           int64      `json:"amount"`
	CreatedAt        time.Time  `json:"created_at"`
	PaidAt           *time.Time `json:"paid_at,omitempty"` // omitted if unpaid
}

func toBatchDTO(b *store.Batch) BatchDTO {
	dto := BatchDTO{
		ID:               b.ID,
		ProjectID:        b.ProjectID,
		CreatedByAdminID: b.CreatedBy,
		Description:      b.Description,
		Amount:           b.Amount,
		CreatedAt:        b.CreatedAt,
	}
	if b.PaidAt.Valid {
		t := b.PaidAt.Time
		dto.PaidAt = &t
	}
	return dto
}

// -------- helpers --------

func (bh *BatchHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u, ok := middleware.GetUser(r)
	if !ok || u == nil || u.Id <= 0 {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "unauthorized"})
		return false
	}
	if u.Role != "admin" {
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "forbidden"})
		return false
	}
	return true
}

func (bh *BatchHandler) adminUserID(r *http.Request) int64 {
	u, _ := middleware.GetUser(r)
	return u.Id
}


func (bh *BatchHandler) HandleCreateBatch(w http.ResponseWriter, r *http.Request) {
	if !bh.requireAdmin(w, r) {
		return
	}

	type reqBody struct {
		ProjectID   int64   `json:"project_id"`
		Description string  `json:"description"`
		Amount      int64   `json:"amount"`
		SessionIDs  []int64 `json:"session_ids"`
	}

	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		bh.logger.Println("decode create batch:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	req.Description = strings.TrimSpace(req.Description)

	if req.ProjectID <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "project_id must be positive"})
		return
	}
	if req.Amount <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "amount must be positive"})
		return
	}
	if len(req.SessionIDs) == 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "session_ids is required"})
		return
	}

	batch := &store.Batch{
		ProjectID:   req.ProjectID,
		CreatedBy:   bh.adminUserID(r),
		Description: req.Description,
		Amount:      req.Amount,
	}

	if err := bh.batchStore.Create(r.Context(), batch); err != nil {
		bh.logger.Println("create batch:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	if err := bh.batchStore.AddItems(r.Context(), batch.ID, req.SessionIDs); err != nil {
		// uq_batch_items_session_id will fail if session already belongs to another batch
		bh.logger.Println("add batch items:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "failed to add batch items"})
		return
	}

	utils.WriteJson(w, http.StatusCreated, utils.Envelope{
		"batch": toBatchDTO(batch),
	})
}


func (bh *BatchHandler) HandleListBatches(w http.ResponseWriter, r *http.Request) {
	if !bh.requireAdmin(w, r) {
		return
	}

	q := r.URL.Query()

	projectIDStr := strings.TrimSpace(q.Get("project_id"))
	if projectIDStr == "" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "project_id is required"})
		return
	}
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil || projectID <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid project_id"})
		return
	}

	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "paid" && status != "unpaid" {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "status must be paid or unpaid"})
		return
	}

	filter := store.Filter{
		Page:         utils.ReadInt(r, "page", 1),
		PageSize:     utils.ReadInt(r, "page_size", 50),
		Sort:         "",  // not used
		SortSafeList: nil, // not used
	}
	if err := filter.Validate(); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": err.Error()})
		return
	}

	rows, total, err := bh.batchStore.List(r.Context(), store.BatchFilters{
		ProjectID: projectID,
		Status:    status,
		Filter:    filter,
	})
	if err != nil {
		bh.logger.Println("list batches:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	out := make([]BatchDTO, 0, len(rows))
	for _, b := range rows {
		out = append(out, toBatchDTO(b))
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"project_id": projectID,
		"status":     status,
		"page":       filter.Page,
		"page_size":  filter.PageSize,
		"total":      total,
		"batches":    out,
	})
}


func (bh *BatchHandler) HandleGetBatch(w http.ResponseWriter, r *http.Request) {
	if !bh.requireAdmin(w, r) {
		return
	}

	id, err := utils.ReadIdParam(r)
	if err != nil || id <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}

	b, err := bh.batchStore.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "not found"})
			return
		}
		bh.logger.Println("get batch:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"batch": toBatchDTO(b)})
}


func (bh *BatchHandler) HandleListBatchItems(w http.ResponseWriter, r *http.Request) {
	if !bh.requireAdmin(w, r) {
		return
	}

	batchID, err := utils.ReadIdParam(r)
	if err != nil || batchID <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}
	

	items, err := bh.batchStore.ListItemDetails(r.Context(), batchID)
	if err != nil {
		bh.logger.Println("list batch items:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{
		"batch_id": batchID,
		"items":    items,
	})
}

func (bh *BatchHandler) HandleMarkBatchPaid(w http.ResponseWriter, r *http.Request) {
	if !bh.requireAdmin(w, r) {
		return
	}

	batchID, err := utils.ReadIdParam(r)
	if err != nil || batchID <= 0 {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid id"})
		return
	}

	type reqBody struct {
		PaidAt *string `json:"paid_at"`
	}

	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid JSON body"})
		return
	}

	paidAt := time.Now().UTC()
	if req.PaidAt != nil && strings.TrimSpace(*req.PaidAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.PaidAt))
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "paid_at must be RFC3339"})
			return
		}
		paidAt = t
	}

	if err := bh.batchStore.MarkPaid(r.Context(), batchID, paidAt); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "not found or already paid"})
			return
		}
		bh.logger.Println("mark paid:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal server error"})
		return
	}

	// return updated batch
	b, err := bh.batchStore.GetByID(r.Context(), batchID)
	if err != nil {
		utils.WriteJson(w, http.StatusOK, utils.Envelope{"message": "batch marked as paid"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"batch": toBatchDTO(b)})
}