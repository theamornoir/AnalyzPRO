package monitoring

import "time"

// ============================================================
// Типы мониторинга
// ============================================================
// ProjectType - тип проекта мониторинга. Список расширяемый: добавьте
// новую константу и она автоматически станет доступна в веб-аппе
// (фронтенд берёт лейблы из PROJECT_TYPES, бэкенд валидирует через
// IsValidProjectType). Логика работы с проектом от типа не зависит.
const (
	ProjectTypeCourse   = "course"   // Курс препаратов
	ProjectTypeDiabetes = "diabetes" // Диабет
	ProjectTypeWeight   = "weight"   // Похудение
	ProjectTypeHealth   = "health"   // Общее здоровье
	ProjectTypeOther    = "other"    // Другое
)

// projectTypeLabels - человекочитаемые названия типов (для API/веб-аппа).
var projectTypeLabels = map[string]string{
	ProjectTypeCourse:   "Курс препаратов",
	ProjectTypeDiabetes: "Диабет",
	ProjectTypeWeight:   "Похудение",
	ProjectTypeHealth:   "Общее здоровье",
	ProjectTypeOther:    "Другое",
}

// ProjectTypes - возвращает все допустимые типы с лейблами. Используется
// бэкендом для валидации и веб-аппом для отрисовки выпадающего списка.
func ProjectTypes() []ProjectTypeInfo {
	return []ProjectTypeInfo{
		{Value: ProjectTypeCourse, Label: projectTypeLabels[ProjectTypeCourse]},
		{Value: ProjectTypeDiabetes, Label: projectTypeLabels[ProjectTypeDiabetes]},
		{Value: ProjectTypeWeight, Label: projectTypeLabels[ProjectTypeWeight]},
		{Value: ProjectTypeHealth, Label: projectTypeLabels[ProjectTypeHealth]},
		{Value: ProjectTypeOther, Label: projectTypeLabels[ProjectTypeOther]},
	}
}

// IsValidProjectType - true, если тип входит в допустимый набор.
func IsValidProjectType(t string) bool {
	_, ok := projectTypeLabels[t]
	return ok
}

// ProjectTypeLabel - лейбл типа или пустая строка, если тип неизвестен.
func ProjectTypeLabel(t string) string {
	return projectTypeLabels[t]
}

// ProjectTypeInfo - описание типа мониторинга для API.
type ProjectTypeInfo struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ============================================================
// Статусы проекта
// ============================================================
const (
	ProjectStatusActive    = "active"
	ProjectStatusCompleted = "completed"
)

// ============================================================
// Модели данных
// ============================================================

// MonitoringProject - проект мониторинга пользователя.
type MonitoringProject struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	StartDate  time.Time `json:"start_date"`
	// EndDate - дата окончания. Пустое (zero) значение = проект без
	// фиксированного окончания (открытый мониторинг).
	EndDate   time.Time `json:"end_date,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// EntryIDs - идентификаторы привязанных записей истории.
	EntryIDs []int64 `json:"entry_ids"`
}

// HistoryEntry - одна запись в истории пользователя
// (анализ крови, биоскан, опросник). Единый источник данных для любого
// проекта мониторинга.
type HistoryEntry struct {
	ID         int64 `json:"id"`
	TelegramID int64 `json:"telegram_id"`
	// Type - "analysis" | "bioscan" | "questionnaire".
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Date       time.Time `json:"date"`
	JsonData   string    `json:"json_data"`
	ReportHTML string    `json:"report_html"`
}

// EntryView - запись истории, обогащённая извлечёнными показателями,
// готовая для отрисовки графиков на фронтенде.
type EntryView struct {
	ID      int64              `json:"id"`
	Type    string             `json:"type"`
	Title   string             `json:"title"`
	Date    time.Time          `json:"date"`
	Metrics map[string]float64 `json:"metrics"`
}

// ProjectDetail - полная информация о проекте для веб-аппа:
// сам проект, привязанные записи (с показателями) и список доступных
// для графиков метрик (объединение ключей всех записей).
type ProjectDetail struct {
	Project          *MonitoringProject `json:"project"`
	Entries          []EntryView        `json:"entries"`
	AvailableMetrics []string           `json:"available_metrics"`
}

// CreateProjectRequest - тело запроса на создание проекта.
type CreateProjectRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	StartDate string `json:"start_date"` // формат YYYY-MM-DD
	EndDate   string `json:"end_date"`   // формат YYYY-MM-DD или "" (необязательно)
}

// BindEntryRequest - тело запроса на привязку записи к проекту.
type BindEntryRequest struct {
	EntryID int64 `json:"entry_id"`
}

// HistoryListResponse - ответ списка истории с пагинацией.
type HistoryListResponse struct {
	Entries   []HistoryEntry `json:"entries"`
	Total     int            `json:"total"`
	Page      int            `json:"page"`
	PageSize  int            `json:"page_size"`
	PageCount int            `json:"page_count"`
}

// ErrorResponse - стандартный ответ с ошибкой.
type ErrorResponse struct {
	Error string `json:"error"`
}
