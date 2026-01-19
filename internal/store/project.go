package store

import (
	"context"
	"database/sql"
	"time"
)

type PostgresProjectStore struct {
	db *sql.DB
}

func NewPostgresProjectStore(db *sql.DB) *PostgresProjectStore {
	return &PostgresProjectStore{db: db}
}

type ProjectStatus struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type ProjectRow struct {
	Id     int64         `json:"-"`
	Name   string        `json:"name"`
	Status ProjectStatus `json:"status"`

	TotalSeconds   int64  `json:"-"`
	TotalDurations string `json:"total_durations"`

	ActiveSessions []ActiveSessionRow `json:"active_sessions"`
}

type Project struct {
	ProjectId   int64  `json:"project_id"`
	ProjectName string `json:"project_name"`
	StatusId    int64  `json:"status_id"`
}

type ActiveUser struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ActiveSessionRow struct {
	ProjectId int64 `json:"id"`
	User ActiveUser `json:"user"`
	StartedAt time.Time `json:"start_at"`
	ActiveSeconds int64 `json:"-"`
	ActiveMinutes int64 `json:"active_minutes"`	
}


type ProjectStore interface {
	CreateProject(ctx context.Context, project *Project) error
	ListProjects(ctx context.Context) ([]ProjectRow, error)
	ListActiveSessions(ctx context.Context) ([]ActiveSessionRow, error)
	UpdateProject(ctx context.Context, id int64, name *string, statusID *int64) error
}

func (pg *PostgresProjectStore) CreateProject(ctx context.Context, project *Project) error {
	query := `
	INSERT into projects (name, status_id)
	VALUES($1, $2)
	RETURNING id`

	err := pg.db.QueryRowContext(ctx, query, project.ProjectName, project.StatusId).Scan(&project.ProjectId)
	if err != nil {
		return err
	}

	return nil
}

func (pg *PostgresProjectStore) ListProjects(ctx context.Context) ([]ProjectRow, error) {
	query := `
		SELECT
			p.id,
			p.name AS name,
			s.id,
			s.name,
			COALESCE(
				SUM(EXTRACT(EPOCH FROM (ws.end_at - ws.start_at))),0)::bigint AS total_seconds
		FROM projects p
		JOIN statuses s ON p.status_id = s.id
		LEFT JOIN work_sessions ws
			ON ws.project_id = p.id
			AND ws.end_at IS NOT NULL
		GROUP BY p.id, p.name, s.id, s.name
		ORDER BY p.name ASC, p.id ASC
	`

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProjectRow

	for rows.Next() {
		var p ProjectRow
		err := rows.Scan(
			&p.Id,
			&p.Name,
			&p.Status.Id,
			&p.Status.Name,
			&p.TotalSeconds,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (pg *PostgresProjectStore) UpdateProject(ctx context.Context, id int64, name *string, statusID *int64) error {
	query := `
		UPDATE projects
		SET
			name      = COALESCE($1, name),
			status_id = COALESCE($2, status_id)
		WHERE id = $3
	`

	res, err := pg.db.ExecContext(ctx, query, name, statusID, id)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	return nil
}

func (pg *PostgresProjectStore) ListActiveSessions(ctx context.Context) ([]ActiveSessionRow, error) {
	query := `
	SELECT 
		ws.project_id,
		
		u.id,
		u.name,
		u.email,
		ws.start_at,
		
		EXTRACT(EPOCH FROM (NOW() - start_at))::bigint AS active_seconds
		FROM work_sessions ws
		JOIN users u on u.id = ws.user_id
		WHERE ws.end_at IS NULL
		ORDER BY ws.project_id, start_at`

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []ActiveSessionRow

	for rows.Next() {
		var a ActiveSessionRow
		err := rows.Scan(
			&a.ProjectId,
			&a.User.Id,
			&a.User.Name,
			&a.User.Email,
			&a.StartedAt,
			&a.ActiveSeconds,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, err
}
