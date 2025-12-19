package app

import (
	"database/sql"
	"log"
	"os"

	"github.com/htojiddinov77-png/worktime/internal/api"
	"github.com/htojiddinov77-png/worktime/internal/store"
)

type Application struct {
	Logger *log.Logger
	UserHandler *api.UserHandler
	DB *sql.DB
}

func NewApplication() (*Application, error) {
	pgDB, err := store.Open()
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	userStore := store.NewPostgresUserStore(pgDB)

	userHandler := api.NewUserHandler(userStore, logger)
	app := &Application{
		Logger: logger,
		UserHandler: userHandler,
	}

	return app, nil
}