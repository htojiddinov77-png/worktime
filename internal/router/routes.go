package router

import (

	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/app"
)

func SetUpRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Route("/v1", func(r chi.Router) {

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register/", app.UserHandler.HandleRegister)
			r.Post("/login/", app.TokenHandler.LoginHandler)
			r.Post("/reset-password/", app.UserHandler.HandleResetPassword)
		})
		
		r.Group(func(r chi.Router) {
			r.Use(app.Middleware.Authenticate)
			r.Get("/statuses/", app.StatusHandler.HandleGetAllStatuses)
			r.Get("/projects/", app.ProjectHandler.HandleListProjects)
			r.Patch("/project/{id}/", app.ProjectHandler.HandleUpdateProject)

			r.Route("/work-sessions", func(r chi.Router) {
				r.Post("/start/", app.WorkSessionHandler.HandleStartSession)
				r.Patch("/stop/{id}/", app.WorkSessionHandler.HandleStopSession)
				r.Get("/list/", app.WorkSessionHandler.HandleListSessions)
				r.Get("/reports/", app.WorkSessionHandler.HandleGetSummaryReport)
			})

			r.Patch("/users/{id}/", app.UserHandler.HandleUpdateUser)
			r.Post("/admin/reset-tokens/", app.UserHandler.HandleGenerateResetToken)
			r.Get("/admin/users/", app.UserHandler.HandleListUsers)
			r.Post("/projects/", app.ProjectHandler.HandleCreateProject)

		})
	})

	return r
}
