package sqlrepo

// Package sqlrepo - реализация хранилища пользователей/диагнозов/курсов/
// предпочтений поверх *sql.DB. Один тип repo реализует все четыре интерфейса
// из storage/interfaces. Драйвер-агностичен: SQL пишется в стиле SQLite (?),
// а BindQuery (db.BindQuery) преобразует ? -> $N для PostgreSQL. Поэтому
// один и тот же код работает и с локальной SQLite, и с Yandex Cloud PG.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

type Repo struct {
	db     *sql.DB
	driver string // "sqlite" | "postgres"
}

// New создаёт SQL-реализацию хранилища. driver опционален (по умолчанию
// "sqlite") - это сохраняет обратную совместимость с тестами, которые
// передают только *sql.DB.
func New(dbConn *sql.DB, driver ...string) *Repo {
	d := "sqlite"
	if len(driver) > 0 && driver[0] == "postgres" {
		d = "postgres"
	}
	return &Repo{db: dbConn, driver: d}
}

// bq адаптирует SQL-запрос под текущий драйвер (для postgres ? -> $N).
func (r *Repo) bq(q string) string { return db.BindQuery(r.driver, q) }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// ---------------------------------------------------------------
// UserRepository
// ---------------------------------------------------------------

func (r *Repo) CreateUser(ctx context.Context, user *sm.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	if user.LastActivityDate.IsZero() {
		user.LastActivityDate = time.Now()
	}

	const q = `INSERT INTO users (telegram_id, name, is_premium, premium_expires_at, onboarding_completed, last_activity_date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(telegram_id) DO UPDATE SET name=excluded.name, onboarding_completed=excluded.onboarding_completed`
	if _, err := r.db.ExecContext(ctx, r.bq(q), user.TelegramID, user.Name, boolToInt(user.IsPremium), nullTime(user.PremiumExpiresAt), boolToInt(user.OnboardingCompleted), nullTime(user.LastActivityDate), user.CreatedAt); err != nil {
		return fmt.Errorf("создание пользователя: %w", err)
	}
	var id int64
	if err := r.db.QueryRowContext(ctx, r.bq(`SELECT id FROM users WHERE telegram_id = ?`), user.TelegramID).Scan(&id); err != nil {
		return fmt.Errorf("получение id пользователя: %w", err)
	}
	user.ID = uint(id)
	return nil
}

func (r *Repo) GetUserByTelegramID(ctx context.Context, telegramID int64) (*sm.User, error) {
	const q = `SELECT id, telegram_id, name, is_premium, premium_expires_at, onboarding_completed, last_activity_date, created_at, tariff_id
		FROM users WHERE telegram_id = ?`
	var u sm.User
	var isPremium int
	var expires sql.NullTime
	var onboardingCompleted int
	var lastActivity sql.NullTime
	var tariffID sql.NullString
	if err := r.db.QueryRowContext(ctx, r.bq(q), telegramID).Scan(&u.ID, &u.TelegramID, &u.Name, &isPremium, &expires, &onboardingCompleted, &lastActivity, &u.CreatedAt, &tariffID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("чтение пользователя: %w", err)
	}
	u.IsPremium = isPremium != 0
	if expires.Valid {
		u.PremiumExpiresAt = expires.Time
	}
	if tariffID.Valid {
		u.TariffID = tariffID.String
	}
	u.OnboardingCompleted = onboardingCompleted != 0
	if lastActivity.Valid {
		u.LastActivityDate = lastActivity.Time
	}
	return &u, nil
}

// GetAllUsers возвращает всех пользователей (для периодических
// напоминаний/рассылок). Используется системой уведомлений.
func (r *Repo) GetAllUsers(ctx context.Context) ([]*sm.User, error) {
	const q = `SELECT id, telegram_id, name, is_premium, premium_expires_at, onboarding_completed, last_activity_date, created_at, tariff_id
		FROM users ORDER BY id`
	rows, err := r.db.QueryContext(ctx, r.bq(q))
	if err != nil {
		return nil, fmt.Errorf("чтение пользователей: %w", err)
	}
	defer rows.Close()

	out := make([]*sm.User, 0)
	for rows.Next() {
		var u sm.User
		var isPremium int
		var expires sql.NullTime
		var onboardingCompleted int
		var lastActivity sql.NullTime
		var tariffID sql.NullString
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Name, &isPremium, &expires, &onboardingCompleted, &lastActivity, &u.CreatedAt, &tariffID); err != nil {
			return nil, fmt.Errorf("сканирование пользователя: %w", err)
		}
		u.IsPremium = isPremium != 0
		if expires.Valid {
			u.PremiumExpiresAt = expires.Time
		}
		if tariffID.Valid {
			u.TariffID = tariffID.String
		}
		u.OnboardingCompleted = onboardingCompleted != 0
		if lastActivity.Valid {
			u.LastActivityDate = lastActivity.Time
		}
		cp := u
		out = append(out, &cp)
	}
	return out, rows.Err()
}

