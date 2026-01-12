package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/app"
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
			r.Post("/login/", app.TokenHandler.LoginHandler)     // checked
			r.Post("/reset-password/", app.UserHandler.HandleResetPassword) //checked
		})

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(app.Middleware.Authenticate)

			r.Get("/projects", app.ProjectHandler.HandleListProjects) // checked

			// Work sessions
			r.Route("/work-sessions", func(r chi.Router){
				r.Post("/start/", app.WorkSessionHandler.HandleStartSession)
				r.Patch("/stop/{id}/", app.WorkSessionHandler.HandleStopSession)
				r.Get("/list/", app.WorkSessionHandler.HandleListSessions)
			})

			r.Patch("/users/{id}/", app.UserHandler.HandleUpdateUser)
	
			// Admin-only
			r.Group(func(r chi.Router) {
				r.Post("/admin/reset-tokens/", app.UserHandler.HandleGenerateResetToken) 
				r.Get("/admin/users/", app.UserHandler.HandleListUsers) 
				r.Post("/projects/", app.ProjectHandler.HandleCreateProject) 
			})
		})
	})

	return r
}
