package monitoring

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// APIHandler - HTTP-обработчики API мониторинга. Все эндпоинты
// защищены валидацией Telegram initData; telegramID извлекается из него.
//
// Модель Free/Premium: мутирующие эндпоинты (создание проекта, привязка
// записи, завершение проекта) запрещены для не-Premium пользователей на
// уровне API - даже при прямом вызове. GET-эндпоинты доступны всем, но
// фронт для Free показывает заглушку вместо интерфейса.
type APIHandler struct {
	svc          *Service
	botToken     string
	premiumCheck func(telegramID int64) bool
	premiumLink  string
}

// NewAPIHandler создаёт обработчик API. premiumCheck возвращает true,
// если пользователь с данным telegramID имеет Premium-подписку.
func NewAPIHandler(svc *Service, botToken string, premiumCheck func(telegramID int64) bool) *APIHandler {
	if premiumCheck == nil {
		premiumCheck = func(int64) bool { return true }
	}
	return &APIHandler{
		svc:          svc,
		botToken:     botToken,
		premiumCheck: premiumCheck,
		premiumLink:  strings.TrimSpace(os.Getenv("WEBAPP_PREMIUM_LINK")),
	}
}

// Handler возвращает http.HandlerFunc, диспетчеризирующий маршруты
// под префиксом /api/monitoring.
func (h *APIHandler) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/monitoring")
		path = strings.Trim(path, "/")

		// Проверка подлинности (кроме случаев, когда маршрут не требует
		// пользователя - таких здесь нет, все защищены).
		telegramID, ok := h.auth(r)
		if !ok {
			h.writeError(w, http.StatusUnauthorized, "unauthorized: invalid initData")
			return
		}

		ctx := r.Context()

		switch {
		case path == "status" && r.Method == http.MethodGet:
			h.handleStatus(ctx, w, telegramID)

		case path == "projects" && r.Method == http.MethodGet:
			h.handleListProjects(ctx, w, telegramID)
		case path == "projects" && r.Method == http.MethodPost:
			h.handleCreateProject(ctx, w, r, telegramID)

		case path == "history" && r.Method == http.MethodGet:
			h.handleListHistory(ctx, w, r, telegramID)

		case strings.HasPrefix(path, "projects/"):
			rest := strings.TrimPrefix(path, "projects/")
			// projects/{id}/entries | projects/{id}/entries/{entryID} | projects/{id} | projects/{id}/complete
			parts := strings.Split(rest, "/")
			if len(parts) == 0 || parts[0] == "" {
				h.writeError(w, http.StatusNotFound, "not found")
				return
			}
			projectID, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				h.writeError(w, http.StatusBadRequest, "invalid project id")
				return
			}

			if len(parts) == 1 {
				if r.Method == http.MethodGet {
					h.handleProjectDetail(ctx, w, telegramID, projectID)
					return
				}
				h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}

			switch parts[1] {
			case "entries":
				if len(parts) == 2 && r.Method == http.MethodPost {
					h.handleBindEntry(ctx, w, r, telegramID, projectID)
					return
				}
				if len(parts) == 3 && r.Method == http.MethodDelete {
					entryID, err := strconv.ParseInt(parts[2], 10, 64)
					if err != nil {
						h.writeError(w, http.StatusBadRequest, "invalid entry id")
						return
					}
					h.handleUnbindEntry(ctx, w, telegramID, projectID, entryID)
					return
				}
			case "complete":
				if len(parts) == 2 && r.Method == http.MethodPost {
					h.handleCompleteProject(ctx, w, telegramID, projectID)
					return
				}
			}
			h.writeError(w, http.StatusNotFound, "not found")
		default:
			h.writeError(w, http.StatusNotFound, "not found")
		}
	}
}

// handleStatus возвращает флаг Premium-статуса и ссылку на оформление
// подписки - чтобы фронт мог показать заглушку Мониторинга для Free
// и кнопку «Открыть Мониторинг».
func (h *APIHandler) handleStatus(ctx context.Context, w http.ResponseWriter, telegramID int64) {
	isPremium := h.premiumCheck(telegramID)
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"isPremium":           isPremium,
		"monitoringAvailable": isPremium,
		"premiumLink":         h.premiumLink,
	})
}