func (r *Repo) UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time, tariffID string) error {
	res, err := r.db.ExecContext(ctx,
		r.bq(`UPDATE users SET is_premium = ?, premium_expires_at = ?, tariff_id = ? WHERE id = ?`),
		boolToInt(isPremium), nullTime(expiresAt), tariffID, userID)
	if err != nil {
		return fmt.Errorf("обновление статуса premium: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user with ID %d not found", userID)
	}
	return nil
}

// UpdateUserOnboardingStatus обновляет флаг прохождения онбординга.
func (r *Repo) UpdateUserOnboardingStatus(ctx context.Context, userID uint, completed bool) error {
	res, err := r.db.ExecContext(ctx,
		r.bq(`UPDATE users SET onboarding_completed = ? WHERE id = ?`),
		boolToInt(completed), userID)
	if err != nil {
		return fmt.Errorf("обновление статуса онбординга: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user with ID %d not found", userID)
	}
	return nil
}

// UpdateUserLastActivity обновляет дату последнего взаимодействия
// пользователя с ботом (система напоминаний об неактивности).
func (r *Repo) UpdateUserLastActivity(ctx context.Context, userID uint, t time.Time) error {
	res, err := r.db.ExecContext(ctx,
		r.bq(`UPDATE users SET last_activity_date = ? WHERE id = ?`),
		nullTime(t), userID)
	if err != nil {
		return fmt.Errorf("обновление даты активности: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user with ID %d not found", userID)
	}
	return nil
}

// IsPromoCodeUsed проверяет, активировал ли пользователь промокод.
// userID здесь - Telegram chat ID (int64).
func (r *Repo) IsPromoCodeUsed(ctx context.Context, userID int64, code string) bool {
	var dummy int
	const q = `SELECT 1 FROM used_promocodes WHERE user_id = ? AND code = ? LIMIT 1`
	if err := r.db.QueryRowContext(ctx, r.bq(q), userID, code).Scan(&dummy); err != nil {
		return false
	}
	return true
}

// MarkPromoCodeUsed помечает промокод использованным (UNIQUE(user_id, code)
// делает вставку идемпотентной - повторный вызов не дублирует запись).
func (r *Repo) MarkPromoCodeUsed(ctx context.Context, userID int64, code string) error {
	if _, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO used_promocodes (user_id, code, used_at) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, code) DO NOTHING`),
		userID, code, time.Now()); err != nil {
		return fmt.Errorf("отметка промокода использованным: %w", err)
	}
	return nil
}

// DeleteAccount полностью удаляет пользователя и все связанные данные
// (анализы, курсы, предпочтения) по Telegram ID. Дочерние таблицы
// (diagnoses/cycles/preferences) ключуются внутренним users.id, поэтому
// сначала получаем внутренний ID, удаляем детей, затем саму запись users.
func (r *Repo) DeleteAccount(ctx context.Context, telegramID int64) error {
	u, err := r.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		// Пользователя и так нет - считаем удалённым.
		return nil
	}
	userID := u.ID

	for _, q := range []string{
		`DELETE FROM diagnoses WHERE user_id = ?`,
		`DELETE FROM cycles WHERE user_id = ?`,
		`DELETE FROM preferences WHERE user_id = ?`,
		`DELETE FROM users WHERE id = ?`,
	} {
		if _, derr := r.db.ExecContext(ctx, r.bq(q), userID); derr != nil {
			return fmt.Errorf("удаление аккаунта: %w", derr)
		}
	}
	return nil
}

// ---------------------------------------------------------------
// DiagnosisRepository
// ---------------------------------------------------------------

func (r *Repo) SaveDiagnosis(ctx context.Context, diagnosis *sm.Diagnosis) error {
	if diagnosis == nil {
		return fmt.Errorf("diagnosis is nil")
	}
	if diagnosis.Date.IsZero() {
		diagnosis.Date = time.Now()
	}
	// PostgreSQL не поддерживает LastInsertId - используем RETURNING id.
	if r.driver == "postgres" {
		const q = `INSERT INTO diagnoses (user_id, date, type, json_data, report_html)
			VALUES (?, ?, ?, ?, ?) RETURNING id`
		var id int64
		if err := r.db.QueryRowContext(ctx, r.bq(q),
			diagnosis.UserID, diagnosis.Date, diagnosis.Type, diagnosis.JsonData, diagnosis.ReportHTML,
		).Scan(&id); err != nil {
			return fmt.Errorf("сохранение диагноза: %w", err)
		}
		diagnosis.ID = uint(id)
		return nil
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO diagnoses (user_id, date, type, json_data, report_html) VALUES (?, ?, ?, ?, ?)`),
		diagnosis.UserID, diagnosis.Date, diagnosis.Type, diagnosis.JsonData, diagnosis.ReportHTML)
	if err != nil {
		return fmt.Errorf("сохранение диагноза: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		diagnosis.ID = uint(id)
	}
	return nil
}

