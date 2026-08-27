package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"  // регистрирует драйвер "postgres"
	_ "modernc.org/sqlite" // регистрирует драйвер "sqlite"

	"github.com/theamornoir/analyzpro/internal/config"
)

// Open открывает (или создаёт) локальную SQLite БД по DSN. Для SQLite dsn -
// это путь к файлу (по умолчанию "./data/analyzpro.db"). Допустимы также
// спец-пути "file::memory:?cache=shared" для тестов. Используется тестами и
// режимом по умолчанию. Для PostgreSQL используйте OpenConfig.
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
	// WAL-журнал (включён прагмой journal_mode(WAL)) разрешает
	// конкурентные чтения + ровно одну запись одновременно. Поднимаем
	// пул до 8 соединений. Бот в одном инстансе (flock в app.go), поэтому
	// файл БД остаётся безопасным для прод-деплоя.
	conn.SetMaxOpenConns(8)
	conn.SetMaxIdleConns(8)
	return conn, nil
}

// OpenConfig открывает СУБД согласно конфигурации. При DBDriver="postgres"
// подключается к управляемой PostgreSQL (Yandex Cloud) по TLS с пулом
// соединений, настроенным для работы через интернет (короткие lifetime,
// чтобы переживать разрывы TLS-сессий провайдером). Иначе делегирует в
// Open(DBPath) - локальная SQLite.
func OpenConfig(cfg *config.Config) (*sql.DB, error) {
	if cfg != nil && cfg.DBDriver == "postgres" {
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = fmt.Sprintf(
				"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s sslrootcert=%s",
				cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword,
				cfg.DBName, cfg.DBSSLMode, cfg.DBSSLRootCert,
			)
		}
		conn, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("не удалось открыть PostgreSQL: %w", err)
		}
		// Тюнинг пула для Managed PG через интернет: умеренный размер,
		// короткий lifetime соединений (TLS-сессии могут рваться у
		// провайдера), чтобы не копить «висящие» соединения.
		conn.SetMaxOpenConns(12)
		conn.SetMaxIdleConns(4)
		conn.SetConnMaxLifetime(30 * time.Minute)
		if err := conn.Ping(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("не удалось подключиться к PostgreSQL: %w", err)
		}
		return conn, nil
	}

	dsn := ""
	if cfg != nil {
		dsn = cfg.DBPath
	}
	return Open(dsn)
}

