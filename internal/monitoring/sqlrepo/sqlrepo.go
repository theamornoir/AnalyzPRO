package sqlrepo

// Package sqlrepo - реализация Repository (мониторинг: проекты + история)
// поверх *sql.DB. Драйвер-агностичен: SQL пишется в стиле SQLite (?), а
// db.BindQuery преобразует ? -> $N для PostgreSQL. Один и тот же код работает
// и с локальной SQLite, и с Yandex Cloud PG.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/monitoring"
)

type repo struct {
	db     *sql.DB
	driver string // "sqlite" | "postgres"
}

// New создаёт SQL-реализацию репозитория мониторинга. driver опционален
// (по умолчанию "sqlite") - обратная совместимость с тестами.
func New(dbConn *sql.DB, driver ...string) *repo {
	d := "sqlite"
	if len(driver) > 0 && driver[0] == "postgres" {
		d = "postgres"
	}
	return &repo{db: dbConn, driver: d}
}

// bq адаптирует SQL-запрос под текущий драйвер (для postgres ? -> $N).
func (r *repo) bq(q string) string { return db.BindQuery(r.driver, q) }

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

// nullTimeScanner - адаптер sql.Scanner для *time.Time (допускает NULL).
type nullTimeScanner struct{ tp *time.Time }

func (n *nullTimeScanner) Scan(src interface{}) error {
	*n.tp = time.Time{}
	if src == nil {
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
// HistorySaver
// ---------------------------------------------------------------

func (r *repo) SaveResult(ctx context.Context, entry *monitoring.HistoryEntry) error {
	if entry == nil {
		return fmt.Errorf("history entry is nil")
	}
	if entry.Date.IsZero() {
		entry.Date = time.Now()
	}
	// PostgreSQL не поддерживает LastInsertId - используем RETURNING id.
	if r.driver == "postgres" {
		const q = `INSERT INTO monitoring_history (telegram_id, type, title, date, json_data, report_html)
			VALUES (?, ?, ?, ?, ?, ?) RETURNING id`
		var id int64
		if err := r.db.QueryRowContext(ctx, r.bq(q),
			entry.TelegramID, entry.Type, entry.Title, entry.Date, entry.JsonData, entry.ReportHTML,
		).Scan(&id); err != nil {
			return fmt.Errorf("сохранение истории: %w", err)
		}
		entry.ID = id
		return nil
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO monitoring_history (telegram_id, type, title, date, json_data, report_html)
		 VALUES (?, ?, ?, ?, ?, ?)`),
		entry.TelegramID, entry.Type, entry.Title, entry.Date, entry.JsonData, entry.ReportHTML)
	if err != nil {
		return fmt.Errorf("сохранение истории: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		entry.ID = id
	}
	return nil
}

// ---------------------------------------------------------------
// Проекты
// ---------------------------------------------------------------

func (r *repo) CreateProject(ctx context.Context, p *monitoring.MonitoringProject) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if p.TelegramID == 0 {
		return fmt.Errorf("project telegram_id is required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.Status == "" {
		p.Status = monitoring.ProjectStatusActive
	}
	if p.EntryIDs == nil {
		p.EntryIDs = []int64{}
	}
	// PostgreSQL не поддерживает LastInsertId - используем RETURNING id.
	if r.driver == "postgres" {
		const q = `INSERT INTO monitoring_projects (telegram_id, name, type, start_date, end_date, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`
		var id int64
		if err := r.db.QueryRowContext(ctx, r.bq(q),
			p.TelegramID, p.Name, p.Type, p.StartDate, nullTime(p.EndDate), p.Status, p.CreatedAt,
		).Scan(&id); err != nil {
			return fmt.Errorf("создание проекта: %w", err)
		}
		p.ID = id
		return nil
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`INSERT INTO monitoring_projects (telegram_id, name, type, start_date, end_date, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`),
		p.TelegramID, p.Name, p.Type, p.StartDate, nullTime(p.EndDate), p.Status, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("создание проекта: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		p.ID = id
	}
	return nil
}

func (r *repo) GetProject(ctx context.Context, id int64) (*monitoring.MonitoringProject, error) {
	const q = `SELECT id, telegram_id, name, type, start_date, end_date, status, created_at
		FROM monitoring_projects WHERE id = ?`
	p := &monitoring.MonitoringProject{}
	var endDate sql.NullTime
	if err := r.db.QueryRowContext(ctx, r.bq(q), id).Scan(
		&p.ID, &p.TelegramID, &p.Name, &p.Type, &p.StartDate, &endDate, &p.Status, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("чтение проекта: %w", err)
	}
	if endDate.Valid {
		p.EndDate = endDate.Time
	}
	entries, err := r.ListProjectEntries(ctx, id)
	if err != nil {
		return nil, err
	}
	p.EntryIDs = entries
	return p, nil
}

func (r *repo) ListProjects(ctx context.Context, telegramID int64) ([]monitoring.MonitoringProject, error) {
	rows, err := r.db.QueryContext(ctx,
		r.bq(`SELECT id, telegram_id, name, type, start_date, end_date, status, created_at
		 FROM monitoring_projects WHERE telegram_id = ? ORDER BY created_at DESC`), telegramID)
	if err != nil {
		return nil, fmt.Errorf("список проектов: %w", err)
	}
	defer rows.Close()

	out := make([]monitoring.MonitoringProject, 0)
	for rows.Next() {
		var p monitoring.MonitoringProject
		var endDate sql.NullTime
		if err := rows.Scan(&p.ID, &p.TelegramID, &p.Name, &p.Type, &p.StartDate, &endDate, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		if endDate.Valid {
			p.EndDate = endDate.Time
		}
		p.EntryIDs = []int64{}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		entries, err := r.ListProjectEntries(ctx, out[i].ID)
		if err == nil {
			out[i].EntryIDs = entries
		}
	}
	return out, nil
}

func (r *repo) CompleteProject(ctx context.Context, id int64, endDate time.Time) error {
	if endDate.IsZero() {
		endDate = time.Now()
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`UPDATE monitoring_projects SET status = ?, end_date = ? WHERE id = ?`),
		monitoring.ProjectStatusCompleted, endDate, id)
	if err != nil {
		return fmt.Errorf("завершение проекта: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

// ---------------------------------------------------------------
// Привязки записей к проектам
// ---------------------------------------------------------------

func (r *repo) BindEntry(ctx context.Context, projectID, entryID int64) error {
	if _, err := r.GetProject(ctx, projectID); err != nil {
		return fmt.Errorf("project not found")
	}
	if _, err := r.GetHistoryEntry(ctx, entryID); err != nil {
		return fmt.Errorf("history entry not found")
	}
	q := `INSERT INTO monitoring_project_entries (project_id, entry_id) VALUES (?, ?)`
	// SQLite: INSERT OR IGNORE; PostgreSQL: ON CONFLICT DO NOTHING.
	if r.driver == "postgres" {
		q += ` ON CONFLICT DO NOTHING`
	} else {
		q = `INSERT OR IGNORE INTO monitoring_project_entries (project_id, entry_id) VALUES (?, ?)`
	}
	if _, err := r.db.ExecContext(ctx, r.bq(q), projectID, entryID); err != nil {
		return fmt.Errorf("привязка записи: %w", err)
	}
	return nil
}

func (r *repo) UnbindEntry(ctx context.Context, projectID, entryID int64) error {
	if _, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_project_entries WHERE project_id = ? AND entry_id = ?`),
		projectID, entryID); err != nil {
		return fmt.Errorf("отвязка записи: %w", err)
	}
	return nil
}

func (r *repo) ListProjectEntries(ctx context.Context, projectID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		r.bq(`SELECT entry_id FROM monitoring_project_entries WHERE project_id = ? ORDER BY entry_id`), projectID)
	if err != nil {
		return nil, fmt.Errorf("список привязок: %w", err)
	}
	defer rows.Close()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------
// История
// ---------------------------------------------------------------

func (r *repo) ListHistory(ctx context.Context, telegramID int64, entryType string, page, pageSize int) ([]monitoring.HistoryEntry, int, error) {
	base := ` FROM monitoring_history WHERE telegram_id = ?`
	args := []interface{}{telegramID}
	if entryType != "" {
		base += ` AND type = ?`
		args = append(args, entryType)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, r.bq(`SELECT COUNT(*)`+base), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("подсчёт истории: %w", err)
	}

	query := `SELECT id, telegram_id, type, title, date, json_data, report_html ` + base + ` ORDER BY date DESC`
	if pageSize <= 0 {
		pageSize = 200
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, offset)

	rows, err := r.db.QueryContext(ctx, r.bq(query), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("чтение истории: %w", err)
	}
	defer rows.Close()

	out := make([]monitoring.HistoryEntry, 0)
	for rows.Next() {
		var h monitoring.HistoryEntry
		if err := rows.Scan(&h.ID, &h.TelegramID, &h.Type, &h.Title, &h.Date, &h.JsonData, &h.ReportHTML); err != nil {
			return nil, 0, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *repo) GetHistoryEntry(ctx context.Context, id int64) (*monitoring.HistoryEntry, error) {
	const q = `SELECT id, telegram_id, type, title, date, json_data, report_html FROM monitoring_history WHERE id = ?`
	var h monitoring.HistoryEntry
	if err := r.db.QueryRowContext(ctx, r.bq(q), id).Scan(&h.ID, &h.TelegramID, &h.Type, &h.Title, &h.Date, &h.JsonData, &h.ReportHTML); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("history entry not found")
		}
		return nil, fmt.Errorf("чтение записи: %w", err)
	}
	cp := h
	return &cp, nil
}

// DeleteHistoryEntry - удаляет запись истории по ID. Сначала отвязываем
// запись от проектов (monitoring_project_entries), затем удаляем саму
// запись из monitoring_history. Возвращает ошибку, если записи нет.
func (r *repo) DeleteHistoryEntry(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_project_entries WHERE entry_id = ?`), id); err != nil {
		return fmt.Errorf("отвязка от проектов: %w", err)
	}
	res, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_history WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("удаление записи: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("history entry not found")
	}
	return nil
}

// DeleteByUser полностью удаляет проекты мониторинга и историю
// пользователя по Telegram ID. Сначала чистим привязки записей к проектам
// (monitoring_project_entries), затем историю и сами проекты.
func (r *repo) DeleteByUser(ctx context.Context, telegramID int64) error {
	if _, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_project_entries WHERE project_id IN (SELECT id FROM monitoring_projects WHERE telegram_id = ?)`),
		telegramID); err != nil {
		return fmt.Errorf("удаление привязок проектов: %w", err)
	}
	if _, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_history WHERE telegram_id = ?`), telegramID); err != nil {
		return fmt.Errorf("удаление истории: %w", err)
	}
	if _, err := r.db.ExecContext(ctx,
		r.bq(`DELETE FROM monitoring_projects WHERE telegram_id = ?`), telegramID); err != nil {
		return fmt.Errorf("удаление проектов: %w", err)
	}
	return nil
}

// compile-time проверка, что repo реализует Repository.
var _ monitoring.Repository = (*repo)(nil)
