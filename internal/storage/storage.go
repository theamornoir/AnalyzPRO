package storage

import (
	"database/sql"

	"github.com/theamornoir/analyzpro/internal/storage/file"
	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
	"github.com/theamornoir/analyzpro/internal/storage/mock"
	"github.com/theamornoir/analyzpro/internal/storage/sqlrepo"
)

// Storage - точка доступа ко всем хранилищам данных.
type Storage struct {
	Users       interfaces.UserRepository
	Diagnoses   interfaces.DiagnosisRepository
	Cycles      interfaces.CycleRepository
	Preferences interfaces.PreferenceRepository
}

// NewMockStorage создаёт хранилище на основе мок-репозиториев.
// Используется в тестах и в режиме USE_MOCK=true.
func NewMockStorage() *Storage {
	return &Storage{
		Users:       mock.NewMockUserRepository(),
		Diagnoses:   mock.NewMockDiagnosisRepository(),
		Cycles:      mock.NewMockCycleRepository(),
		Preferences: mock.NewMockPreferenceRepository(),
	}
}

// NewFileStorage создаёт хранилище на основе JSON-файла (реальная
// персистентность, без внешней БД). Файл создаётся/дополняется при записи.
// Расположение задаётся переменной STORAGE_PATH (по умолчанию
// ./data/analyzpro.db.json). Это «база данных» для единичного инстанса бота.
//
// Deprecated: используйте NewSQLStorage на базе *sql.DB.
func NewFileStorage(path string) *Storage {
	s := file.New(path)
	return &Storage{
		Users:       s,
		Diagnoses:   s,
		Cycles:      s,
		Preferences: s,
	}
}

// NewSQLStorage создаёт хранилище поверх *sql.DB (SQLite/Postgres). Все четыре
// под-репозитория реализованы одним SQL-бэкендом. Это основной способ
// хранения для прод-деплоя: данные переживают перезапуск бота.
func NewSQLStorage(db *sql.DB) *Storage {
	s := sqlrepo.New(db)
	return &Storage{
		Users:       s,
		Diagnoses:   s,
		Cycles:      s,
		Preferences: s,
	}
}

// compile-time checks: мок-репозитории реализуют интерфейсы
var (
	_ interfaces.UserRepository       = (*mock.MockUserRepository)(nil)
	_ interfaces.DiagnosisRepository  = (*mock.MockDiagnosisRepository)(nil)
	_ interfaces.CycleRepository      = (*mock.MockCycleRepository)(nil)
	_ interfaces.PreferenceRepository = (*mock.MockPreferenceRepository)(nil)
)
