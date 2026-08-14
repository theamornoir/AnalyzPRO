package sqlrepo

// Package sqlrepo — реализация хранилища пользователей/диагнозов/курсов/
// предпочтений поверх *sql.DB (SQLite через modernc). Один тип repo
// реализует все четыре интерфейса из storage/interfaces, как и file.Store,
// поэтому Storage может использовать его без изменения вызывающих.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

type Repo struct {
	db *sql.DB
}

// New создаёт SQL-реализацию хранилища.
func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

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

	const q = `INSERT INTO users (telegram_id, name, is_premium, premium_expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(telegram_id) DO UPDATE SET name=excluded.name`
	if _, err := r.db.ExecContext(ctx, q, user.TelegramID, user.Name, boolToInt(user.IsPremium), nullTime(user.PremiumExpiresAt), user.CreatedAt); err != nil {
		return fmt.Errorf("создание пользователя: %w", err)
	}
	var id int64
	if err := r.db.QueryRowContext(ctx, `SELECT id FROM users WHERE telegram_id = ?`, user.TelegramID).Scan(&id); err != nil {
		return fmt.Errorf("получение id пользователя: %w", err)
	}
	user.ID = uint(id)
	return nil
}

func (r *Repo) GetUserByTelegramID(ctx context.Context, telegramID int64) (*sm.User, error) {
	const q = `SELECT id, telegram_id, name, is_premium, premium_expires_at, created_at
		FROM users WHERE telegram_id = ?`
	var u sm.User
	var isPremium int
	var expires sql.NullTime
	if err := r.db.QueryRowContext(ctx, q, telegramID).Scan(&u.ID, &u.TelegramID, &u.Name, &isPremium, &expires, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("чтение пользователя: %w", err)
	}
	u.IsPremium = isPremium != 0
	if expires.Valid {
		u.PremiumExpiresAt = expires.Time
	}
	return &u, nil
}

func (r *Repo) UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_premium = ?, premium_expires_at = ? WHERE id = ?`,
		boolToInt(isPremium), nullTime(expiresAt), userID)
	if err != nil {
		return fmt.Errorf("обновление статуса premium: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("user with ID %d not found", userID)
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
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO diagnoses (user_id, date, type, json_data, report_html) VALUES (?, ?, ?, ?, ?)`,
		diagnosis.UserID, diagnosis.Date, diagnosis.Type, diagnosis.JsonData, diagnosis.ReportHTML)
	if err != nil {
		return fmt.Errorf("сохранение диагноза: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		diagnosis.ID = uint(id)
	}
	return nil
}

func (r *Repo) GetAllDiagnosesByUserID(ctx context.Context, userID uint) ([]sm.Diagnosis, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, date, type, json_data, report_html FROM diagnoses WHERE user_id = ? ORDER BY date DESC`, userID)
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
	if err := r.db.QueryRowContext(ctx, q, userID, diagnosisType).Scan(&d.ID, &d.UserID, &d.Date, &d.Type, &d.JsonData, &d.ReportHTML); err != nil {
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
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO cycles (user_id, name, start_date, end_date, tracked_markers) VALUES (?, ?, ?, ?, ?)`,
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
	const q = `SELECT id, user_id, name, start_date, end_date, tracked_markers
		FROM cycles WHERE user_id = ? AND (end_date IS NULL OR end_date = '') ORDER BY start_date DESC LIMIT 1`
	var c sm.Cycle
	var markers string
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.StartDate, &nullTimeScanner{tp: &c.EndDate}, &markers); err != nil {
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
	res, err := r.db.ExecContext(ctx, `UPDATE cycles SET end_date = ? WHERE id = ?`, endDate, cycleID)
	if err != nil {
		return fmt.Errorf("завершение курса: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("cycle with ID %d not found", cycleID)
	}
	return nil
}

// nullTimeScanner — адаптер sql.Scanner для *time.Time (допускает NULL).
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
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&p.UserID, &p.ReminderFrequency, &p.Units, &notif); err != nil {
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
		`INSERT INTO preferences (user_id, reminder_frequency, units, notifications_enabled)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   reminder_frequency=excluded.reminder_frequency,
		   units=excluded.units,
		   notifications_enabled=excluded.notifications_enabled`,
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
