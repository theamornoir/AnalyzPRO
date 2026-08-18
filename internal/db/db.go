// Package db предоставляет единый доступ к реальной СУБД (SQLite через
// modernc.org/sqlite - чистый Go, без CGO, работает в статическом бинаре и
// в Docker). Бот принудительно запускается в единственном экземпляре
// (flock в app.go), поэтому SQLite-файл безопасен для прод-деплоя.
//
// Чтобы перейти на Postgres, достаточно добавить драйвер (например
// github.com/lib/pq) и менять только Open(): sql.Open("postgres", dsn).
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // регистрирует драйвер "sqlite"
)

// Open открывает (или создаёт) базу данных по DSN. Для SQLite dsn - это путь
// к файлу (по умолчанию "./data/analyzpro.db"). Допустимы также спец-пути
// "file::memory:?cache=shared" для тестов.
func Open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = "./data/analyzpro.db"
	}

	// Для файлового SQLite создаём директорию и добавляем прагмы для
	// устойчивости к параллельным запросам из разных горутин.
	if isFileDSN(dsn) {
		if dir := filepath.Dir(dsn); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("не удалось создать каталог БД: %w", err)
			}
		}
		dsn = dsn + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(0)&_pragma=synchronous(NORMAL)"
	}

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть БД: %w", err)
	}
	// Проверяем соединение сразу, чтобы ошибка конфигурации БД не всплыла
	// позже в рантайме.
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("не удалось подключиться к БД: %w", err)
	}
	conn.SetMaxOpenConns(1) // SQLite + один инстанс бота: сериализуем запись
	return conn, nil
}

// isFileDSN - true, если dsn похож на путь к файлу, а не на ":memory:" или URL.
func isFileDSN(dsn string) bool {
	if dsn == "" {
		return true
	}
	if dsn == ":memory:" {
		return false
	}
	if len(dsn) > 5 && (dsn[:5] == "file:" || dsn[:5] == "http:" || dsn[:5] == "postg") {
		return false
	}
	return true
}

// Migrate создаёт все таблицы, если их ещё нет. Идемпотентно - безопасно
// вызывать при каждом старте.
func Migrate(conn *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			is_premium INTEGER NOT NULL DEFAULT 0,
			premium_expires_at DATETIME,
			onboarding_completed INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS diagnoses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			date DATETIME NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			json_data TEXT NOT NULL DEFAULT '',
			report_html TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_user ON diagnoses(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_date ON diagnoses(date)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_user_type ON diagnoses(user_id, type)`,
		`CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)`,
		`CREATE TABLE IF NOT EXISTS cycles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			start_date DATETIME NOT NULL,
			end_date DATETIME,
			tracked_markers TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS preferences (
			user_id INTEGER PRIMARY KEY,
			reminder_frequency TEXT NOT NULL DEFAULT '',
			units TEXT NOT NULL DEFAULT '',
			notifications_enabled INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS monitoring_projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			start_date DATETIME NOT NULL,
			end_date DATETIME,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user ON monitoring_projects(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_created ON monitoring_projects(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user_type ON monitoring_projects(telegram_id, type)`,

		`CREATE TABLE IF NOT EXISTS monitoring_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			date DATETIME NOT NULL,
			json_data TEXT NOT NULL DEFAULT '',
			report_html TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_history_user ON monitoring_history(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_history_date ON monitoring_history(date)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_history_user_type ON monitoring_history(telegram_id, type)`,
		`CREATE TABLE IF NOT EXISTS monitoring_project_entries (
			project_id INTEGER NOT NULL,
			entry_id INTEGER NOT NULL,
			PRIMARY KEY (project_id, entry_id)
		)`,
		`CREATE TABLE IF NOT EXISTS used_promocodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			code TEXT NOT NULL,
			used_at DATETIME NOT NULL,
			UNIQUE(user_id, code)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_used_promocodes_user ON used_promocodes(user_id)`,
	}

	for _, stmt := range schema {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("ошибка миграции: %w", err)
		}
	}

	// Для уже существующих баз (до добавления онбординга) добавляем
	// столбец onboarding_completed идемпотентно. Если столбец уже есть -
	// SELECT завершится успешно и ALTER не выполнится. ВАЖНО: возвращаемый
	// *sql.Rows обязательно закрываем, иначе соединение (SetMaxOpenConns(1))
	// «утечёт» и следующая операция с БД зависнет (deadlock пула).
	rows, qerr := conn.Query("SELECT onboarding_completed FROM users LIMIT 0")
	if qerr != nil {
		if _, aerr := conn.Exec("ALTER TABLE users ADD COLUMN onboarding_completed INTEGER NOT NULL DEFAULT 0"); aerr != nil {
			return fmt.Errorf("ошибка миграции (onboarding_completed): %w", aerr)
		}
	} else {
		rows.Close()
	}

	// Для баз, созданных до появления системы уведомлений, добавляем
	// столбец last_activity_date идемпотентно (тот же паттерн с
	// обязательным rows.Close()).
	rows2, qerr2 := conn.Query("SELECT last_activity_date FROM users LIMIT 0")
	if qerr2 != nil {
		if _, aerr := conn.Exec("ALTER TABLE users ADD COLUMN last_activity_date DATETIME"); aerr != nil {
			return fmt.Errorf("ошибка миграции (last_activity_date): %w", aerr)
		}
	} else {
		rows2.Close()
	}

	return nil
}
