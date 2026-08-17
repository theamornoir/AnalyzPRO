package monitoring

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// HistorySaver - минимальный интерфейс для сохранения результата в историю.
// Используется слоями загрузки анализов/биосканов, чтобы любой результат
// автоматически попадал в историю пользователя. Позже реализуется
// реальным репозиторием БД.
type HistorySaver interface {
	SaveResult(ctx context.Context, entry *HistoryEntry) error
}

// Repository - хранилище проектов мониторинга и истории пользователя.
// На текущем этапе - in-memory мок; интерфейс стабилен для будущей БД.
type Repository interface {
	HistorySaver

	// Проекты
	CreateProject(ctx context.Context, p *MonitoringProject) error
	GetProject(ctx context.Context, id int64) (*MonitoringProject, error)
	ListProjects(ctx context.Context, telegramID int64) ([]MonitoringProject, error)
	CompleteProject(ctx context.Context, id int64, endDate time.Time) error

	// Записи проекта (привязки)
	BindEntry(ctx context.Context, projectID, entryID int64) error
	UnbindEntry(ctx context.Context, projectID, entryID int64) error
	ListProjectEntries(ctx context.Context, projectID int64) ([]int64, error)

	// История
	ListHistory(ctx context.Context, telegramID int64, entryType string, page, pageSize int) ([]HistoryEntry, int, error)
	GetHistoryEntry(ctx context.Context, id int64) (*HistoryEntry, error)
}

// MockRepository - потокобезопасная in-memory реализация Repository.
// Общий экземпляр используется и API (чтение), и слоями загрузки
// (запись истории), поэтому хранится в одном месте и инжектируется.
type MockRepository struct {
	mu         sync.RWMutex
	projects   map[int64]*MonitoringProject
	histories  map[int64]*HistoryEntry
	nextProjID int64
	nextHistID int64
}

// NewMockRepository создаёт пустой in-memory репозиторий.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		projects:   make(map[int64]*MonitoringProject),
		histories:  make(map[int64]*HistoryEntry),
		nextProjID: 1,
		nextHistID: 1,
	}
}

// SaveResult - сохраняет запись в историю (авто-инкремент ID, дата по
// умолчанию = сейчас). Реализует HistorySaver.
func (m *MockRepository) SaveResult(ctx context.Context, entry *HistoryEntry) error {
	if entry == nil {
		return fmt.Errorf("history entry is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.ID = m.nextHistID
	m.nextHistID++
	if entry.Date.IsZero() {
		entry.Date = time.Now()
	}
	cp := *entry
	m.histories[cp.ID] = &cp
	return nil
}

// CreateProject - сохраняет проект мониторинга.
func (m *MockRepository) CreateProject(ctx context.Context, p *MonitoringProject) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if p.TelegramID == 0 {
		return fmt.Errorf("project telegram_id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p.ID = m.nextProjID
	m.nextProjID++
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.Status == "" {
		p.Status = ProjectStatusActive
	}
	if p.EntryIDs == nil {
		p.EntryIDs = []int64{}
	}
	cp := *p
	m.projects[cp.ID] = &cp
	return nil
}

// GetProject - возвращает проект по ID (с копией среза привязок).
func (m *MockRepository) GetProject(ctx context.Context, id int64) (*MonitoringProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[id]
	if !ok {
		return nil, fmt.Errorf("project not found")
	}
	cp := *p
	cp.EntryIDs = append([]int64{}, p.EntryIDs...)
	return &cp, nil
}

// ListProjects - список проектов пользователя (новые сверху).
func (m *MockRepository) ListProjects(ctx context.Context, telegramID int64) ([]MonitoringProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]MonitoringProject, 0, len(m.projects))
	for _, p := range m.projects {
		if p.TelegramID != telegramID {
			continue
		}
		cp := *p
		cp.EntryIDs = append([]int64{}, p.EntryIDs...)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// CompleteProject - завершает проект (проставляет дату окончания и статус).
func (m *MockRepository) CompleteProject(ctx context.Context, id int64, endDate time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[id]
	if !ok {
		return fmt.Errorf("project not found")
	}
	p.Status = ProjectStatusCompleted
	if !endDate.IsZero() {
		p.EndDate = endDate
	} else {
		p.EndDate = time.Now()
	}
	return nil
}

// BindEntry - привязывает запись истории к проекту (без дублей).
func (m *MockRepository) BindEntry(ctx context.Context, projectID, entryID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[projectID]
	if !ok {
		return fmt.Errorf("project not found")
	}
	if _, ok := m.histories[entryID]; !ok {
		return fmt.Errorf("history entry not found")
	}
	for _, e := range p.EntryIDs {
		if e == entryID {
			return nil // уже привязано
		}
	}
	p.EntryIDs = append(p.EntryIDs, entryID)
	return nil
}

// UnbindEntry - отвязывает запись от проекта.
func (m *MockRepository) UnbindEntry(ctx context.Context, projectID, entryID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[projectID]
	if !ok {
		return fmt.Errorf("project not found")
	}
	filtered := p.EntryIDs[:0]
	for _, e := range p.EntryIDs {
		if e != entryID {
			filtered = append(filtered, e)
		}
	}
	p.EntryIDs = filtered
	return nil
}

// ListProjectEntries - возвращает список привязанных ID записей.
func (m *MockRepository) ListProjectEntries(ctx context.Context, projectID int64) ([]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.projects[projectID]
	if !ok {
		return nil, fmt.Errorf("project not found")
	}
	out := append([]int64{}, p.EntryIDs...)
	return out, nil
}

// ListHistory - история пользователя с фильтром по типу и пагинацией.
// page - 1-based; pageSize <= 0 означает «вернуть все».
func (m *MockRepository) ListHistory(ctx context.Context, telegramID int64, entryType string, page, pageSize int) ([]HistoryEntry, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := make([]HistoryEntry, 0, len(m.histories))
	for _, h := range m.histories {
		if h.TelegramID != telegramID {
			continue
		}
		if entryType != "" && h.Type != entryType {
			continue
		}
		filtered = append(filtered, *h)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Date.After(filtered[j].Date)
	})

	total := len(filtered)
	if pageSize <= 0 {
		return filtered, total, nil
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []HistoryEntry{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

// GetHistoryEntry - возвращает запись истории по ID.
func (m *MockRepository) GetHistoryEntry(ctx context.Context, id int64) (*HistoryEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.histories[id]
	if !ok {
		return nil, fmt.Errorf("history entry not found")
	}
	cp := *h
	return &cp, nil
}

// compile-time проверка, что MockRepository реализует Repository.
var _ Repository = (*MockRepository)(nil)

// PreviousReportJSON возвращает JSON-данные (поле JsonData) самой свежей
// записи указанного типа пользователя. Используется для сравнительного
// повторного анализа/биоскана: перед генерацией нового отчёта мы берём
// предыдущий и подсовываем ИИ, чтобы он построил СРАВНИТЕЛЬНЫЙ отчёт
// (что улучшилось / что улучшить), а не «с нуля». Возвращает ok=false,
// если предыдущих записей этого типа нет.
func PreviousReportJSON(ctx context.Context, repo Repository, telegramID int64, entryType string) (string, bool) {
	entries, _, err := repo.ListHistory(ctx, telegramID, entryType, 1, 1)
	if err != nil || len(entries) == 0 {
		return "", false
	}
	return entries[0].JsonData, true
}
