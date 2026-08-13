package monitoring

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// buildInitData строит валидный initData тем же алгоритмом, что Telegram.
func buildInitData(botToken string, userID int64) string {
	values := url.Values{}
	values.Set("id", strconv.FormatInt(userID, 10))
	values.Set("first_name", "Tester")
	values.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	values.Set("user", `{"id":`+strconv.FormatInt(userID, 10)+`,"first_name":"Tester"}`)

	keys := []string{"auth_date", "first_name", "id", "user"}
	dataCheck := ""
	for i, k := range keys {
		if i > 0 {
			dataCheck += "\n"
		}
		dataCheck += k + "=" + values.Get(k)
	}
	// Официальный алгоритм Telegram: secret_key = HMAC_SHA256(bot_token, "WebAppData").
	secret := hmac.New(sha256.New, []byte(botToken))
	secret.Write([]byte("WebAppData"))
	computed := hmac.New(sha256.New, secret.Sum(nil))
	computed.Write([]byte(dataCheck))
	values.Set("hash", hex.EncodeToString(computed.Sum(nil)))
	return values.Encode()
}

func TestExtractMetrics(t *testing.T) {
	jsonStr := `{
		"patient": {"name": "Ivan"},
		"markers": [
			{"name": "Глюкоза", "value": 5.4},
			{"name": "Гемоглобин", "value": 150}
		],
		"summary": {"score": 80}
	}`
	m := extractMetrics(jsonStr)
	if m["Глюкоза"] != 5.4 {
		t.Errorf("expected Глюкоза=5.4, got %v", m["Глюкоза"])
	}
	if m["Гемоглобин"] != 150 {
		t.Errorf("expected Гемоглобин=150, got %v", m["Гемоглобин"])
	}
	if m["summary / score"] != 80 {
		t.Errorf("expected summary / score=80, got %v", m["summary / score"])
	}
}

func TestServiceFlow(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo)

	repo.SaveResult(ctx, &HistoryEntry{TelegramID: 111, Type: "analysis", Title: "A1", Date: time.Now().Add(-48 * time.Hour), JsonData: `{"markers":[{"name":"Глюкоза","value":6.1}]}`})
	repo.SaveResult(ctx, &HistoryEntry{TelegramID: 111, Type: "analysis", Title: "A2", Date: time.Now(), JsonData: `{"markers":[{"name":"Глюкоза","value":5.2}]}`})
	repo.SaveResult(ctx, &HistoryEntry{TelegramID: 999, Type: "analysis", Title: "Other", JsonData: `{"markers":[{"name":"Глюкоза","value":9.9}]}`})

	hist, _ := svc.ListHistory(ctx, 111, "", 1, 50)
	if len(hist.Entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(hist.Entries))
	}

	p, err := svc.CreateProject(ctx, 111, CreateProjectRequest{Name: "Course", Type: "course", StartDate: "2026-01-01"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if p.Status != ProjectStatusActive {
		t.Errorf("expected active, got %s", p.Status)
	}

	if _, err := svc.CreateProject(ctx, 111, CreateProjectRequest{Name: "X", Type: "bogus", StartDate: "2026-01-01"}); err == nil {
		t.Error("expected error for invalid type")
	}

	if err := svc.BindEntry(ctx, 111, p.ID, hist.Entries[0].ID); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := svc.BindEntry(ctx, 111, p.ID, hist.Entries[1].ID); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := svc.BindEntry(ctx, 111, p.ID, 999999); err == nil {
		t.Error("expected error binding non-existent entry")
	}

	detail, err := svc.GetProjectDetail(ctx, 111, p.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(detail.Entries) != 2 {
		t.Errorf("expected 2 bound entries, got %d", len(detail.Entries))
	}
	if len(detail.AvailableMetrics) == 0 {
		t.Error("expected available metrics")
	}
	if detail.Entries[0].Date.After(detail.Entries[1].Date) {
		t.Error("entries must be sorted ascending by date")
	}

	if err := svc.CompleteProject(ctx, 111, p.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	updated, _ := repo.GetProject(ctx, p.ID)
	if updated.Status != ProjectStatusCompleted {
		t.Errorf("expected completed, got %s", updated.Status)
	}

	if _, err := svc.GetProjectDetail(ctx, 222, p.ID); err == nil {
		t.Error("expected access error for other user")
	}
}

func TestValidateInitData(t *testing.T) {
	botToken := "TEST_BOT_TOKEN"
	userID := int64(777)

	// Используем тот же алгоритм, что применяется в real Telegram (и в buildInitData),
	// чтобы проверить корректность ValidateInitData на реальном порядке HMAC.
	values := url.Values{}
	values.Set("id", "777")
	values.Set("first_name", "Tester")
	values.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	values.Set("user", `{"id":777,"first_name":"Tester"}`)

	keys := []string{"auth_date", "first_name", "id", "user"}
	dataCheck := ""
	for i, k := range keys {
		if i > 0 {
			dataCheck += "\n"
		}
		dataCheck += k + "=" + values.Get(k)
	}
	// secret_key = HMAC_SHA256(bot_token, "WebAppData")
	secret := hmac.New(sha256.New, []byte(botToken))
	secret.Write([]byte("WebAppData"))
	computed := hmac.New(sha256.New, secret.Sum(nil))
	computed.Write([]byte(dataCheck))
	values.Set("hash", hex.EncodeToString(computed.Sum(nil)))

	initData := values.Encode()

	gotID, ok := ValidateInitData(initData, botToken)
	if !ok {
		t.Fatal("expected valid initData")
	}
	if gotID != userID {
		t.Errorf("expected userID=%d, got %d", userID, gotID)
	}
	if _, ok := ValidateInitData(initData, "WRONG"); ok {
		t.Error("expected rejection for wrong token")
	}
	if _, ok := ValidateInitData("", botToken); ok {
		t.Error("expected rejection for empty initData")
	}
}

func TestAPIHandler(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo)
	botToken := "TEST_BOT_TOKEN"
	h := NewAPIHandler(svc, botToken).Handler()

	initData := buildInitData(botToken, 111)
	authHeader := func(req *http.Request) { req.Header.Set("X-Telegram-Init-Data", initData) }

	// Без initData → 401.
	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/projects", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401 без initData, получили %d", rec.Code)
	}

	// Создание проекта (POST) с валидным initData.
	body, _ := json.Marshal(CreateProjectRequest{Name: "Курс", Type: "course", StartDate: "2026-01-01"})
	req = httptest.NewRequest(http.MethodPost, "/api/monitoring/projects", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	authHeader(req)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201 при создании, получили %d: %s", rec.Code, rec.Body.String())
	}
	var created MonitoringProject
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Fatal("ожидали ID проекта")
	}

	// Список проектов.
	req = httptest.NewRequest(http.MethodGet, "/api/monitoring/projects", nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для списка, получили %d", rec.Code)
	}
	var list []MonitoringProject
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("ожидали 1 проект в списке, получили %d", len(list))
	}

	// Деталь проекта.
	req = httptest.NewRequest(http.MethodGet, "/api/monitoring/projects/"+strconv.FormatInt(created.ID, 10), nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для детали, получили %d", rec.Code)
	}

	// История (пустая, но 200).
	req = httptest.NewRequest(http.MethodGet, "/api/monitoring/history", nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для истории, получили %d", rec.Code)
	}
	var hist HistoryListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &hist)
	if hist.Page != 1 {
		t.Errorf("ожидали page=1, получили %d", hist.Page)
	}

	_ = ctx
}

