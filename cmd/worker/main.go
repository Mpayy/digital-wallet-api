package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mpayy/digital-wallet-api/internal/pkg/queue"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, cleanup, err := InitializeWorker()
	if err != nil {
		log.Fatalf("failed to initialize worker: %v", err)
	}
	defer cleanup()

	if err := queue.SetupTopology(app.App.RabbitMQ, queue.KnownQueues); err != nil {
		app.App.Log.Errorf("failed to setup topology: %v", err)
		return
	}

	if err := app.Worker.Run(ctx); err != nil {
		app.App.Log.Errorf("worker stopped with error: %v", err)
		return
	}

	app.App.Log.Info("worker shut down gracefully")
}
