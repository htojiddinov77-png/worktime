package store

import (
	"context"
	"database/sql"
)

type Status struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

type PostgresStatusStore struct {
	db *sql.DB
}

func NewPostgresStatusStore(db *sql.DB) *PostgresStatusStore {
	return &PostgresStatusStore{
		db: db,
	}
}

type StatusStore interface {
	GetStatusbyId(id int64) (*Status, error)
	GetAllStatuses(ctx context.Context) ([]*Status, error)
}

func (pg *PostgresStatusStore) GetStatusbyId(id int64) (*Status, error) {
	status := &Status{}
	query := `SELECT id, name FROM statuses WHERE id = $1`
	row := pg.db.QueryRow(query, id)
	err := row.Scan(&status.Id, &status.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (pg *PostgresStatusStore) GetAllStatuses(ctx context.Context) ([]*Status, error) {
	query := `SELECT id, name FROM statuses ORDER BY id`

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*Status
	for rows.Next() {
		status := &Status{}
		if err := rows.Scan(&status.Id, &status.Name); err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statuses, nil
}


