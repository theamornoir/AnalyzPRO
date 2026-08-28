package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// Store is a thread-safe, JSON-file-backed, dependency-free implementation of
// the storage interfaces. It is intended as a drop-in persistence layer for
// single-instance deployments (the bot enforces a single instance via a file
// lock, see app.go acquireInstanceLock). To migrate to a real RDBMS, implement
// the same four interfaces against SQL and swap the constructor in storage.go -
// no caller changes are required.
//
// All mutations go through a single mutex; the on-disk write uses an atomic
// temp-file + rename so a crash never leaves a half-written JSON file.
type Store struct {
	mu   sync.RWMutex // guards data and serializes file writes (RLock for reads)
	path string
	data *fileData
}

type fileData struct {
	Users        map[int64]*sm.User        `json:"users"` // key: telegram_id
	Diagnoses    []sm.Diagnosis            `json:"diagnoses"`
	Cycles       []sm.Cycle                `json:"cycles"`
	Preferences  map[uint]*sm.Preference   `json:"preferences"`     // key: user_id
	UsedPromo    map[int64]map[string]bool `json:"used_promocodes"` // key: user_id -> code -> true
	Profiles     map[int64]*sm.Profile     `json:"profiles"`        // key: telegram_id
	BlockedUsers map[int64]string          `json:"blocked_users"`   // key: telegram_id -> reason
	NextUserID   uint                      `json:"next_user_id"`
	NextDiagID   uint                      `json:"next_diag_id"`
	NextCycleID  uint                      `json:"next_cycle_id"`
}

// New открывает (или создаёт) JSON-файл хранилища и загружает данные.
func New(path string) *Store {
	s := &Store{
		path: path,
		data: &fileData{
			Users:        map[int64]*sm.User{},
			Diagnoses:    nil,
			Cycles:       nil,
			Preferences:  map[uint]*sm.Preference{},
			Profiles:     map[int64]*sm.Profile{},
			BlockedUsers: map[int64]string{},
		},
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	s.load()
	return s
}

func (s *Store) load() {
	f, err := os.Open(s.path)
	if err != nil {
		return // пустой старт - файл появится после первой записи
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(s.data)
	if s.data.Users == nil {
		s.data.Users = map[int64]*sm.User{}
	}
	if s.data.Preferences == nil {
		s.data.Preferences = map[uint]*sm.Preference{}
	}
	if s.data.Profiles == nil {
		s.data.Profiles = map[int64]*sm.Profile{}
	}
	if s.data.BlockedUsers == nil {
		s.data.BlockedUsers = map[int64]string{}
	}
}

// save - вызывать ТОЛЬКО под s.mu. Атомарная запись через temp + rename.
func (s *Store) save() {
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := s.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.data); err != nil {
		f.Close()
		return
	}
	if err := f.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

// ---------------------------------------------------------------
// UserRepository
// ---------------------------------------------------------------

func (s *Store) CreateUser(ctx context.Context, user *sm.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	// Upsert по TelegramID: сохраняем оригинальный ID, обновляем поля.
	if existing, ok := s.data.Users[user.TelegramID]; ok && existing != nil {
		if user.ID == 0 {
			user.ID = existing.ID
		}
	} else if user.ID == 0 {
		s.data.NextUserID++
		user.ID = s.data.NextUserID
	}

	cp := *user
	s.data.Users[user.TelegramID] = &cp
	s.save()
	return nil
}

func (s *Store) GetUserByTelegramID(ctx context.Context, telegramID int64) (*sm.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.data.Users[telegramID]
	if !ok || u == nil {
		return nil, fmt.Errorf("user not found")
	}
	cp := *u
	return &cp, nil
}

// GetAllUsers возвращает всех пользователей (для напоминаний).
func (s *Store) GetAllUsers(ctx context.Context) ([]*sm.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*sm.User, 0, len(s.data.Users))
	for _, u := range s.data.Users {
		if u == nil {
			continue
		}
		cp := *u
		out = append(out, &cp)
	}
	return out, nil
}

func (s *Store) UpdateUserPremiumStatus(ctx context.Context, userID uint, isPremium bool, expiresAt time.Time, tariffID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		if u.ID == userID {
			u.IsPremium = isPremium
			u.PremiumExpiresAt = expiresAt
			u.TariffID = tariffID
			s.save()
			return nil
		}
	}
	return fmt.Errorf("user with ID %d not found", userID)
}