// BindQuery преобразует ?-плейсхолдеры в $N для драйвера postgres. Для
// остальных драйверов (sqlite) возвращает запрос без изменений. Позволяет
// писать SQL один раз в стиле sqlite и переиспользовать тот же код поверх
// обеих СУБД: репозитории оборачивают запросы через этот хелпер. Нумерация
// $N сквозная по всему запросу, порядок args должен совпадать.
func BindQuery(driver, query string) string {
	if driver != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
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
// вызывать при каждом старте. Принимает driver ("sqlite"|"postgres");
// не указан - считается sqlite (обратная совместимость с тестами). Диалект
// (AUTOINCREMENT/BIGSERIAL, DATETIME/TIMESTAMP) подбирается по driver.
func Migrate(conn *sql.DB, driver ...string) error {
	d := "sqlite"
	if len(driver) > 0 && driver[0] == "postgres" {
		d = "postgres"
	}

	mkPK := func() string {
		if d == "postgres" {
			return "BIGSERIAL PRIMARY KEY"
		}
		return "INTEGER PRIMARY KEY AUTOINCREMENT"
	}
	mkTS := func() string {
		if d == "postgres" {
			return "TIMESTAMP"
		}
		return "DATETIME"
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id ` + mkPK() + `,
			telegram_id BIGINT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			is_premium INTEGER NOT NULL DEFAULT 0,
			premium_expires_at ` + mkTS() + `,
			tariff_id TEXT NOT NULL DEFAULT '',
			onboarding_completed INTEGER NOT NULL DEFAULT 0,
			last_activity_date ` + mkTS() + `,
			created_at ` + mkTS() + ` NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS diagnoses (
			id ` + mkPK() + `,
			user_id INTEGER NOT NULL,
			date ` + mkTS() + ` NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			json_data TEXT NOT NULL DEFAULT '',
			report_html TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_user ON diagnoses(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_date ON diagnoses(date)`,
		`CREATE INDEX IF NOT EXISTS idx_diagnoses_user_type ON diagnoses(user_id, type)`,
		`CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_users_premium_expires_at ON users(premium_expires_at)`,
		// Постоянный профиль пользователя (имя/возраст/пол/рост/вес/цель).
		// Заполняется и обновляется после каждого опросника (Оценка здоровья,
		// Bioscan PRO), чтобы бот не переспрашивал уже известные данные.
		`CREATE TABLE IF NOT EXISTS user_profiles (
			id ` + mkPK() + `,
			telegram_id BIGINT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			age INTEGER NOT NULL DEFAULT 0,
			gender TEXT NOT NULL DEFAULT '',
			height INTEGER NOT NULL DEFAULT 0,
			weight INTEGER NOT NULL DEFAULT 0,
			goal TEXT NOT NULL DEFAULT '',
			updated_at ` + mkTS() + ` NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_profiles_telegram ON user_profiles(telegram_id)`,
		`CREATE TABLE IF NOT EXISTS cycles (
			id ` + mkPK() + `,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			start_date ` + mkTS() + ` NOT NULL,
			end_date ` + mkTS() + `,
			tracked_markers TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS preferences (
			user_id INTEGER PRIMARY KEY,
			reminder_frequency TEXT NOT NULL DEFAULT '',
			units TEXT NOT NULL DEFAULT '',
			notifications_enabled INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS monitoring_projects (
			id ` + mkPK() + `,
			telegram_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			start_date ` + mkTS() + ` NOT NULL,
			end_date ` + mkTS() + `,
			status TEXT NOT NULL DEFAULT 'active',
			created_at ` + mkTS() + ` NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user ON monitoring_projects(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_created ON monitoring_projects(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user_type ON monitoring_projects(telegram_id, type)`,
		`CREATE TABLE IF NOT EXISTS monitoring_history (
			id ` + mkPK() + `,
			telegram_id INTEGER NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			date ` + mkTS() + ` NOT NULL,
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
			id ` + mkPK() + `,
			user_id INTEGER NOT NULL,
			code TEXT NOT NULL,
			used_at ` + mkTS() + ` NOT NULL,
			UNIQUE(user_id, code)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_used_promocodes_user ON used_promocodes(user_id)`,
		`CREATE TABLE IF NOT EXISTS subscription_notifications (
			id ` + mkPK() + `,
			telegram_id INTEGER NOT NULL,
			days_before INTEGER NOT NULL,
			sent_at ` + mkTS() + ` NOT NULL,
			UNIQUE(telegram_id, days_before)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sub_notif_user ON subscription_notifications(telegram_id)`,
		`CREATE TABLE IF NOT EXISTS notification_suppressions (
			id ` + mkPK() + `,
			telegram_id INTEGER NOT NULL,
			indicator TEXT NOT NULL,
			suppressed_until ` + mkTS() + ` NOT NULL,
			UNIQUE(telegram_id, indicator)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notif_supp_user ON notification_suppressions(telegram_id)`,
	}

	for _, stmt := range schema {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("ошибка миграции: %w", err)
		}
	}

	// Для уже существующих баз (до добавления онбординга/активности/
	// тарифа) добавляем столбцы идемпотентно. Если столбец уже есть -
	// SELECT завершится успешно и ALTER не выполнится. ВАЖНО: возвращаемый
	// *sql.Rows обязательно закрываем, иначе соединение «утечёт» (deadlock
	// пула).
	addColIfMissing := func(col, typ string) error {
		rows, qerr := conn.Query("SELECT " + col + " FROM users LIMIT 0")
		if qerr != nil {
			if _, aerr := conn.Exec("ALTER TABLE users ADD COLUMN " + col + " " + typ); aerr != nil {
				return fmt.Errorf("ошибка миграции (%s): %w", col, aerr)
			}
		} else {
			rows.Close()
		}
		return nil
	}
	if err := addColIfMissing("onboarding_completed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColIfMissing("last_activity_date", mkTS()); err != nil {
		return err
	}
	if err := addColIfMissing("tariff_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if err := ensureNotificationSchema(conn, d); err != nil {
		return fmt.Errorf("ошибка миграции (уведомления): %w", err)
	}

	return nil
}

// ensureNotificationSchema приводит таблицы subscription_notifications и
// notification_suppressions к актуальной схеме. Если таблица уже имеет
// нужные столбцы (days_before / indicator) - ничего не делает. Если
// существует старая схема (kind / key) или таблицы нет вовсе - пересоздаёт
// (DROP + CREATE). Диалект (AUTOINCREMENT/BIGSERIAL, DATETIME/TIMESTAMP)
// подбирается по driver.
func ensureNotificationSchema(conn *sql.DB, driver string) error {
	pk := "INTEGER PRIMARY KEY AUTOINCREMENT"
	ts := "DATETIME"
	if driver == "postgres" {
		pk = "BIGSERIAL PRIMARY KEY"
		ts = "TIMESTAMP"
	}

	if _, err := conn.Exec("SELECT days_before FROM subscription_notifications LIMIT 0"); err != nil {
		if _, derr := conn.Exec("DROP TABLE IF EXISTS subscription_notifications"); derr != nil {
			return derr
		}
		if _, cerr := conn.Exec(`CREATE TABLE subscription_notifications (
			id ` + pk + `,
			telegram_id INTEGER NOT NULL,
			days_before INTEGER NOT NULL,
			sent_at ` + ts + ` NOT NULL,
			UNIQUE(telegram_id, days_before)
		)`); cerr != nil {
			return cerr
		}
		if _, ierr := conn.Exec("CREATE INDEX IF NOT EXISTS idx_sub_notif_user ON subscription_notifications(telegram_id)"); ierr != nil {
			return ierr
		}
	}

	if _, err := conn.Exec("SELECT indicator FROM notification_suppressions LIMIT 0"); err != nil {
		if _, derr := conn.Exec("DROP TABLE IF EXISTS notification_suppressions"); derr != nil {
			return derr
		}
		if _, cerr := conn.Exec(`CREATE TABLE notification_suppressions (
			id ` + pk + `,
			telegram_id INTEGER NOT NULL,
			indicator TEXT NOT NULL,
			suppressed_until ` + ts + ` NOT NULL,
			UNIQUE(telegram_id, indicator)
		)`); cerr != nil {
			return cerr
		}
		if _, ierr := conn.Exec("CREATE INDEX IF NOT EXISTS idx_notif_supp_user ON notification_suppressions(telegram_id)"); ierr != nil {
			return ierr
		}
	}

	return nil
}