func TestValidateInitDataAltOrder(t *testing.T) {
	botToken := "TEST_BOT_TOKEN"
	userID := int64(777)

	// Подпишем initData АЛЬТЕРНАТИВНЫМ порядком HMAC (key="WebAppData"),
	// который встречается в эталонной библиотеке init-data-golang.
	values := url.Values{}
	values.Set("id", "777")
	values.Set("first_name", "Tester")
	values.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	values.Set("user", `{"id":777,"first_name":"Tester"}`)

	keys := []string{"auth_date", "first_name", "id", "user"}
	dataCheck := ""
	for i, k := range keys {
		if i > 0 {
			dataCheck += "\n"
		}
		dataCheck += k + "=" + values.Get(k)
	}
	// secret = HMAC_SHA256("WebAppData", bot_token)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	computed := hmac.New(sha256.New, secret.Sum(nil))
	computed.Write([]byte(dataCheck))
	values.Set("hash", hex.EncodeToString(computed.Sum(nil)))

	initData := values.Encode()

	// Валидатор должен принять initData, подписанный альтернативным порядком.
	gotID, ok := ValidateInitData(initData, botToken)
	if !ok {
		t.Fatal("expected valid initData signed with alternative HMAC order")
	}
	if gotID != userID {
		t.Errorf("expected userID=%d, got %d", userID, gotID)
	}

	// Неправильный токен должен отклоняться при обоих порядках.
	if _, ok := ValidateInitData(initData, "WRONG_TOKEN"); ok {
		t.Error("expected rejection for wrong token")
	}
}

func TestServeWebApp(t *testing.T) {
	// Главная страница.
	req := httptest.NewRequest(http.MethodGet, "/monitoring/", nil)
	rec := httptest.NewRecorder()
	ServeWebApp(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для /monitoring/, получили %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("ожидали text/html, получили %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "Мониторинг") {
		t.Error("ожидали содержимое веб-аппа")
	}

	// Статика: app.js.
	req = httptest.NewRequest(http.MethodGet, "/monitoring/app.js", nil)
	rec = httptest.NewRecorder()
	ServeWebApp(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Errorf("ожидали JS-файл, код=%d тип=%s", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Неизвестный файл → 404.
	req = httptest.NewRequest(http.MethodGet, "/monitoring/secret.txt", nil)
	rec = httptest.NewRecorder()
	ServeWebApp(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("ожидали 404, получили %d", rec.Code)
	}
}
