package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// MockUserRepository - мок-реализация UserRepository.
type MockUserRepository struct {
	mu     sync.RWMutex
	users  map[int64]*sm.User // key: TelegramID
	nextID uint
}

// NewMockUserRepository создаёт новый мок-репозиторий пользователей.
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[int64]*sm.User),
		nextID: 1,
	}
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *sm.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	user.ID = m.nextID
	m.nextID++
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	m.users[user.TelegramID] = user
	return nil
}

func (m *MockUserRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*sm.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if user, ok := m.users[telegramID]; ok {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserRepository) UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.ID == userID {
			user.IsPremium = isPremium
			user.PremiumExpiresAt = expiresAt
			return nil
		}
	}
	return fmt.Errorf("user with ID %d not found", userID)
}

func (m *MockUserRepository) UpdateUserOnboardingStatus(ctx context.Context, userID uint, completed bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.ID == userID {
			user.OnboardingCompleted = completed
			return nil
		}
	}
	return fmt.Errorf("user with ID %d not found", userID)
}

// MockDiagnosisRepository - мок-реализация DiagnosisRepository.
type MockDiagnosisRepository struct {
	mu        sync.RWMutex
	diagnoses []sm.Diagnosis
	nextID    uint
}

// NewMockDiagnosisRepository создаёт новый мок-репозиторий диагнозов.
func NewMockDiagnosisRepository() *MockDiagnosisRepository {
	return &MockDiagnosisRepository{
		diagnoses: make([]sm.Diagnosis, 0),
		nextID:    1,
	}
}

func (m *MockDiagnosisRepository) SaveDiagnosis(ctx context.Context, diagnosis *sm.Diagnosis) error {
	if diagnosis == nil {
		return fmt.Errorf("diagnosis is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	diagnosis.ID = m.nextID
	m.nextID++
	if diagnosis.Date.IsZero() {
		diagnosis.Date = time.Now()
	}
	m.diagnoses = append(m.diagnoses, *diagnosis)
	return nil
}

func (m *MockDiagnosisRepository) GetAllDiagnosesByUserID(ctx context.Context, userID uint) ([]sm.Diagnosis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []sm.Diagnosis
	for _, d := range m.diagnoses {
		if d.UserID == userID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MockDiagnosisRepository) GetLastDiagnosisByType(ctx context.Context, userID uint, diagnosisType string) (*sm.Diagnosis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var last *sm.Diagnosis
	for i := range m.diagnoses {
		d := &m.diagnoses[i]
		if d.UserID == userID && d.Type == diagnosisType {
			last = d
		}
	}
	if last == nil {
		return nil, fmt.Errorf("diagnosis not found")
	}
	return last, nil
}

// MockCycleRepository - мок-реализация CycleRepository.
type MockCycleRepository struct {
	mu     sync.RWMutex
	cycles []sm.Cycle
	nextID uint
}

// NewMockCycleRepository создаёт новый мок-репозиторий курсов.
func NewMockCycleRepository() *MockCycleRepository {
	return &MockCycleRepository{
		cycles: make([]sm.Cycle, 0),
		nextID: 1,
	}
}

func (m *MockCycleRepository) CreateCycle(ctx context.Context, cycle *sm.Cycle) error {
	if cycle == nil {
		return fmt.Errorf("cycle is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cycle.ID = m.nextID
	m.nextID++
	if cycle.StartDate.IsZero() {
		cycle.StartDate = time.Now()
	}
	m.cycles = append(m.cycles, *cycle)
	return nil
}

func (m *MockCycleRepository) GetActiveCycleByUserID(ctx context.Context, userID uint) (*sm.Cycle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.cycles {
		c := &m.cycles[i]
		if c.UserID == userID && c.EndDate.IsZero() {
			return c, nil
		}
	}
	return nil, fmt.Errorf("active cycle not found")
}

func (m *MockCycleRepository) CompleteCycle(ctx context.Context, cycleID uint, endDate time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.cycles {
		if m.cycles[i].ID == cycleID {
			m.cycles[i].EndDate = endDate
			return nil
		}
	}
	return fmt.Errorf("cycle with ID %d not found", cycleID)
}

// MockPreferenceRepository - мок-реализация PreferenceRepository.
type MockPreferenceRepository struct {
	mu          sync.RWMutex
	preferences map[uint]*sm.Preference // key: UserID
}

// NewMockPreferenceRepository создаёт новый мок-репозиторий предпочтений.
func NewMockPreferenceRepository() *MockPreferenceRepository {
	return &MockPreferenceRepository{
		preferences: make(map[uint]*sm.Preference),
	}
}

func (m *MockPreferenceRepository) GetPreferences(ctx context.Context, userID uint) (*sm.Preference, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.preferences[userID]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("preferences not found")
}

func (m *MockPreferenceRepository) UpdatePreferences(ctx context.Context, preferences *sm.Preference) error {
	if preferences == nil {
		return fmt.Errorf("preferences is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preferences[preferences.UserID] = preferences
	return nil
}