func (s *Store) UpdateUserOnboardingStatus(ctx context.Context, userID uint, completed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		if u.ID == userID {
			u.OnboardingCompleted = completed
			s.save()
			return nil
		}
	}
	return fmt.Errorf("user with ID %d not found", userID)
}

// UpdateUserLastActivity обновляет дату последнего взаимодействия.
func (s *Store) UpdateUserLastActivity(ctx context.Context, userID uint, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.data.Users {
		if u.ID == userID {
			u.LastActivityDate = t
			s.save()
			return nil
		}
	}
	return fmt.Errorf("user with ID %d not found", userID)
}

// IsPromoCodeUsed - проверяет, активировал ли пользователь промокод.
func (s *Store) IsPromoCodeUsed(ctx context.Context, userID int64, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.UsedPromo == nil {
		return false
	}
	if codes, ok := s.data.UsedPromo[userID]; ok {
		return codes[code]
	}
	return false
}

// MarkPromoCodeUsed - помечает промокод использованным (идемпотентно).
func (s *Store) MarkPromoCodeUsed(ctx context.Context, userID int64, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.UsedPromo == nil {
		s.data.UsedPromo = make(map[int64]map[string]bool)
	}
	if s.data.UsedPromo[userID] == nil {
		s.data.UsedPromo[userID] = make(map[string]bool)
	}
	s.data.UsedPromo[userID][code] = true
	s.save()
	return nil
}

// DeleteAccount полностью удаляет пользователя и все связанные данные
// (анализы, курсы, предпочтения, промокоды) по Telegram ID. Необратимо.
func (s *Store) DeleteAccount(ctx context.Context, telegramID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.data.Users[telegramID]
	if !ok || u == nil {
		return nil // и так нет - считаем удалённым
	}
	userID := u.ID
	delete(s.data.Users, telegramID)
	delete(s.data.Preferences, userID)
	delete(s.data.UsedPromo, telegramID)

	keptDiag := s.data.Diagnoses[:0]
	for _, d := range s.data.Diagnoses {
		if d.UserID != userID {
			keptDiag = append(keptDiag, d)
		}
	}
	s.data.Diagnoses = keptDiag
	keptCycle := s.data.Cycles[:0]
	for _, c := range s.data.Cycles {
		if c.UserID != userID {
			keptCycle = append(keptCycle, c)
		}
	}
	s.data.Cycles = keptCycle
	s.save()
	return nil
}

// ---------------------------------------------------------------
// DiagnosisRepository
// ---------------------------------------------------------------

func (s *Store) SaveDiagnosis(ctx context.Context, diagnosis *sm.Diagnosis) error {
	if diagnosis == nil {
		return fmt.Errorf("diagnosis is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if diagnosis.ID == 0 {
		s.data.NextDiagID++
		diagnosis.ID = s.data.NextDiagID
	}
	if diagnosis.Date.IsZero() {
		diagnosis.Date = time.Now()
	}
	cp := *diagnosis
	s.data.Diagnoses = append(s.data.Diagnoses, cp)
	s.save()
	return nil
}

func (s *Store) GetAllDiagnosesByUserID(ctx context.Context, userID uint, limit, offset int) ([]sm.Diagnosis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]sm.Diagnosis, 0)
	for _, d := range s.data.Diagnoses {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	if limit > 0 {
		if offset >= len(out) {
			out = nil
		} else {
			end := offset + limit
			if end > len(out) {
				end = len(out)
			}
			out = out[offset:end]
		}
	}
	return out, nil
}

func (s *Store) GetLastDiagnosisByType(ctx context.Context, userID uint, diagnosisType string) (*sm.Diagnosis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var last *sm.Diagnosis
	for i := range s.data.Diagnoses {
		d := &s.data.Diagnoses[i]
		if d.UserID == userID && d.Type == diagnosisType {
			last = d
		}
	}
	if last == nil {
		return nil, fmt.Errorf("diagnosis not found")
	}
	cp := *last
	return &cp, nil
}

