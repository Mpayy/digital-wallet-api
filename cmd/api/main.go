package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title           Digital Wallet API
// @version         1.0
// @description     Digital wallet built with Go, Gin, GORM, and Redis.
// @host            localhost:8080
// @BasePath        /api/v1
// @securitydefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Type "Bearer" followed by a space and JWT token.
// @contact.name               Achmad Rifai
// @contact.url                https://github.com/Mpayy
// @license.name               MIT
// @schemes                    http https
func main() {
	application := InitializeAPI()
	app := application.App
	router := application.Router

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
			app.Log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.Log.Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		app.Log.Fatalf("Server forced to shutdown: %v", err)
	}
	app.Log.Infof("Server exited properly")

	db, err := app.DB.DB()
	if err != nil {
		app.Log.Fatalf("Failed to get database connection: %v", err)
	}
	if err := db.Close(); err != nil {
		app.Log.Fatalf("Failed to close database connection: %v", err)
	}
	app.Log.Infof("Database connection closed")

	if err := app.Redis.Close(); err != nil {
		app.Log.Fatalf("Failed to close redis connection: %v", err)
	}
	app.Log.Infof("Redis connection closed")
}
