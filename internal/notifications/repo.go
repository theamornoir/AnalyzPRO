package notifications

import (
	"context"
	"database/sql"
	"time"
)

// repo - минимальный доступ к таблицам subscription_notifications и
// notification_suppressions. Запросы используют QueryRowContext/ExecContext
// (без незакрытых *sql.Rows), чтобы не допускать утечки соединений из
// пула (см. правило в internal/db: SetMaxOpenConns(8), утечка → deadlock).
type repo struct {
	db *sql.DB
}

func newRepo(db *sql.DB) *repo {
	return &repo{db: db}
}

// hasSubscriptionNotification возвращает true, если уведомление заданного
// типа (days_before ∈ {7,3,1,0}) уже отправлялось этому пользователю.
// UNIQUE(telegram_id, days_before) гарантирует, что одно и то же
// напоминание не уйдёт дважды.
func (r *repo) hasSubscriptionNotification(ctx context.Context, telegramID int64, daysBefore int) (bool, error) {
	const q = `SELECT 1 FROM subscription_notifications WHERE telegram_id = ? AND days_before = ?`
	var dummy int
	err := r.db.QueryRowContext(ctx, q, telegramID, daysBefore).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// recordSubscriptionNotification логирует отправку уведомления об окончании
// подписки. INSERT OR IGNORE защищает от дубля при повторном прогоне
// (UNIQUE(telegram_id, days_before)).
func (r *repo) recordSubscriptionNotification(ctx context.Context, telegramID int64, daysBefore int, sentAt time.Time) error {
	const q = `INSERT OR IGNORE INTO subscription_notifications (telegram_id, days_before, sent_at) VALUES (?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, q, telegramID, daysBefore, sentAt); err != nil {
		return err
	}
	return nil
}

// isSuppressed возвращает true, если для пары (telegram_id, indicator) ещё
// не истёк срок подавления (suppressed_until > now). При отсутствии записи
// - false (уведомление по этому показателю ещё не отправлялось).
func (r *repo) isSuppressed(ctx context.Context, telegramID int64, indicator string, now time.Time) (bool, error) {
	const q = `SELECT suppressed_until FROM notification_suppressions WHERE telegram_id = ? AND indicator = ?`
	var until time.Time
	err := r.db.QueryRowContext(ctx, q, telegramID, indicator).Scan(&until)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return now.Before(until), nil
}

// suppress устанавливает (или продлевает upsert'ом) срок подавления по
// показателю indicator до момента until. UNIQUE(telegram_id, indicator)
// делает вставку идемпотентной.
func (r *repo) suppress(ctx context.Context, telegramID int64, indicator string, until time.Time) error {
	const q = `INSERT INTO notification_suppressions (telegram_id, indicator, suppressed_until) VALUES (?, ?, ?)
		ON CONFLICT(telegram_id, indicator) DO UPDATE SET suppressed_until = excluded.suppressed_until`
	if _, err := r.db.ExecContext(ctx, q, telegramID, indicator, until); err != nil {
		return err
	}
	return nil
}

// clearSuppression удаляет запись подавления по показателю (для dev-сброса).
func (r *repo) clearSuppression(ctx context.Context, telegramID int64, indicator string) error {
	const q = `DELETE FROM notification_suppressions WHERE telegram_id = ? AND indicator = ?`
	if _, err := r.db.ExecContext(ctx, q, telegramID, indicator); err != nil {
		return err
	}
	return nil
}
