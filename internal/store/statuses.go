package store

import "database/sql"

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

