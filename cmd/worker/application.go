package main

import (
	"github.com/Mpayy/digital-wallet-api/internal/config"
)

type ApplicationWorker struct {
	App    *config.WorkerInfra
	Worker *Worker
}

func NewApplicationWorker(app *config.WorkerInfra, worker *Worker) *ApplicationWorker {
	return &ApplicationWorker{App: app, Worker: worker}
}
