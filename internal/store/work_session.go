package store

import "database/sql"

type PostgresWorktimeStore struct {
	db *sql.DB
}

func NewPostgresWorktimeStore(db *sql.DB) *PostgresWorktimeStore {
	return &PostgresWorktimeStore{
		db: db,
	}
}

type WorkSession struct {
	
}

type WorktimeStore interface {
	
}