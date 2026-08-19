package interfaces

import (
	"context"
	"time"

	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// UserRepository - репозиторий пользователей.
type UserRepository interface {
	CreateUser(ctx context.Context, user *sm.User) error
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*sm.User, error)
	// GetAllUsers - все пользователи (для периодических рассылок/напоминаний).
	GetAllUsers(ctx context.Context) ([]*sm.User, error)
	// UpdateUserPremiumStatus - обновляет флаг Premium, дату окончания
	// подписки и ID активного тарифа. userID здесь - внутренний id
	// пользователя (uint), как в таблице users.
	UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time, tariffID string) error
	UpdateUserOnboardingStatus(ctx context.Context, userID uint, completed bool) error
	// UpdateUserLastActivity - обновляет дату последнего взаимодействия
	// пользователя с ботом (нужно системе напоминаний об неактивности).
	UpdateUserLastActivity(ctx context.Context, userID uint, t time.Time) error
	// IsPromoCodeUsed - проверяет, активировал ли пользователь промокод.
	// userID здесь - Telegram chat ID (int64), как во всём боте.
	IsPromoCodeUsed(ctx context.Context, userID int64, code string) bool
	// MarkPromoCodeUsed - помечает промокод использованным пользователем
	// (идемпотентно: повторный вызов для того же userID+code не дублирует запись).
	MarkPromoCodeUsed(ctx context.Context, userID int64, code string) error
}

// DiagnosisRepository - репозиторий диагнозов.
type DiagnosisRepository interface {
	SaveDiagnosis(ctx context.Context, diagnosis *sm.Diagnosis) error
	GetAllDiagnosesByUserID(ctx context.Context, userID uint, limit, offset int) ([]sm.Diagnosis, error)
	GetLastDiagnosisByType(ctx context.Context, userID uint, diagnosisType string) (*sm.Diagnosis, error)
}

// CycleRepository - репозиторий спортивных курсов.
type CycleRepository interface {
	CreateCycle(ctx context.Context, cycle *sm.Cycle) error
	GetActiveCycleByUserID(ctx context.Context, userID uint) (*sm.Cycle, error)
	CompleteCycle(ctx context.Context, cycleID uint, endDate time.Time) error
}

// PreferenceRepository - репозиторий предпочтений.
type PreferenceRepository interface {
	GetPreferences(ctx context.Context, userID uint) (*sm.Preference, error)
	UpdatePreferences(ctx context.Context, preferences *sm.Preference) error
}
