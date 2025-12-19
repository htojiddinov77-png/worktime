package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/htojiddinov77-png/worktime/internal/app"
)

func SetUpRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()

	r.Post("/users/register/", app.UserHandler.HandleRegister)
	
	return r
}