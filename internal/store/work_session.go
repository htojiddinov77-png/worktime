package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PostgresWorkSessionStore struct {
	db *sql.DB
}

func NewPostgresWorkSessionStore(db *sql.DB) *PostgresWorkSessionStore {
	return &PostgresWorkSessionStore{db: db}
}

type WorkSession struct {
	Id        int64      `json:"id"`
	UserId    int64      `json:"user_id"`
	ProjectId int64      `json:"project_id"`
	StartAt   time.Time  `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Note      string     `json:"note"`
	CreatedAt time.Time  `json:"created_at"`
}

type WorkSessionRow struct {
	WorkSession
	ProjectName   string `json:"project_name"`
	UserName      string `json:"user_name"`
	UserEmail     string `json:"user_email"`
	DerivedStatus string `json:"status"` // "active" if end_at IS NULL else "inactive"
}

// Book-ish filter inputs.
// Note: keep pointers for optional filters.
type WorkSessionFilter struct {
	// Ownership / filters
	UserID    *int64
	ProjectID *int64

	// If set:
	//   true  => only active (end_at IS NULL)
	//   false => only inactive (end_at IS NOT NULL)
	Active *bool

	// Start date range filter on start_at
	StartFrom *time.Time
	StartTo   *time.Time

	// Optional text search (matches project name OR user name/email OR note)
	Search *string

	// Book-style pagination + sorting
	Page     int
	PageSize int
	Sort     string // e.g. "start_at", "-start_at", "created_at", "-created_at", "id", "-id"
}

type DailySummary struct {
	Date          string `json:"date"`
	UserID        int64  `json:"user_id"`
	TotalSessions int    `json:"total_sessions"`
	TotalMinutes  int    `json:"total_minutes"`
}

type SummaryRangeFilter struct {
	UserID    *int64
	ProjectID *int64
	FromDate  time.Time // date (YYYY-MM-DD) parsed -> any time ok
	ToDate    time.Time // date (YYYY-MM-DD)
}

type ReportUser struct {
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

type OverallSummary struct {
	TotalSessions int `json:"total_sessions"`
	TotalMinutes  int `json:"total_minutes"`
}

type ProjectSummary struct {
	ProjectID     int64  `json:"project_id"`
	ProjectName   string `json:"project_name"`
	TotalSessions int    `json:"total_sessions"`
	TotalMinutes  int    `json:"total_minutes"`
}

type SummaryFilters struct {
	UserID    *int64 `json:"user_id"`
	ProjectID *int64 `json:"project_id"`
}

type SummaryReport struct {
	From    string         `json:"from"`
	To      string         `json:"to"`
	Filters SummaryFilters `json:"filters"`

	// Only set when reporting for a single user (non-admin or admin + user_id)
	User *ReportUser `json:"user,omitempty"`

	Overall   OverallSummary   `json:"overall"`
	ByProject []ProjectSummary `json:"by_project"`
}

type WorkSessionStore interface {
	StartSession(*WorkSession) error
	StopSession(sessionID int64, userID int64) error
	GetSummaryReport(filter SummaryRangeFilter) (*SummaryReport, error)

	// book-style: return total records too (for pagination metadata)
	ListSessions(filter WorkSessionFilter) ([]WorkSessionRow, int, error)
}

func (pg *PostgresWorkSessionStore) StartSession(ws *WorkSession) error {
	query := `
		INSERT INTO work_sessions (user_id, project_id, note, start_at, created_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, start_at, created_at;
	`

	return pg.db.QueryRow(query, ws.UserId, ws.ProjectId, ws.Note).
		Scan(&ws.Id, &ws.StartAt, &ws.CreatedAt)
}

func (pg *PostgresWorkSessionStore) StopSession(sessionID int64, userID int64) error {
	query := `
		UPDATE work_sessions
		SET end_at = NOW()
		WHERE id = $1 AND user_id = $2 AND end_at IS NULL
	`

	result, err := pg.db.Exec(query, sessionID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (pg *PostgresWorkSessionStore) ListSessions(filter WorkSessionFilter) ([]WorkSessionRow, int, error) {
	// Defaults (book-ish)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 200 {
		filter.PageSize = 50
	}

	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize

	// SAFE sort allowlist (book rule)
	sortSQL := "ws.start_at DESC, ws.id DESC"
	switch filter.Sort {
	case "start_at":
		sortSQL = "ws.start_at ASC, ws.id ASC"
	case "-start_at", "":
		sortSQL = "ws.start_at DESC, ws.id DESC"
	case "created_at":
		sortSQL = "ws.created_at ASC, ws.id ASC"
	case "-created_at":
		sortSQL = "ws.created_at DESC, ws.id DESC"
	case "id":
		sortSQL = "ws.id ASC"
	case "-id":
		sortSQL = "ws.id DESC"
	default:
		// fallback safe
		sortSQL = "ws.start_at DESC, ws.id DESC"
	}

	base := `
		SELECT
			COUNT(*) OVER() AS total_records,
			ws.id,
			ws.user_id,
			ws.project_id,
			ws.start_at,
			ws.end_at,
			ws.note,
			ws.created_at,
			p.name AS project_name,
			u.name AS user_name,
			u.email AS user_email,
			CASE WHEN ws.end_at IS NULL THEN 'active' ELSE 'inactive' END AS status
		FROM work_sessions ws
		JOIN projects p ON ws.project_id = p.id
		JOIN users u ON ws.user_id = u.id
		WHERE 1=1
	`

	var (
		sb   strings.Builder
		args []any
		pos  = 1
	)

	sb.WriteString(base)

	add := func(cond string, v any) {
		sb.WriteString(fmt.Sprintf(" AND "+cond, pos))
		args = append(args, v)
		pos++
	}

	if filter.UserID != nil {
		add("ws.user_id = $%d", *filter.UserID)
	}
	if filter.ProjectID != nil {
		add("ws.project_id = $%d", *filter.ProjectID)
	}
	if filter.Active != nil {
		if *filter.Active {
			sb.WriteString(" AND ws.end_at IS NULL")
		} else {
			sb.WriteString(" AND ws.end_at IS NOT NULL")
		}
	}
	if filter.StartFrom != nil {
		add("ws.start_at >= $%d", *filter.StartFrom)
	}
	if filter.StartTo != nil {
		add("ws.start_at <= $%d", *filter.StartTo)
	}
	if filter.Search != nil {
		s := strings.TrimSpace(*filter.Search)
		if s != "" {
			like := "%" + s + "%"
			sb.WriteString(fmt.Sprintf(`
				AND (
					p.name ILIKE $%d OR
					u.name ILIKE $%d OR
					u.email ILIKE $%d OR
					COALESCE(ws.note,'') ILIKE $%d
				)
			`, pos, pos+1, pos+2, pos+3))
			args = append(args, like, like, like, like)
			pos += 4
		}
	}

	sb.WriteString(" ORDER BY " + sortSQL)
	sb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", pos, pos+1))
	args = append(args, limit, offset)

	rows, err := pg.db.Query(sb.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]WorkSessionRow, 0, limit)
	total := 0

	for rows.Next() {
		var (
			row          WorkSessionRow
			note         sql.NullString
			totalRecords int
		)

		err := rows.Scan(
			&totalRecords,
			&row.Id,
			&row.UserId,
			&row.ProjectId,
			&row.StartAt,
			&row.EndAt,
			&note,
			&row.CreatedAt,
			&row.ProjectName,
			&row.UserName,
			&row.UserEmail,
			&row.DerivedStatus,
		)
		if err != nil {
			return nil, 0, err
		}

		if note.Valid {
			row.Note = note.String
		} else {
			row.Note = ""
		}

		total = totalRecords
		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func (pg *PostgresWorkSessionStore) GetSummaryReport(filter SummaryRangeFilter) (*SummaryReport, error) {
	// Normalize dates to [fromStart, toEnd) in UTC
	fromStart := time.Date(filter.FromDate.Year(), filter.FromDate.Month(), filter.FromDate.Day(), 0, 0, 0, 0, time.UTC)
	toStart := time.Date(filter.ToDate.Year(), filter.ToDate.Month(), filter.ToDate.Day(), 0, 0, 0, 0, time.UTC)
	toEnd := toStart.Add(24 * time.Hour)

	if !toEnd.After(fromStart) {
		return nil, fmt.Errorf("invalid date range: to must be >= from")
	}

	rep := &SummaryReport{
		From: fromStart.Format("2006-01-02"),
		To:   toStart.Format("2006-01-02"),
		Filters: SummaryFilters{
			UserID:    filter.UserID,
			ProjectID: filter.ProjectID,
		},
		ByProject: []ProjectSummary{},
	}

	// Optional user info for single-user report
	if filter.UserID != nil {
		var u ReportUser
		err := pg.db.QueryRow(`
			SELECT id, name, email
			FROM users
			WHERE id = $1
		`, *filter.UserID).Scan(&u.UserID, &u.UserName, &u.UserEmail)
		if err != nil {
			return nil, err
		}
		rep.User = &u
	}

	// Overall summary
	overallSQL := `
		SELECT
			COUNT(*) AS total_sessions,
			COALESCE(SUM(
				GREATEST(0, EXTRACT(EPOCH FROM (COALESCE(ws.end_at, NOW()) - ws.start_at)) / 60)
			), 0)::int AS total_minutes
		FROM work_sessions ws
		WHERE ws.start_at >= $1 AND ws.start_at < $2
	`

	args := []any{fromStart, toEnd}
	pos := 3

	if filter.UserID != nil {
		overallSQL += fmt.Sprintf(" AND ws.user_id = $%d", pos)
		args = append(args, *filter.UserID)
		pos++
	}
	if filter.ProjectID != nil {
		overallSQL += fmt.Sprintf(" AND ws.project_id = $%d", pos)
		args = append(args, *filter.ProjectID)
		pos++
	}

	if err := pg.db.QueryRow(overallSQL, args...).Scan(&rep.Overall.TotalSessions, &rep.Overall.TotalMinutes); err != nil {
		return nil, err
	}

	// By-project summary
	byProjectSQL := `
		SELECT
			p.id AS project_id,
			p.name AS project_name,
			COUNT(*) AS total_sessions,
			COALESCE(SUM(
				GREATEST(0, EXTRACT(EPOCH FROM (COALESCE(ws.end_at, NOW()) - ws.start_at)) / 60)
			), 0)::int AS total_minutes
		FROM work_sessions ws
		JOIN projects p ON p.id = ws.project_id
		WHERE ws.start_at >= $1 AND ws.start_at < $2
	`

	args2 := []any{fromStart, toEnd}
	pos2 := 3

	if filter.UserID != nil {
		byProjectSQL += fmt.Sprintf(" AND ws.user_id = $%d", pos2)
		args2 = append(args2, *filter.UserID)
		pos2++
	}
	if filter.ProjectID != nil {
		byProjectSQL += fmt.Sprintf(" AND ws.project_id = $%d", pos2)
		args2 = append(args2, *filter.ProjectID)
		pos2++
	}

	byProjectSQL += `
		GROUP BY p.id, p.name
		ORDER BY total_minutes DESC, p.id ASC
	`

	rows, err := pg.db.Query(byProjectSQL, args2...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ps ProjectSummary
		if err := rows.Scan(&ps.ProjectID, &ps.ProjectName, &ps.TotalSessions, &ps.TotalMinutes); err != nil {
			return nil, err
		}
		rep.ByProject = append(rep.ByProject, ps)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rep, nil
}
