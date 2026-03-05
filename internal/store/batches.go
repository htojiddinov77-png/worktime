package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("record not found")

type Batch struct {
	ID          int64        `json:"id"`
	ProjectID   int64        `json:"project_id"`
	CreatedBy   int64        `json:"created_by"`
	Description string       `json:"description"`
	Amount      int64        `json:"amount"`
	CreatedAt   time.Time    `json:"created_at"`
	PaidAt      sql.NullTime `json:"paid_at"`
}

type BatchItem struct {
	BatchID   int64     `json:"batch_id"`
	SessionID int64     `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

type BatchFilters struct {
	ProjectID int64
	Status    string // "", "paid", "unpaid"
	Filter    Filter
}

type BatchItemDetail struct {
	BatchID   int64     `json:"batch_id"`
	SessionID int64     `json:"session_id"`

	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`

	StartAt time.Time  `json:"start_at"`
	EndAt   time.Time  `json:"end_at"`
	Seconds int64      `json:"seconds"`
	Note    string     `json:"note"`
}

type BatchStore interface {
	Create(ctx context.Context, b *Batch) error
	AddItems(ctx context.Context, batchID int64, sessionIDs []int64) error

	GetByID(ctx context.Context, id int64) (*Batch, error)
	List(ctx context.Context, bf BatchFilters) ([]*Batch, int, error)

	ListItems(ctx context.Context, batchID int64) ([]*BatchItem, error)
	MarkPaid(ctx context.Context, batchID int64, paidAt time.Time) error
	ListItemDetails(ctx context.Context, batchID int64) ([]*BatchItemDetail, error)
}

type PostgresBatchStore struct {
	db *sql.DB
}

func NewPostgresBatchStore(db *sql.DB) *PostgresBatchStore {
	return &PostgresBatchStore{db: db}
}

func (s *PostgresBatchStore) Create(ctx context.Context, b *Batch) error {
	const q = `
		INSERT INTO batches (project_id, created_by, description, amount)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, paid_at;
	`

	return s.db.QueryRowContext(ctx, q, b.ProjectID, b.CreatedBy, b.Description, b.Amount).
		Scan(&b.ID, &b.CreatedAt, &b.PaidAt)
}

func (s *PostgresBatchStore) AddItems(ctx context.Context, batchID int64, sessionIDs []int64) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
		INSERT INTO batch_items (batch_id, session_id)
		VALUES ($1, $2);
	`

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sid := range sessionIDs {
		if _, err := stmt.ExecContext(ctx, batchID, sid); err != nil {
			// uq_batch_items_session_id will fail if this session already belongs to another batch
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresBatchStore) GetByID(ctx context.Context, id int64) (*Batch, error) {
	const q = `
		SELECT id, project_id, created_by, description, amount, created_at, paid_at
		FROM batches
		WHERE id = $1;
	`

	var b Batch
	err := s.db.QueryRowContext(ctx, q, id).
		Scan(&b.ID, &b.ProjectID, &b.CreatedBy, &b.Description, &b.Amount, &b.CreatedAt, &b.PaidAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &b, nil
}

func (s *PostgresBatchStore) List(ctx context.Context, bf BatchFilters) ([]*Batch, int, error) {
	// We only use Filter for pagination
	bf.Filter.Sort = ""
	bf.Filter.SortSafeList = nil

	if err := bf.Filter.Validate(); err != nil {
		return nil, 0, err
	}

	if bf.ProjectID <= 0 {
		return nil, 0, errors.New("project_id must be positive")
	}

	where := []string{"project_id = $1"}
	args := []any{bf.ProjectID}

	switch strings.TrimSpace(bf.Status) {
	case "":
		// no status filter
	case "paid":
		where = append(where, "paid_at IS NOT NULL")
	case "unpaid":
		where = append(where, "paid_at IS NULL")
	default:
		return nil, 0, errors.New("invalid status filter")
	}

	// positions for LIMIT/OFFSET depend on args length
	args = append(args, bf.Filter.Limit(), bf.Filter.Offset())
	limitPos := len(args) - 1
	offsetPos := len(args)

	q := fmt.Sprintf(`
		SELECT count(*) OVER() AS total_records,
		       id, project_id, created_by, description, amount, created_at, paid_at
		FROM batches
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d;
	`, strings.Join(where, " AND "), limitPos, offsetPos)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var (
		out          []*Batch
		totalRecords int
	)

	for rows.Next() {
		var b Batch
		if err := rows.Scan(
			&totalRecords,
			&b.ID, &b.ProjectID, &b.CreatedBy, &b.Description, &b.Amount, &b.CreatedAt, &b.PaidAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, &b)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, totalRecords, nil
}

func (s *PostgresBatchStore) ListItems(ctx context.Context, batchID int64) ([]*BatchItem, error) {
	const q = `
		SELECT batch_id, session_id, created_at
		FROM batch_items
		WHERE batch_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := s.db.QueryContext(ctx, q, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*BatchItem
	for rows.Next() {
		var it BatchItem
		if err := rows.Scan(&it.BatchID, &it.SessionID, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &it)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *PostgresBatchStore) MarkPaid(ctx context.Context, batchID int64, paidAt time.Time) error {
	const q = `
		UPDATE batches
		SET paid_at = $1
		WHERE id = $2 AND paid_at IS NULL;
	`

	res, err := s.db.ExecContext(ctx, q, paidAt, batchID)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound // not found OR already paid
	}

	return nil
}

func (s *PostgresBatchStore) ListItemDetails(ctx context.Context, batchID int64) ([]*BatchItemDetail, error) {
	const q = `
		SELECT
			bi.batch_id,
			bi.session_id,

			u.id   AS user_id,
			u.name AS user_name,

			ws.start_at,
			ws.end_at,

			COALESCE(EXTRACT(EPOCH FROM (ws.end_at - ws.start_at))::BIGINT, 0) AS seconds,
			COALESCE(ws.note, '') AS note
		FROM batch_items bi
		JOIN work_sessions ws ON ws.id = bi.session_id
		JOIN users u ON u.id = ws.user_id
		WHERE bi.batch_id = $1
		ORDER BY ws.start_at ASC, ws.id ASC;
	`

	rows, err := s.db.QueryContext(ctx, q, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*BatchItemDetail
	for rows.Next() {
		var it BatchItemDetail
		if err := rows.Scan(
			&it.BatchID,
			&it.SessionID,
			&it.UserID,
			&it.UserName,
			&it.StartAt,
			&it.EndAt,
			&it.Seconds,
			&it.Note,
		); err != nil {
			return nil, err
		}
		out = append(out, &it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}