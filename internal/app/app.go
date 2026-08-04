package app

import (
	"log"

	"github.com/theamornoir/analyzpro/internal/config"
)

type App struct {
	cfg *config.Config
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	app := &App{
		cfg: cfg,
	}

	return app, nil
}

func (a *App) Run() {
	log.Println("AnalyzPro started")
}
