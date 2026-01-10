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

	// Pagination: LIMIT is page size, OFFSET is how many rows to skip.
	limit := filter.Limit()
	offset := filter.Offset()

	// We pass optional filters as NULL when they are not provided.
	// In SQL we use: ($1 IS NULL OR column = $1)
	// so if the value is NULL -> filter is ignored.
	var userID any
	if filter.UserID != nil {
		userID = *filter.UserID
	}

	var projectID any
	if filter.ProjectID != nil {
		projectID = *filter.ProjectID
	}

	// Search is optional. If provided, we convert it into a pattern like: "%text%"
	// and use ILIKE for case-insensitive search across multiple columns.
	var search any
	if filter.Search != nil {
		s := strings.TrimSpace(*filter.Search)
		if s != "" {
			search = "%" + s + "%"
		}
	}

	// Active filter:
	// - nil   => don't filter by active/inactive
	// - true  => only sessions with end_at IS NULL
	// - false => only sessions with end_at IS NOT NULL
	var active any
	if filter.Active != nil {
		active = *filter.Active
	}

	query := `
		SELECT
			-- COUNT(*) OVER() returns total rows ignoring LIMIT/OFFSET.
			-- This allows us to return pagination metadata without running a second COUNT query.
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

			-- Derived status is calculated from end_at:
			-- end_at NULL => active, otherwise inactive.
			CASE WHEN ws.end_at IS NULL THEN 'active' ELSE 'inactive' END AS status
		FROM work_sessions ws
		JOIN projects p ON ws.project_id = p.id
		JOIN users u ON ws.user_id = u.id
		WHERE
			-- Optional user filter
			($1::bigint IS NULL OR ws.user_id = $1)

			-- Optional project filter
			AND ($2::bigint IS NULL OR ws.project_id = $2)

			-- Optional search filter (matches project/user/note)
			AND (
				$3::text IS NULL OR (
					p.name ILIKE $3 OR
					u.name ILIKE $3 OR
					u.email ILIKE $3 OR
					COALESCE(ws.note,'') ILIKE $3
				)
			)

			-- Optional active filter (true => end_at IS NULL, false => end_at IS NOT NULL)
			AND (
				$4::boolean IS NULL OR
				($4 = true  AND ws.end_at IS NULL) OR
				($4 = false AND ws.end_at IS NOT NULL)
			)

	
		ORDER BY ws.start_at DESC, ws.id DESC

		-- Pagination
		LIMIT $5 OFFSET $6
	`

	rows, err := pg.db.Query(
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

	// Preallocate slice capacity to avoid extra allocations.
	out := make([]WorkSessionRow, 0, limit)
	total := 0

	for rows.Next() {
		var (
			row          WorkSessionRow
			note         sql.NullString // ws.note can be NULL in DB
			totalRecords int            // same number for every row (from COUNT(*) OVER())
		)

		if err := rows.Scan(
			&totalRecords,       // total records for pagination
			&row.Id,
			&row.UserId,
			&row.ProjectId,
			&row.StartAt,
			&row.EndAt,
			&note,               // scan nullable note
			&row.CreatedAt,
			&row.ProjectName,
			&row.UserName,
			&row.UserEmail,
			&row.DerivedStatus,
		); err != nil {
			return nil, 0, err
		}

		// Convert NULL note to empty string in response.
		if note.Valid {
			row.Note = note.String
		} else {
			row.Note = ""
		}

		// totalRecords is identical for all rows; we just keep the last seen value.
		total = totalRecords
		out = append(out, row)
	}

	// rows.Err() reports any scan/iteration errors that happened during the loop.
	return out, total, rows.Err()
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
