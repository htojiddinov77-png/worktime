package store

import (
	"context"
	"database/sql"
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
	Id     int64         `json:"id"`
	Name   string        `json:"name"`
	Status ProjectStatus `json:"status"`
}

type Project struct {
	ProjectId   int64  `json:"project_id"`
	ProjectName string `json:"project_name"`
	StatusId    int64  `json:"status_id"`
}

type ProjectStore interface {
	CreateProject(ctx context.Context, project *Project) error
	ListProjects(ctx context.Context) ([]ProjectRow, error)
	UpdateProject(ctx context.Context, id int64, name *string, statusID *int64) error
	DeleteProject(ctx context.Context, id int64) error
}

func (pg PostgresProjectStore) CreateProject(ctx context.Context, project *Project) error {
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
			COALESCE(p.name, '') AS name,
			s.id,
			s.name
		FROM projects p
		JOIN statuses s ON p.status_id = s.id
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

func (pg *PostgresProjectStore) DeleteProject(ctx context.Context, id int64) error {
	query := `DELETE FROM projects
	WHERE id = $1`

	_, err := pg.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}
