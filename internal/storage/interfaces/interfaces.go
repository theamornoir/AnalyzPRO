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
	UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time) error
	UpdateUserOnboardingStatus(ctx context.Context, userID uint, completed bool) error
}

// DiagnosisRepository - репозиторий диагнозов.
type DiagnosisRepository interface {
	SaveDiagnosis(ctx context.Context, diagnosis *sm.Diagnosis) error
	GetAllDiagnosesByUserID(ctx context.Context, userID uint) ([]sm.Diagnosis, error)
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
