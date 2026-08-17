// Package logging централизует настройку логирования приложения.
//
// Все модули проекта (исторически) используют стандартный пакет log
// (log.Printf и т.п.). Чтобы «перейти на slog» без правки десятков файлов и
// риска регрессий, SetupLogging:
//  1. создаёт slog-логгер с уровнем из LOG_LEVEL (DEBUG/INFO/WARN/ERROR);
//  2. делает его глобальным slog-логгером (slog.SetDefault);
//  3. перенаправляет стандартный пакет log на slog через log.SetOutput,
//     поэтому ВСЕ существующие log.Printf штатно идут через slog с учётом
//     выбранного уровня.
package logging

import (
	"log"
	"log/slog"
	"os"
	"strings"
)

// logWriter перенаправляет вывод стандартного пакета log в slog.
// Уровень стандартного log считаем INFO (log.Print*/log.Fatal без уровня);
// log.Fatal при этом сохраняет выход через os.Exit(1) самим пакетом log.
type logWriter struct {
	logger *slog.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	// log добавляет trailing newline - убираем, чтобы slog не дублировал.
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	w.logger.Info(msg)
	return len(p), nil
}

// SetupLogging инициализирует глобальный slog-логгер с уровнем level
// (DEBUG/INFO/WARN/ERROR; неизвестное/пустое → INFO) и перенаправляет
// стандартный пакет log на slog. Идемпотентна - повторный вызов безопасен.
func SetupLogging(level string) {
	var l slog.Level
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		l = slog.LevelDebug
	case "WARN", "WARNING":
		l = slog.LevelWarn
	case "ERROR":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Перенаправляем стандартный log на тот же slog-обработчик: все
	// log.Printf/log.Println в коде проекта становятся slog-логами с тем же
	// уровнем. log.SetDefault не существует в стандартной библиотеке, поэтому
	// используем log.SetOutput + адаптер-писатель.
	log.SetOutput(&logWriter{logger: logger})
}