func (r *Repo) GetAllDiagnosesByUserID(ctx context.Context, userID uint, limit, offset int) ([]sm.Diagnosis, error) {
	q := `SELECT id, user_id, date, type, json_data, report_html FROM diagnoses WHERE user_id = ? ORDER BY date DESC`
	// Пагинация: при limit>0 добавляем LIMIT/OFFSET (index idx_diagnoses_date
	// покрывает сортировку). limit<=0 - без ограничения (обратная совместимость).
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}
	rows, err := r.db.QueryContext(ctx, r.bq(q), userID)
	if err != nil {
		return nil, fmt.Errorf("чтение диагнозов: %w", err)
	}
	defer rows.Close()

	out := make([]sm.Diagnosis, 0)
	for rows.Next() {
		var d sm.Diagnosis
		if err := rows.Scan(&d.ID, &d.UserID, &d.Date, &d.Type, &d.JsonData, &d.ReportHTML); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) GetLastDiagnosisByType(ctx context.Context, userID uint, diagnosisType string) (*sm.Diagnosis, error) {
	const q = `SELECT id, user_id, date, type, json_data, report_html
		FROM diagnoses WHERE user_id = ? AND type = ? ORDER BY date DESC LIMIT 1`
	var d sm.Diagnosis
	if err := r.db.QueryRowContext(ctx, r.bq(q), userID, diagnosisType).Scan(&d.ID, &d.UserID, &d.Date, &d.Type, &d.JsonData, &d.ReportHTML); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("diagnosis not found")
		}
		return nil, fmt.Errorf("чтение последнего диагноза: %w", err)
	}
	cp := d
	return &cp, nil
}

// ---------------------------------------------------------------
// CycleRepository
// ---------------------------------------------------------------

