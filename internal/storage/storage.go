package storage

import (
	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
	"github.com/theamornoir/analyzpro/internal/storage/mock"
)

// Storage - точка доступа ко всем хранилищам данных.
type Storage struct {
	Users       interfaces.UserRepository
	Diagnoses   interfaces.DiagnosisRepository
	Cycles      interfaces.CycleRepository
	Preferences interfaces.PreferenceRepository
}

// NewMockStorage создаёт хранилище на основе мок-репозиториев.
func NewMockStorage() *Storage {
	return &Storage{
		Users:       mock.NewMockUserRepository(),
		Diagnoses:   mock.NewMockDiagnosisRepository(),
		Cycles:      mock.NewMockCycleRepository(),
		Preferences: mock.NewMockPreferenceRepository(),
	}
}

// compile-time checks: мок-репозитории реализуют интерфейсы
var (
	_ interfaces.UserRepository       = (*mock.MockUserRepository)(nil)
	_ interfaces.DiagnosisRepository  = (*mock.MockDiagnosisRepository)(nil)
	_ interfaces.CycleRepository      = (*mock.MockCycleRepository)(nil)
	_ interfaces.PreferenceRepository = (*mock.MockPreferenceRepository)(nil)
)
