package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	worker, err := InitializeWorker()
	if err != nil {
		log.Fatalf("failed to initialize worker: %v", err)
	}

	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("worker stopped with error: %v", err)
	}
}