func (r *Repo) CreateCycle(ctx context.Context, cycle *sm.Cycle) error {
	if cycle == nil {
		return fmt.Errorf("cycle is nil")
	}
	if cycle.StartDate.IsZero() {
		cycle.StartDate = time.Now()
	}
	markers, _ := json.Marshal(cycle.TrackedMarkers)
	// PostgreSQL не поддерживает LastInsertId - используем RETURNING id.
	if r.driver == "postgres" {
		const q = `INSERT INTO cycles (user_id, name, start_date, end_date, tracked_markers)
			VALUES (?, ?, ?, ?, ?) RETURNING id`
		var id int64
		if err := r.db.QueryRowContext(ctx, r.bq(q),
			cycle.UserID, cycle.Name, cycle.StartDate, nullTime(cycle.EndDate), string(markers),
		).Scan(&id); err != nil {
			return fmt.Errorf("создание курса: %w", err)
		}
		cycle.ID = uint(id)
		return nil
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO cycles (user_id, name, start_date, end_date, tracked_markers) VALUES (?, ?, ?, ?, ?)`),
		cycle.UserID, cycle.Name, cycle.StartDate, nullTime(cycle.EndDate), string(markers))
	if err != nil {
		return fmt.Errorf("создание курса: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		cycle.ID = uint(id)
	}
	return nil
}

func (r *Repo) GetActiveCycleByUserID(ctx context.Context, userID uint) (*sm.Cycle, error) {
	// end_date - NULL означает «курс ещё не завершён». Сравнение с '' не
	// используем (в PostgreSQL TIMESTAMP с пустой строкой не сравнить).
	const q = `SELECT id, user_id, name, start_date, end_date, tracked_markers
		FROM cycles WHERE user_id = ? AND end_date IS NULL ORDER BY start_date DESC LIMIT 1`
	var c sm.Cycle
	var markers string
	if err := r.db.QueryRowContext(ctx, r.bq(q), userID).Scan(&c.ID, &c.UserID, &c.Name, &c.StartDate, &nullTimeScanner{tp: &c.EndDate}, &markers); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("active cycle not found")
		}
		return nil, fmt.Errorf("чтение активного курса: %w", err)
	}
	_ = json.Unmarshal([]byte(markers), &c.TrackedMarkers)
	cp := c
	return &cp, nil
}

func (r *Repo) CompleteCycle(ctx context.Context, cycleID uint, endDate time.Time) error {
	if endDate.IsZero() {
		endDate = time.Now()
	}
	res, err := r.db.ExecContext(ctx, r.bq(`UPDATE cycles SET end_date = ? WHERE id = ?`), endDate, cycleID)
	if err != nil {
		return fmt.Errorf("завершение курса: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("cycle with ID %d not found", cycleID)
	}
	return nil
}

// nullTimeScanner - адаптер sql.Scanner для *time.Time (допускает NULL).
type nullTimeScanner struct {
	tp *time.Time
}

func (n *nullTimeScanner) Scan(src interface{}) error {
	if src == nil {
		*n.tp = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*n.tp = v
	case string:
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			*n.tp = t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			*n.tp = t
		}
	case []byte:
		if t, err := time.Parse("2006-01-02 15:04:05", string(v)); err == nil {
			*n.tp = t
		}
	}
	return nil
}

// ---------------------------------------------------------------
// PreferenceRepository
// ---------------------------------------------------------------

func (r *Repo) GetPreferences(ctx context.Context, userID uint) (*sm.Preference, error) {
	const q = `SELECT user_id, reminder_frequency, units, notifications_enabled
		FROM preferences WHERE user_id = ?`
	var p sm.Preference
	var notif int
	if err := r.db.QueryRowContext(ctx, r.bq(q), userID).Scan(&p.UserID, &p.ReminderFrequency, &p.Units, &notif); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("preferences not found")
		}
		return nil, fmt.Errorf("чтение предпочтений: %w", err)
	}
	p.NotificationsEnabled = notif != 0
	cp := p
	return &cp, nil
}

func (r *Repo) UpdatePreferences(ctx context.Context, preferences *sm.Preference) error {
	if preferences == nil {
		return fmt.Errorf("preferences is nil")
	}
	_, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO preferences (user_id, reminder_frequency, units, notifications_enabled)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   reminder_frequency=excluded.reminder_frequency,
		   units=excluded.units,
		   notifications_enabled=excluded.notifications_enabled`),
		preferences.UserID, preferences.ReminderFrequency, preferences.Units, boolToInt(preferences.NotificationsEnabled))
	if err != nil {
		return fmt.Errorf("обновление предпочтений: %w", err)
	}
	return nil
}

// compile-time проверки, что repo реализует все интерфейсы.
var (
	_ interfaces.UserRepository       = (*Repo)(nil)
	_ interfaces.DiagnosisRepository  = (*Repo)(nil)
	_ interfaces.CycleRepository      = (*Repo)(nil)
	_ interfaces.PreferenceRepository = (*Repo)(nil)
)

// UpdateUserPremiumStatusByTelegramID обновляет Premium по Telegram ID
// (используется вебхуком YooKassa). Не зависит от внутреннего id.
func (r *Repo) UpdateUserPremiumStatusByTelegramID(ctx context.Context, telegramID int64, isPremium bool, expiresAt time.Time, tariffID string) error {
	res, err := r.db.ExecContext(ctx,
		r.bq(`UPDATE users SET is_premium = ?, premium_expires_at = ?, tariff_id = ? WHERE telegram_id = ?`),
		boolToInt(isPremium), nullTime(expiresAt), tariffID, telegramID)
	if err != nil {
		return fmt.Errorf("обновление статуса premium по telegram_id: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user with telegram_id %d not found", telegramID)
	}
	return nil
}
