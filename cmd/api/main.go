package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
)

// @title           Digital Wallet API
// @version         1.0
// @description     Digital wallet built with Go, Gin, GORM, and Redis.
// @BasePath        /api/v1
// @securitydefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Type "Bearer" followed by a space and JWT token.
// @contact.name               Achmad Rifai
// @contact.url                https://github.com/Mpayy
// @license.name               MIT
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, cleanup, err := InitializeAPI()
	if err != nil {
		log.Fatalf("Failed to initialize API: %v", err)
	}
	defer cleanup()

	app := application.App
	router := application.Router

	err = queue.SetupTopology(app.RabbitMQ, queue.KnownQueues)
	if err != nil {
		app.Log.Errorf("Failed to setup topology: %v", err)
		return
	}

	router.Setup()

	host := app.Config.GetString("APP_HOST")
	port := app.Config.GetInt("APP_PORT")
	addr := fmt.Sprintf("%s:%d", host, port)

	server := &http.Server{
		Addr:    addr,
		Handler: app.App,
	}

	go func() {
		app.Log.Infof("Server starting on: %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Log.Errorf("Failed to start server: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	app.Log.Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		app.Log.Errorf("Server forced to shutdown: %v", err)
	}
	app.Log.Infof("Server exited properly")
}
