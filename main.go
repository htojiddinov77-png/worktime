package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/htojiddinov77-png/worktime/internal/app"
	"github.com/htojiddinov77-png/worktime/internal/middleware"
	"github.com/htojiddinov77-png/worktime/internal/router"
)

func main() {
	// Load .env (local development only)
	_ = godotenv.Load()

	// Default port from env
	envPort := 4000
	if p := os.Getenv("WORKTIME_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			envPort = parsed
		}
	}

	// CLI flag overrides env
	port := flag.Int("port", envPort, "backend port")
	flag.Parse()

	application, err := app.NewApplication()
	if err != nil {
		panic(err)
	}

	routes := router.SetUpRoutes(application)

	allowed := map[string]bool{
		"http://localhost:5173": true,
		"http://localhost:3000": true,
	}
	handler := middleware.CORS(allowed)(routes)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	application.Logger.Printf("Server running on port %d", *port)

	if err := server.ListenAndServe(); err != nil {
		application.Logger.Fatal(err)
	}
}
