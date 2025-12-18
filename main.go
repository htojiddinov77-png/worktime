package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/app"
	"github.com/htojiddinov77-png/worktime/internal/router"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 4000, "backend port")
	flag.Parse()

	app, err := app.NewApplication()
	if err != nil {
		panic(err)
	}

	routes := router.SetUpRoutes(app)


	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: routes,
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app.Logger.Printf("We are running on port %d", port)

	err = server.ListenAndServe()
	if err != nil {
		app.Logger.Fatal(err)
	}
}