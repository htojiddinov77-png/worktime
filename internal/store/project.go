package store

import "database/sql"

type PostgresProjectStore struct {
	db *sql.DB
}

type Project struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	StatusId int    `json:"status_id"`
}

type ProjectStore interface {
	CreateProject(*Project) error
}

func (pg PostgresProjectStore) CreateProject(project *Project) error {
	query := `
	INSERT into projects (name, status_id)
	VALUES($1, $2)
	RETURNING id`

	err := pg.db.QueryRow(query, project.Name, project.StatusId).Scan(&project.Id)
	if err != nil {
		return err
	}

	return nil
}



