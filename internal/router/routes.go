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
		//  Public 
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", app.UserHandler.HandleRegister)
			r.Post("/login", app.TokenHandler.LoginHandler)
		})

		//  Protected
		r.Group(func(r chi.Router) {
			r.Use(app.Middleware.Authenticate)

			// Work sessions
			r.Route("/work-sessions", func(r chi.Router) {
				// Start a new session (POST body: project_id, note)
				r.Post("/start", app.WorkSessionHandler.HandleStartSession)

				// Stop current active session
				r.Post("/stop", app.WorkSessionHandler.HandleStopSession)

				// List + filter + search (query params)
				r.Get("/", app.WorkSessionHandler.HandleListSessions)
			})

			r.Patch("/users/me/password", app.UserHandler.HandleChangePassword)

			// Admin-only 
			
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireAdmin)
					r.Patch("/admin/users/{id}/disable", app.UserHandler.HandleDisableUser)
					r.Post("/projects", app.ProjectHandler.HandleCreateProject)
				})
			
			
		})
	})

	return r
}

