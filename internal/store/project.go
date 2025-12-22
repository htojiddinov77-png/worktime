package store

import "database/sql"

type PostgresProjectStore struct {
	db *sql.DB
}

func NewPostgresProjectStore(db *sql.DB) *PostgresProjectStore {
	return &PostgresProjectStore{db: db}
}

type Project struct {
	Id       int64    `json:"id"`
	Name     string `json:"name"`
	StatusId int64   `json:"status_id"`
}

type ProjectStore interface {
	CreateProject(*Project) error
	ListProjects() ([]Project, error)
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

func (pg *PostgresProjectStore) ListProjects() ([]Project, error) {
    query := `
        SELECT id, name, status_id
        FROM projects
        ORDER BY name ASC, id ASC
    `
    rows, err := pg.db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []Project
    for rows.Next() {
        var p Project
        if err := rows.Scan(&p.Id, &p.Name, &p.StatusId); err != nil {
            return nil, err
        }
        out = append(out, p)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return out, nil
}



