package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/pressly/goose/v3"
)

func Open() (*sql.DB, error) {
	dsn := os.Getenv("WORKTIME_DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("WORKTIME_DB_DSN is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open %w", err)
	}
	
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("db: ping %w", err)
	}
	

	fmt.Println("Connected to database....")
	return db, nil
}

func MigrateFs(db *sql.DB, migrationsFS fs.FS, dir string) error {
	goose.SetBaseFS(migrationsFS)
	defer func() {
		goose.SetBaseFS(nil)
	}()
	return Migrate(db, dir)
}

func Migrate(db *sql.DB, dir string) error {
	err := goose.SetDialect("postgres") // specify database
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	err = goose.Up(db, dir)
	if err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}