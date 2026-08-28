-- Schema for Yandex Cloud Managed PostgreSQL (ПРИМЕНЯТЬ ОДИН РАЗ).
-- Накатывается автоматически при старте бота (db.Migrate, driver=postgres),
-- этот файл - справочная копия для ручного применения / pgloader.
-- Типы: AUTOINCREMENT -> BIGSERIAL, DATETIME -> TIMESTAMP.

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    is_premium INTEGER NOT NULL DEFAULT 0,
    premium_expires_at TIMESTAMP,
    tariff_id TEXT NOT NULL DEFAULT '',
    onboarding_completed INTEGER NOT NULL DEFAULT 0,
    last_activity_date TIMESTAMP,
    created_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS diagnoses (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    date TIMESTAMP NOT NULL,
    type TEXT NOT NULL DEFAULT '',
    json_data TEXT NOT NULL DEFAULT '',
    report_html TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_diagnoses_user ON diagnoses(user_id);
CREATE INDEX IF NOT EXISTS idx_diagnoses_date ON diagnoses(date);
CREATE INDEX IF NOT EXISTS idx_diagnoses_user_type ON diagnoses(user_id, type);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE INDEX IF NOT EXISTS idx_users_premium_expires_at ON users(premium_expires_at);

CREATE TABLE IF NOT EXISTS cycles (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP,
    tracked_markers TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS preferences (
    user_id INTEGER PRIMARY KEY,
    reminder_frequency TEXT NOT NULL DEFAULT '',
    units TEXT NOT NULL DEFAULT '',
    notifications_enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS monitoring_projects (
    id BIGSERIAL PRIMARY KEY,
    telegram_id INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user ON monitoring_projects(telegram_id);
CREATE INDEX IF NOT EXISTS idx_monitoring_projects_created ON monitoring_projects(created_at);
CREATE INDEX IF NOT EXISTS idx_monitoring_projects_user_type ON monitoring_projects(telegram_id, type);

CREATE TABLE IF NOT EXISTS monitoring_history (
    id BIGSERIAL PRIMARY KEY,
    telegram_id INTEGER NOT NULL,
    type TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    date TIMESTAMP NOT NULL,
    json_data TEXT NOT NULL DEFAULT '',
    report_html TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_monitoring_history_user ON monitoring_history(telegram_id);
CREATE INDEX IF NOT EXISTS idx_monitoring_history_date ON monitoring_history(date);
CREATE INDEX IF NOT EXISTS idx_monitoring_history_user_type ON monitoring_history(telegram_id, type);
CREATE TABLE IF NOT EXISTS monitoring_project_entries (
    project_id INTEGER NOT NULL,
    entry_id INTEGER NOT NULL,
    PRIMARY KEY (project_id, entry_id)
);
CREATE TABLE IF NOT EXISTS used_promocodes (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    code TEXT NOT NULL,
    used_at TIMESTAMP NOT NULL,
    UNIQUE(user_id, code)
);
CREATE INDEX IF NOT EXISTS idx_used_promocodes_user ON used_promocodes(user_id);
CREATE TABLE IF NOT EXISTS subscription_notifications (
    id BIGSERIAL PRIMARY KEY,
    telegram_id INTEGER NOT NULL,
    days_before INTEGER NOT NULL,
    sent_at TIMESTAMP NOT NULL,
    UNIQUE(telegram_id, days_before)
);
CREATE INDEX IF NOT EXISTS idx_sub_notif_user ON subscription_notifications(telegram_id);
CREATE TABLE IF NOT EXISTS notification_suppressions (
    id BIGSERIAL PRIMARY KEY,
    telegram_id INTEGER NOT NULL,
    indicator TEXT NOT NULL,
    suppressed_until TIMESTAMP NOT NULL,
    UNIQUE(telegram_id, indicator)
);
CREATE INDEX IF NOT EXISTS idx_notif_supp_user ON notification_suppressions(telegram_id);

CREATE TABLE IF NOT EXISTS blocked_users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    reason TEXT NOT NULL DEFAULT '',
    blocked_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_blocked_users ON blocked_users(telegram_id);