// auth извлекает и валидирует initData (header или query-параметр).
func (h *APIHandler) auth(r *http.Request) (int64, bool) {
	initData := r.Header.Get("X-Telegram-Init-Data")
	src := "X-Telegram-Init-Data (header)"
	if initData == "" {
		initData = r.URL.Query().Get("initData")
		src = "initData (query)"
	}
	if initData == "" {
		log.Printf("[MONITORING-AUTH] отказ: initData НЕ передан ни в заголовке, ни в query. remote=%s", r.RemoteAddr)
		return 0, false
	}
	if h.botToken == "" {
		log.Printf("[MONITORING-AUTH] отказ: botToken на стороне API пуст (не сконфигурирован).")
		return 0, false
	}
	log.Printf("[MONITORING-AUTH] проверка: source=%q initDataLen=%d botTokenLen=%d remote=%s",
		src, len(initData), len(h.botToken), r.RemoteAddr)
	id, ok := ValidateInitData(initData, h.botToken)
	if !ok {
		return 0, false
	}
	if id == 0 {
		log.Printf("[MONITORING-AUTH] отказ: initData валиден, но telegramID=0 (нет поля user/id).")
		return 0, false
	}
	return id, true
}

// requirePremium - true, если мутирующее действие разрешено (Premium).
// Для Free возвращает 403 и false. Бэкенд НЕ доверяет demo-флагу при
// мутациях: Free-пользователь не может обойти гейт через ?demo=1,
// иначе прямой вызов /api/monitoring/projects создал бы проект.
func (h *APIHandler) requirePremium(w http.ResponseWriter, telegramID int64) bool {
	if h.premiumCheck(telegramID) {
		return true
	}
	h.writeError(w, http.StatusForbidden, "monitoring_requires_premium")
	return false
}

func (h *APIHandler) handleListProjects(ctx context.Context, w http.ResponseWriter, telegramID int64) {
	projects, err := h.svc.ListProjects(ctx, telegramID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, projects)
}

func (h *APIHandler) handleCreateProject(ctx context.Context, w http.ResponseWriter, r *http.Request, telegramID int64) {
	if !h.requirePremium(w, telegramID) {
		return
	}
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	p, err := h.svc.CreateProject(ctx, telegramID, req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[MONITORING] создан проект id=%d user=%d type=%s", p.ID, telegramID, p.Type)
	h.writeJSON(w, http.StatusCreated, p)
}

func (h *APIHandler) handleProjectDetail(ctx context.Context, w http.ResponseWriter, telegramID, projectID int64) {
	detail, err := h.svc.GetProjectDetail(ctx, telegramID, projectID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}

func (h *APIHandler) handleBindEntry(ctx context.Context, w http.ResponseWriter, r *http.Request, telegramID, projectID int64) {
	if !h.requirePremium(w, telegramID) {
		return
	}
	var req BindEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EntryID == 0 {
		h.writeError(w, http.StatusBadRequest, "invalid JSON или entry_id")
		return
	}
	if err := h.svc.BindEntry(ctx, telegramID, projectID, req.EntryID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleUnbindEntry(ctx context.Context, w http.ResponseWriter, telegramID, projectID, entryID int64) {
	if !h.requirePremium(w, telegramID) {
		return
	}
	if err := h.svc.UnbindEntry(ctx, telegramID, projectID, entryID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleCompleteProject(ctx context.Context, w http.ResponseWriter, telegramID, projectID int64) {
	if !h.requirePremium(w, telegramID) {
		return
	}
	if err := h.svc.CompleteProject(ctx, telegramID, projectID); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleListHistory(ctx context.Context, w http.ResponseWriter, r *http.Request, telegramID int64) {
	q := r.URL.Query()
	entryType := q.Get("type")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	resp, err := h.svc.ListHistory(ctx, telegramID, entryType, page, pageSize)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *APIHandler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, ErrorResponse{Error: msg})
}
