package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileRepository - файловый (JSON) репозиторий модуля Мониторинг,
// переживающий перезапуск бота. Реализует тот же интерфейс Repository,
// что и MockRepository, поэтому подменяется в app.go без изменения вызывающих.
//
// Все мутации сериализуются одним mutex; запись на диск - атомарная
// (temp-файл + rename), поэтому сбой не оставляет битый JSON.
type FileRepository struct {
	mu   sync.Mutex // guards data and serializes file writes
	path string
	data *fileRepoData
}

type fileRepoData struct {
	Projects   map[int64]*MonitoringProject `json:"projects"`
	Histories  map[int64]*HistoryEntry      `json:"histories"`
	NextProjID int64                        `json:"next_project_id"`
	NextHistID int64                        `json:"next_history_id"`
}

// NewFileRepository открывает (или создаёт) JSON-файл и загружает данные.
func NewFileRepository(path string) *FileRepository {
	r := &FileRepository{
		path: path,
		data: &fileRepoData{
			Projects:   make(map[int64]*MonitoringProject),
			Histories:  make(map[int64]*HistoryEntry),
			NextProjID: 1,
			NextHistID: 1,
		},
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	r.load()
	return r
}

func (r *FileRepository) load() {
	f, err := os.Open(r.path)
	if err != nil {
		return // пустой старт - файл появится после первой записи
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(r.data)
	if r.data.Projects == nil {
		r.data.Projects = make(map[int64]*MonitoringProject)
	}
	if r.data.Histories == nil {
		r.data.Histories = make(map[int64]*HistoryEntry)
	}
	// Восстанавливаем счётчики, чтобы не перезаписать существующие ID.
	for id := range r.data.Projects {
		if id >= r.data.NextProjID {
			r.data.NextProjID = id + 1
		}
	}
	for id := range r.data.Histories {
		if id >= r.data.NextHistID {
			r.data.NextHistID = id + 1
		}
	}
}

// save - вызывать ТОЛЬКО под r.mu. Атомарная запись через temp + rename.
func (r *FileRepository) save() {
	if dir := filepath.Dir(r.path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := r.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r.data); err != nil {
		f.Close()
		return
	}
	if err := f.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp, r.path)
}

// ---------------------------------------------------------------
// HistorySaver
// ---------------------------------------------------------------

// SaveResult - сохраняет запись в историю (авто-инкремент ID).
func (r *FileRepository) SaveResult(ctx context.Context, entry *HistoryEntry) error {
	if entry == nil {
		return fmt.Errorf("history entry is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	entry.ID = r.data.NextHistID
	r.data.NextHistID++
	if entry.Date.IsZero() {
		entry.Date = time.Now()
	}
	cp := *entry
	r.data.Histories[cp.ID] = &cp
	r.save()
	return nil
}

// ---------------------------------------------------------------
// Проекты
// ---------------------------------------------------------------

func (r *FileRepository) CreateProject(ctx context.Context, p *MonitoringProject) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if p.TelegramID == 0 {
		return fmt.Errorf("project telegram_id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	p.ID = r.data.NextProjID
	r.data.NextProjID++
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
	r.data.Projects[cp.ID] = &cp
	r.save()
	return nil
}

func (r *FileRepository) GetProject(ctx context.Context, id int64) (*MonitoringProject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.data.Projects[id]
	if !ok {
		return nil, fmt.Errorf("project not found")
	}
	cp := *p
	cp.EntryIDs = append([]int64{}, p.EntryIDs...)
	return &cp, nil
}

func (r *FileRepository) ListProjects(ctx context.Context, telegramID int64) ([]MonitoringProject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]MonitoringProject, 0, len(r.data.Projects))
	for _, p := range r.data.Projects {
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

func (r *FileRepository) CompleteProject(ctx context.Context, id int64, endDate time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.data.Projects[id]
	if !ok {
		return fmt.Errorf("project not found")
	}
	p.Status = ProjectStatusCompleted
	if !endDate.IsZero() {
		p.EndDate = endDate
	} else {
		p.EndDate = time.Now()
	}
	r.save()
	return nil
}

// ---------------------------------------------------------------
// Привязки записей к проектам
// ---------------------------------------------------------------

func (r *FileRepository) BindEntry(ctx context.Context, projectID, entryID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.data.Projects[projectID]
	if !ok {
		return fmt.Errorf("project not found")
	}
	if _, ok := r.data.Histories[entryID]; !ok {
		return fmt.Errorf("history entry not found")
	}
	for _, e := range p.EntryIDs {
		if e == entryID {
			return nil // уже привязано
		}
	}
	p.EntryIDs = append(p.EntryIDs, entryID)
	r.save()
	return nil
}

func (r *FileRepository) UnbindEntry(ctx context.Context, projectID, entryID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.data.Projects[projectID]
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
	r.save()
	return nil
}

func (r *FileRepository) ListProjectEntries(ctx context.Context, projectID int64) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.data.Projects[projectID]
	if !ok {
		return nil, fmt.Errorf("project not found")
	}
	out := append([]int64{}, p.EntryIDs...)
	return out, nil
}

// ---------------------------------------------------------------
// История
// ---------------------------------------------------------------

// ListHistory - история пользователя с фильтром по типу и пагинацией.
// page - 1-based; pageSize <= 0 означает «вернуть все».
func (r *FileRepository) ListHistory(ctx context.Context, telegramID int64, entryType string, page, pageSize int) ([]HistoryEntry, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := make([]HistoryEntry, 0, len(r.data.Histories))
	for _, h := range r.data.Histories {
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

func (r *FileRepository) GetHistoryEntry(ctx context.Context, id int64) (*HistoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	h, ok := r.data.Histories[id]
	if !ok {
		return nil, fmt.Errorf("history entry not found")
	}
	cp := *h
	return &cp, nil
}

// DeleteHistoryEntry - удаляет запись истории по ID и отвязывает её от
// всех проектов мониторинга (если была привязана).
func (r *FileRepository) DeleteHistoryEntry(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data.Histories[id]; !ok {
		return fmt.Errorf("history entry not found")
	}
	delete(r.data.Histories, id)
	// Отвязываем запись от всех проектов.
	for _, p := range r.data.Projects {
		filtered := p.EntryIDs[:0]
		for _, e := range p.EntryIDs {
			if e != id {
				filtered = append(filtered, e)
			}
		}
		p.EntryIDs = filtered
	}
	r.save()
	return nil
}

// DeleteByUser полностью удаляет проекты мониторинга и историю
// пользователя по Telegram ID (вместе с привязками записей в проектах).
func (r *FileRepository) DeleteByUser(ctx context.Context, telegramID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, p := range r.data.Projects {
		if p.TelegramID == telegramID {
			delete(r.data.Projects, id)
		}
	}
	for id, h := range r.data.Histories {
		if h.TelegramID == telegramID {
			delete(r.data.Histories, id)
		}
	}
	r.save()
	return nil
}

// compile-time проверка, что FileRepository реализует Repository.
var _ Repository = (*FileRepository)(nil)
