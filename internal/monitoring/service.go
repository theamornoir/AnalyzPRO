package monitoring

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Service - бизнес-логика модуля мониторинга. Оперирует Repository
// (проекты + история) и извлекает показатели для графиков.
type Service struct {
	repo Repository
}

// NewService создаёт сервис мониторинга.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListProjects - список проектов пользователя.
func (s *Service) ListProjects(ctx context.Context, telegramID int64) ([]MonitoringProject, error) {
	return s.repo.ListProjects(ctx, telegramID)
}

// CreateProject - создаёт новый проект мониторинга.
func (s *Service) CreateProject(ctx context.Context, telegramID int64, req CreateProjectRequest) (*MonitoringProject, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("название проекта обязательно")
	}
	if !IsValidProjectType(req.Type) {
		return nil, fmt.Errorf("неизвестный тип мониторинга: %q", req.Type)
	}

	start, err := parseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("некорректная дата начала (ожидается YYYY-MM-DD): %w", err)
	}

	end := time.Time{}
	if strings.TrimSpace(req.EndDate) != "" {
		end, err = parseDate(req.EndDate)
		if err != nil {
			return nil, fmt.Errorf("некорректная дата окончания (ожидается YYYY-MM-DD): %w", err)
		}
		if end.Before(start) {
			return nil, fmt.Errorf("дата окончания раньше даты начала")
		}
	}

	p := &MonitoringProject{
		TelegramID: telegramID,
		Name:       name,
		Type:       req.Type,
		StartDate:  start,
		EndDate:    end,
		Status:     ProjectStatusActive,
		EntryIDs:   []int64{},
	}
	if err := s.repo.CreateProject(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProjectDetail - детальная информация о проекте: проект, привязанные
// записи (с извлечёнными показателями) и объединённый список доступных
// метрик. Проверяет, что проект принадлежит пользователю.
func (s *Service) GetProjectDetail(ctx context.Context, telegramID, projectID int64) (*ProjectDetail, error) {
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if p.TelegramID != telegramID {
		return nil, fmt.Errorf("проект не принадлежит пользователю")
	}

	entryIDs := p.EntryIDs
	entries := make([]EntryView, 0, len(entryIDs))
	metricSet := map[string]struct{}{}

	for _, id := range entryIDs {
		h, err := s.repo.GetHistoryEntry(ctx, id)
		if err != nil {
			continue // запись могла быть удалена - пропускаем
		}
		metrics := extractMetrics(h.JsonData)
		entries = append(entries, EntryView{
			ID:      h.ID,
			Type:    h.Type,
			Title:   h.Title,
			Date:    h.Date,
			Metrics: metrics,
		})
		for k := range metrics {
			metricSet[k] = struct{}{}
		}
	}

	// Сортируем записи по дате (старые → новые) для корректных графиков.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Date.Before(entries[j].Date)
	})

	available := make([]string, 0, len(metricSet))
	for k := range metricSet {
		available = append(available, k)
	}
	sort.Strings(available)

	return &ProjectDetail{
		Project:          p,
		Entries:          entries,
		AvailableMetrics: available,
	}, nil
}

// BindEntry - привязывает запись истории к проекту (с проверкой владения).
func (s *Service) BindEntry(ctx context.Context, telegramID, projectID, entryID int64) error {
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if p.TelegramID != telegramID {
		return fmt.Errorf("проект не принадлежит пользователю")
	}
	h, err := s.repo.GetHistoryEntry(ctx, entryID)
	if err != nil {
		return err
	}
	if h.TelegramID != telegramID {
		return fmt.Errorf("запись не принадлежит пользователю")
	}
	return s.repo.BindEntry(ctx, projectID, entryID)
}

// UnbindEntry - отвязывает запись от проекта (с проверкой владения).
func (s *Service) UnbindEntry(ctx context.Context, telegramID, projectID, entryID int64) error {
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if p.TelegramID != telegramID {
		return fmt.Errorf("проект не принадлежит пользователю")
	}
	return s.repo.UnbindEntry(ctx, projectID, entryID)
}

// CompleteProject - завершает проект мониторинга.
func (s *Service) CompleteProject(ctx context.Context, telegramID, projectID int64) error {
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if p.TelegramID != telegramID {
		return fmt.Errorf("проект не принадлежит пользователю")
	}
	return s.repo.CompleteProject(ctx, projectID, time.Time{})
}

// ListHistory - история пользователя с фильтром по типу и пагинацией.
func (s *Service) ListHistory(ctx context.Context, telegramID int64, entryType string, page, pageSize int) (*HistoryListResponse, error) {
	entries, total, err := s.repo.ListHistory(ctx, telegramID, entryType, page, pageSize)
	if err != nil {
		return nil, err
	}
	pageCount := 0
	if pageSize > 0 {
		pageCount = (total + pageSize - 1) / pageSize
	}
	return &HistoryListResponse{
		Entries:   entries,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		PageCount: pageCount,
	}, nil
}

// parseDate парсит дату в формате YYYY-MM-DD.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(s))
}
