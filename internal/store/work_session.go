package store

import (
	"context"
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

type UserResponse struct {
	UserId   int64  `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
}

type WorkSessionResponse struct {
	Id        int64      `json:"id"`
	StartAt   time.Time  `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Note      string     `json:"note"`
	CreatedAt time.Time  `json:"created_at"`
}

type WorkSessionRow struct {
	User    UserResponse        `json:"user"`
	Project ProjectRow          `json:"project"`
	Session WorkSessionResponse `json:"sessions"`

	DerivedStatus string `json:"status"` // "active" if end_at IS NULL else "inactive"
}

type WorkSessionFilter struct {
	Filter

	UserID    *int64
	ProjectID *int64
	Active    *bool
	Search    *string
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

	User      *ReportUser      `json:"user,omitempty"`
	Overall   OverallSummary   `json:"overall"`
	ByProject []ProjectSummary `json:"by_project"`
}

type WorkSessionStore interface {
	StartSession(ctx context.Context, ws *WorkSession) error
	StopSession(ctx context.Context, sessionID int64, userID int64) error
	GetSummaryReport(ctx context.Context, filter SummaryRangeFilter) (*SummaryReport, error)

	// book-style: return total records too (for pagination metadata)
	ListSessions(ctx context.Context, filter WorkSessionFilter) ([]WorkSessionRow, int, error)
}

func (pg *PostgresWorkSessionStore) StartSession(ctx context.Context, ws *WorkSession) error {
	query := `
		INSERT INTO work_sessions (user_id, project_id, note, start_at, created_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, start_at, created_at;
	`

	err := pg.db.QueryRowContext(ctx, query, ws.UserId, ws.ProjectId, ws.Note).
		Scan(&ws.Id, &ws.StartAt, &ws.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (pg *PostgresWorkSessionStore) StopSession(ctx context.Context, sessionID int64, userID int64) error {
	query := `
		UPDATE work_sessions
		SET end_at = NOW()
		WHERE id = $1 AND user_id = $2 AND end_at IS NULL
	`

	result, err := pg.db.ExecContext(ctx, query, sessionID, userID)
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

func (pg *PostgresWorkSessionStore) ListSessions(ctx context.Context, filter WorkSessionFilter) ([]WorkSessionRow, int, error) {
	limit := filter.Limit()
	offset := filter.Offset()

	userID := int64(0)
	if filter.UserID != nil {
		userID = *filter.UserID
	}

	projectID := int64(0)
	if filter.ProjectID != nil {
		projectID = *filter.ProjectID
	}

	search := ""
	if filter.Search != nil {
		search = strings.TrimSpace(*filter.Search)
	}

	active := ""
	if filter.Active != nil {
		if *filter.Active {
			active = "true"
		} else {
			active = "false"
		}
	}

	query := `
	SELECT
		COUNT(*) OVER() AS total_records,
		ws.id AS session_id,                    

		u.id      AS user_id,
		u.name    AS user_name,
		u.email   AS user_email,
		u.is_active AS user_is_active,

		p.id      AS project_id,
		p.name    AS project_name,
		COALESCE(s.id, 0)    AS project_status_id,
		COALESCE(s.name, '') AS project_status_name,


		ws.start_at,
		ws.end_at,
		COALESCE(ws.note, '') AS note,
		ws.created_at,

		CASE WHEN ws.end_at IS NULL THEN 'active' ELSE 'inactive' END AS status
	FROM work_sessions ws
	JOIN projects p ON p.id = ws.project_id
	JOIN users u ON u.id = ws.user_id
	LEFT JOIN statuses s ON s.id = p.status_id
	WHERE
		($1 = 0 OR ws.user_id = $1)
		AND ($2 = 0 OR ws.project_id = $2)
		AND (
			$3 = '' OR (
				p.name ILIKE $3 || '%%' OR
				u.name ILIKE $3 || '%%' OR
				u.email ILIKE $3 || '%%' OR
				COALESCE(ws.note,'') ILIKE $3 || '%%'
			)
		)
		AND (
			$4 = '' OR
			($4 = 'true'  AND ws.end_at IS NULL) OR
			($4 = 'false' AND ws.end_at IS NOT NULL)
		)
	ORDER BY ws.start_at DESC, ws.id DESC
	LIMIT $5 OFFSET $6;
`

	rows, err := pg.db.QueryContext(
		ctx,
		query,
		userID,
		projectID,
		search,
		active,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]WorkSessionRow, 0, limit)
	total := 0

	for rows.Next() {
		var (
			row          WorkSessionRow
			totalRecords int
		)

		if err := rows.Scan(
			&totalRecords,
			&row.Session.Id,

			&row.User.UserId,
			&row.User.Name,
			&row.User.Email,
			&row.User.IsActive,

			&row.Project.Id,
			&row.Project.Name,
			&row.Project.Status.Id,
			&row.Project.Status.Name,

			&row.Session.StartAt,
			&row.Session.EndAt,
			&row.Session.Note,
			&row.Session.CreatedAt,

			&row.DerivedStatus,
		); err != nil {
			return nil, 0, err
		}

		total = totalRecords
		out = append(out, row)
	}

	return out, total, rows.Err()
}


func (pg *PostgresWorkSessionStore) GetSummaryReport(ctx context.Context, filter SummaryRangeFilter) (*SummaryReport, error) {
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
	pos := 3 // third value

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
