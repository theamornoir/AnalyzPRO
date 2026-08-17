package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/theamornoir/analyzpro/internal/app"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("не удалось загрузить конфигурацию", "error", err)
		os.Exit(1)
	}

	// Централизованная настройка slog с уровнем из LOG_LEVEL (по умолчанию
	// INFO). Перенаправляет и стандартный пакет log на slog, поэтому все
	// log.Printf в проекте становятся slog-логами с учётом уровня.
	logging.SetupLogging(cfg.LogLevel)

	application, err := app.New()
	if err != nil {
		slog.Error("не удалось создать приложение", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info(locales.LogAppStarting)

	application.Run(ctx)
}
