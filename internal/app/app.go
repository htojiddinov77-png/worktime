package app

import (
	"database/sql"
	"log"
	"os"

	"github.com/htojiddinov77-png/worktime/internal/api"
	"github.com/htojiddinov77-png/worktime/internal/auth"
	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/store"
)

type Application struct {
	Logger *log.Logger
	DB     *sql.DB

	UserHandler        *api.UserHandler
	WorkSessionHandler *api.WorkSessionHandler
	TokenHandler       *api.TokenHandler
	ProjectHandler     *api.ProjectHandler
	StatusHandler      *api.StatusHandler

	Middleware *middleware.Middleware
	JWT        *auth.JWTManager
}

func NewApplication() (*Application, error) {
	pgDB, err := store.Open()
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// Stores
	userStore := store.NewPostgresUserStore(pgDB)
	workSessionStore := store.NewPostgresWorkSessionStore(pgDB)
	projectStore := store.NewPostgresProjectStore(pgDB)
	statusStore := store.NewPostgresStatusStore(pgDB)

	// JWT manager (auth package)
	jwtManager := auth.NewJWTManager()

	// Handlers
	userHandler := api.NewUserHandler(userStore, logger, jwtManager)
	projectHandler := api.NewProjectHandler(projectStore, userStore, logger)
	workSessionHandler := api.NewWorkSessionHandler(workSessionStore, userStore, logger, middleware.Middleware{JWT: jwtManager})
	tokenHandler := api.NewTokenHandler(userStore, jwtManager, logger)
	statusHandler := api.NewStatusHandler(statusStore)

	// Middleware (depends on auth only)
	mw := &middleware.Middleware{JWT: jwtManager}

	app := &Application{
		Logger:             logger,
		DB:                 pgDB,
		UserHandler:        userHandler,
		WorkSessionHandler: workSessionHandler,
		ProjectHandler:     projectHandler,
		StatusHandler:      statusHandler,
		TokenHandler:       tokenHandler,
		Middleware:         mw,
		JWT:                jwtManager,
	}

	return app, nil
}
