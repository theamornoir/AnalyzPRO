package models

import "time"

// User - модель пользователя.
type User struct {
	ID               uint      `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	Name             string    `json:"name"`
	IsPremium        bool      `json:"is_premium"`
	PremiumExpiresAt time.Time `json:"premium_expires_at"`
	// OnboardingCompleted - пройден ли онбординг (слайдер + соглашение) для
	// пользователя. Новые пользователи проходят онбординг при первом /start.
	OnboardingCompleted bool `json:"onboarding_completed"`
	// LastActivityDate - дата последнего взаимодействия пользователя с ботом
	// (любое сообщение/нажатие кнопки). Используется системой уведомлений
	// для определения периода неактивности (напоминание о повторном анализе).
	LastActivityDate time.Time `json:"last_activity_date"`
	CreatedAt        time.Time `json:"created_at"`
}

// Diagnosis - модель диагноза/анализа.
type Diagnosis struct {
	ID         uint      `json:"id"`
	UserID     uint      `json:"user_id"`
	Date       time.Time `json:"date"`
	Type       string    `json:"type"` // "bioscan", "blood_analysis", "full_diagnosis"
	JsonData   string    `json:"json_data"`
	ReportHTML string    `json:"report_html"`
}

// Cycle - модель спортивного курса.
type Cycle struct {
	ID             uint      `json:"id"`
	UserID         uint      `json:"user_id"`
	Name           string    `json:"name"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	TrackedMarkers []string  `json:"tracked_markers"`
}

// Preference - модель предпочтений пользователя.
type Preference struct {
	UserID               uint   `json:"user_id"`
	ReminderFrequency    string `json:"reminder_frequency"`
	Units                string `json:"units"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
}
