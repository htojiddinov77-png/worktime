package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/app"
)

func SetUpRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register/", app.UserHandler.HandleRegister)
			r.Post("/login/", app.TokenHandler.LoginHandler)
			r.Post("/reset-password/{token}", app.ResetTokenHandler.HandleResetPassword)
		})

		r.Group(func(r chi.Router) {
			r.Use(app.Middleware.Authenticate)
			r.Get("/events/", app.WorkSessionHandler.ServeSSE)

			r.Get("/statuses/", app.StatusHandler.HandleGetAllStatuses)
			r.Get("/projects/", app.ProjectHandler.HandleListProjects)
			r.Patch("/project/{id}/", app.ProjectHandler.HandleUpdateProject)

			r.Route("/work-sessions", func(r chi.Router) {
				r.Post("/start/", app.WorkSessionHandler.HandleStartSession)
				r.Patch("/stop/{id}/", app.WorkSessionHandler.HandleStopSession)
				r.Get("/list/", app.WorkSessionHandler.HandleListSessions)
				r.Get("/batch-candidates/", app.WorkSessionHandler.HandleListBatchCandidates)
				r.Get("/reports/", app.WorkSessionHandler.HandleGetSummaryReport)
			})

			r.Route("/batches", func(r chi.Router) {
				r.Post("/", app.BatchHandler.HandleCreateBatch)
				r.Get("/", app.BatchHandler.HandleListBatches)

				r.Get("/{id}/", app.BatchHandler.HandleListBatchItems)
				r.Patch("/{id}/pay/", app.BatchHandler.HandleMarkBatchPaid)
			})

			r.Patch("/users/{id}/", app.UserHandler.HandleUpdateUser)
			r.Post("/admin/reset-tokens/", app.ResetTokenHandler.HandleGenerateResetLink)
			r.Get("/admin/users/", app.UserHandler.HandleListUsers)
			r.Post("/projects/", app.ProjectHandler.HandleCreateProject)

		})
	})

	return r
}
