package notifications

import (
	"context"
	"database/sql"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
)

// repo - минимальный доступ к таблицам subscription_notifications и
// notification_suppressions. Запросы используют QueryRowContext/ExecContext
// (без незакрытых *sql.Rows), чтобы не допускать утечки соединений из
// пула (см. правило в internal/db: SetMaxOpenConns(8), утечка → deadlock).
type repo struct {
	db     *sql.DB
	driver string // "sqlite" | "postgres"
}

func newRepo(dbConn *sql.DB, driver string) *repo {
	return &repo{db: dbConn, driver: driver}
}

// bq адаптирует SQL-запрос под текущий драйвер (для postgres ? -> $N).
func (r *repo) bq(q string) string { return db.BindQuery(r.driver, q) }

// hasSubscriptionNotification возвращает true, если уведомление заданного
// типа (days_before ∈ {7,3,1,0}) уже отправлялось этому пользователю.
// UNIQUE(telegram_id, days_before) гарантирует, что одно и то же
// напоминание не уйдёт дважды.
func (r *repo) hasSubscriptionNotification(ctx context.Context, telegramID int64, daysBefore int) (bool, error) {
	const q = `SELECT 1 FROM subscription_notifications WHERE telegram_id = ? AND days_before = ?`
	var dummy int
	err := r.db.QueryRowContext(ctx, r.bq(q), telegramID, daysBefore).Scan(&dummy)
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
	q := `INSERT INTO subscription_notifications (telegram_id, days_before, sent_at) VALUES (?, ?, ?)`
	if r.driver == "postgres" {
		q += ` ON CONFLICT DO NOTHING`
	}
	if _, err := r.db.ExecContext(ctx, r.bq(q), telegramID, daysBefore, sentAt); err != nil {
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
	err := r.db.QueryRowContext(ctx, r.bq(q), telegramID, indicator).Scan(&until)
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
	if _, err := r.db.ExecContext(ctx, r.bq(q), telegramID, indicator, until); err != nil {
		return err
	}
	return nil
}

// clearSuppression удаляет запись подавления по показателю (для dev-сброса).
func (r *repo) clearSuppression(ctx context.Context, telegramID int64, indicator string) error {
	const q = `DELETE FROM notification_suppressions WHERE telegram_id = ? AND indicator = ?`
	if _, err := r.db.ExecContext(ctx, r.bq(q), telegramID, indicator); err != nil {
		return err
	}
	return nil
}

// deleteByUser полностью удаляет все данные уведомлений пользователя:
// использованные промокоды, логи напоминаний о подписке и подавления
// уведомлений об отклонениях. Используется функцией «Удалить аккаунт».
func (r *repo) deleteByUser(ctx context.Context, telegramID int64) error {
	for _, q := range []string{
		`DELETE FROM used_promocodes WHERE user_id = ?`,
		`DELETE FROM subscription_notifications WHERE telegram_id = ?`,
		`DELETE FROM notification_suppressions WHERE telegram_id = ?`,
	} {
		if _, err := r.db.ExecContext(ctx, r.bq(q), telegramID); err != nil {
			return err
		}
	}
	return nil
}
