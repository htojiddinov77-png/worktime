package store

import (
	"database/sql"
	"time"
)

type PostgresWorkSessionStore struct {
	db *sql.DB
}

func NewPostgresWorktimeStore(db *sql.DB) *PostgresWorkSessionStore {
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

type WorkSessionStore interface {
	StartSession(*WorkSession) error
	StopSession(id int64) error
}

func (pg *PostgresWorkSessionStore) StartSession(worksession *WorkSession) error {
	query := `
	INSERT INTO work_sessions(user_id, project_id, note, start_at)
	VALUES($1, $2, $3, NOW())
	Returning id, start_at`

	err := pg.db.QueryRow(query, worksession.UserId, worksession.ProjectId, worksession.Note).Scan(&worksession.Id, &worksession.StartAt)
	if err != nil {
		return err
	}

	return nil
}

func (pg *PostgresWorkSessionStore) StopSession(id int64) error {
	query := `
	UPDATE work_sessions
	SET end_at = $1
	WHERE user_id = $2 AND end_at IS NULL`

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
