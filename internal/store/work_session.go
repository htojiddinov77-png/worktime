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
	return &PostgresWorkSessionStore{
		db: db,
	}
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

	// Optional text search (matches project name OR user name/email)
	Search *string

	// Pagination (optional)
	Limit  int
	Offset int
}

type DailySummary struct {
    Date          string `json:"date"` 
    UserID        int64  `json:"user_id"`
    TotalSessions int    `json:"total_sessions"`
    TotalMinutes  int    `json:"total_minutes"`
}


type WorkSessionStore interface {
	StartSession(*WorkSession) error
	StopSession(id int64) error
	GetDailySummary(userID *int64, date time.Time) ([]DailySummary, error)
	ListSessions(filter WorkSessionFilter) ([]WorkSessionRow, error)
}

func (pg *PostgresWorkSessionStore) StartSession(worksession *WorkSession) error {
	query := `
	INSERT INTO work_sessions(user_id, project_id, note, start_at, created_at)
	VALUES($1, $2, $3, NOW(), NOW())
	RETURNING id, start_at, created_at;
 `

	err := pg.db.QueryRow(query, worksession.UserId, worksession.ProjectId, worksession.Note).Scan(&worksession.Id, &worksession.StartAt, &worksession.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (pg *PostgresWorkSessionStore) StopSession(id int64) error {
	query := `
	UPDATE work_sessions
	SET end_at = NOW()
	WHERE user_id = $1 AND end_at IS NULL`

	result, err := pg.db.Exec(query, id)
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

func (pg *PostgresWorkSessionStore) ListSessions(filter WorkSessionFilter) ([]WorkSessionRow, error) {

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var sb strings.Builder
	sb.WriteString(`
	SELECT ws.id,ws.user_id, ws.project_id, ws.start_at, ws.end_at,
	COALESCE(ws.note, '') AS note,ws.created_at,
	COALESCE(p.name, '') AS project_name,
	COALESCE(u.name, '') AS user_name,
	COALESCE(u.email, '') AS user_email,
	CASE WHEN ws.end_at IS NULL THEN 'active' ELSE 'inactive' END AS status
	FROM work_sessions ws
	JOIN projects p ON ws.project_id = p.id
	JOIN users u ON ws.user_id = u.id
	WHERE 1=1`)

	args := []any{}
	argPos := 1

	addCond := func(cond string, val any) {
		sb.WriteString(" AND ")
		sb.WriteString(fmt.Sprintf(cond, argPos))
		args = append(args, val)
		argPos++
	}

	if filter.UserID != nil {
		addCond("ws.user_id = $%d", *filter.UserID)
	}

	if filter.ProjectID != nil {
		addCond("ws.project_id = $%d", *filter.ProjectID)
	}

	if filter.Active != nil {
		if *filter.Active {
			sb.WriteString(" AND ws.end_at IS NULL")
		} else {
			sb.WriteString(" AND ws.end_at IS NOT NULL")
		}
	}

	if filter.StartFrom != nil {
		addCond("ws.start_at >= $%d", *filter.StartFrom)
	}

	if filter.StartTo != nil {
		addCond("ws.start_at <= $%d", *filter.StartTo)
	}

	if filter.Search != nil {
		s := strings.TrimSpace(*filter.Search)
		if s != "" {
			like := "%" + s + "%"
			sb.WriteString(fmt.Sprintf(`
				AND (
					p.name ILIKE $%d
					OR u.name ILIKE $%d
					OR u.email ILIKE $%d
				)
			`, argPos, argPos+1, argPos+2))
			args = append(args, like, like, like)
			argPos += 3
		}
	}

	sb.WriteString(" ORDER BY ws.start_at DESC, ws.id DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1))
	args = append(args, limit, filter.Offset)

	rows, err := pg.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkSessionRow
	for rows.Next() {
		var row WorkSessionRow

		err := rows.Scan(
			&row.Id,
			&row.UserId,
			&row.ProjectId,
			&row.StartAt,
			&row.EndAt,
			&row.Note,
			&row.CreatedAt,
			&row.ProjectName,
			&row.UserName,
			&row.UserEmail,
			&row.DerivedStatus,
		)
		if err != nil {
			return nil, err
		}

		out = append(out, row)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (pg *PostgresWorkSessionStore) GetDailySummary(userID *int64, date time.Time) ([]DailySummary, error) {
    dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
    dayEnd := dayStart.Add(24 * time.Hour)

    // We count sessions that start within the day.
    // Minutes = difference between end_at and start_at; if end_at is null, use NOW().
    base := `
        SELECT
            to_char($1::timestamptz, 'YYYY-MM-DD') AS day,
            ws.user_id,
            COUNT(*) AS total_sessions,
            COALESCE(SUM(
                GREATEST(0, EXTRACT(EPOCH FROM (COALESCE(ws.end_at, NOW()) - ws.start_at)) / 60)
            ), 0)::int AS total_minutes
        FROM work_sessions ws
        WHERE ws.start_at >= $1 AND ws.start_at < $2
    `
    args := []any{dayStart, dayEnd}
    argPos := 3

    if userID != nil {
        base += fmt.Sprintf(" AND ws.user_id = $%d", argPos)
        args = append(args, *userID)
        argPos++
    }

    base += `
        GROUP BY ws.user_id
        ORDER BY ws.user_id ASC
    `

    rows, err := pg.db.Query(base, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []DailySummary
    for rows.Next() {
        var s DailySummary
        if err := rows.Scan(&s.Date, &s.UserID, &s.TotalSessions, &s.TotalMinutes); err != nil {
            return nil, err
        }
        out = append(out, s)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return out, nil
}