// ---------------------------------------------------------------
// CycleRepository
// ---------------------------------------------------------------

func (s *Store) CreateCycle(ctx context.Context, cycle *sm.Cycle) error {
	if cycle == nil {
		return fmt.Errorf("cycle is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if cycle.ID == 0 {
		s.data.NextCycleID++
		cycle.ID = s.data.NextCycleID
	}
	if cycle.StartDate.IsZero() {
		cycle.StartDate = time.Now()
	}
	cp := *cycle
	s.data.Cycles = append(s.data.Cycles, cp)
	s.save()
	return nil
}

func (s *Store) GetActiveCycleByUserID(ctx context.Context, userID uint) (*sm.Cycle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Cycles {
		c := &s.data.Cycles[i]
		if c.UserID == userID && c.EndDate.IsZero() {
			cp := *c
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("active cycle not found")
}

func (s *Store) CompleteCycle(ctx context.Context, cycleID uint, endDate time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Cycles {
		if s.data.Cycles[i].ID == cycleID {
			s.data.Cycles[i].EndDate = endDate
			s.save()
			return nil
		}
	}
	return fmt.Errorf("cycle with ID %d not found", cycleID)
}

// ---------------------------------------------------------------
// PreferenceRepository
// ---------------------------------------------------------------

func (s *Store) GetPreferences(ctx context.Context, userID uint) (*sm.Preference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.data.Preferences[userID]
	if !ok || p == nil {
		return nil, fmt.Errorf("preferences not found")
	}
	cp := *p
	return &cp, nil
}

func (s *Store) UpdatePreferences(ctx context.Context, preferences *sm.Preference) error {
	if preferences == nil {
		return fmt.Errorf("preferences is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *preferences
	s.data.Preferences[preferences.UserID] = &cp
	s.save()
	return nil
}

// compile-time проверки, что Store реализует все интерфейсы.
var (
	_ interfaces.UserRepository       = (*Store)(nil)
	_ interfaces.DiagnosisRepository  = (*Store)(nil)
	_ interfaces.CycleRepository      = (*Store)(nil)
	_ interfaces.PreferenceRepository = (*Store)(nil)
)

// UpdateUserPremiumStatusByTelegramID обновляет Premium по Telegram ID.
func (s *Store) UpdateUserPremiumStatusByTelegramID(ctx context.Context, telegramID int64, isPremium bool, expiresAt time.Time, tariffID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if u, ok := s.data.Users[telegramID]; ok && u != nil {
		u.IsPremium = isPremium
		u.PremiumExpiresAt = expiresAt
		u.TariffID = tariffID
		s.save()
		return nil
	}
	return fmt.Errorf("user with telegram_id %d not found", telegramID)
}

// BlockUser блокирует пользователя (по Telegram chat ID).
func (s *Store) BlockUser(ctx context.Context, telegramID int64, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.BlockedUsers == nil {
		s.data.BlockedUsers = map[int64]string{}
	}
	s.data.BlockedUsers[telegramID] = reason
	s.save()
	return nil
}

// UnblockUser снимает блокировку пользователя.
func (s *Store) UnblockUser(ctx context.Context, telegramID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data.BlockedUsers, telegramID)
	s.save()
	return nil
}

// IsBlocked возвращает true, если пользователь заблокирован.
func (s *Store) IsBlocked(ctx context.Context, telegramID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data.BlockedUsers[telegramID]
	return ok
}

// ListBlocked возвращает список заблокированных Telegram chat ID.
func (s *Store) ListBlocked(ctx context.Context) ([]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]int64, 0, len(s.data.BlockedUsers))
	for id := range s.data.BlockedUsers {
		out = append(out, id)
	}
	return out, nil
}

// GetProfile возвращает постоянный профиль пользователя по Telegram ID.
// Если профиль не заполнен - (nil, nil).
func (s *Store) GetProfile(ctx context.Context, telegramID int64) (*sm.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.data.Profiles[telegramID]
	if !ok || p == nil {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

// UpsertProfile создаёт или обновляет постоянный профиль пользователя.
func (s *Store) UpsertProfile(ctx context.Context, profile *sm.Profile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *profile
	s.data.Profiles[profile.TelegramID] = &cp
	s.save()
	return nil
}
