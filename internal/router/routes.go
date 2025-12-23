package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/app"
	"github.com/htojiddinov77-png/worktime/internal/middleware"
)

func SetUpRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/v1", func(r chi.Router) {
		// Public
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register/", app.UserHandler.HandleRegister) // checked
			r.Post("/login/", app.TokenHandler.LoginHandler) // checked
		})

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(app.Middleware.Authenticate)

			r.Get("/reports/daily", app.WorkSessionHandler.HandleDailyReport) 
			r.Get("/projects", app.ProjectHandler.HandleListProjects)

			// Work sessions
			r.Route("/work-sessions", func(r chi.Router) {
				r.Post("/start/", app.WorkSessionHandler.HandleStartSession)
				r.Patch("/stop/{id}/", app.WorkSessionHandler.HandleStopSession)
				r.Get("/list/", app.WorkSessionHandler.HandleListSessions)
			})

			// User self
			r.Patch("/users/me/password/", app.UserHandler.HandleChangePassword)

			// Admin-only
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Get("/admin/users", app.UserHandler.HandleAdminListUsers)
				r.Patch("/admin/users/{id}/disable/", app.UserHandler.HandleDisableUser)
				r.Post("/projects/", app.ProjectHandler.HandleCreateProject) // checked
			})
		})
	})

	return r
}
