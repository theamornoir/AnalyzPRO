// Integration-тест дашборда: реальная БД (SQLite), гейт по Premium и
// подлинность initData, и построение метрик из истории пользователя.
package dashboard

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	monitoring_sqlrepo "github.com/theamornoir/analyzpro/internal/monitoring/sqlrepo"
	"github.com/theamornoir/analyzpro/internal/payment"
)

const testBotToken = "TEST_BOT_TOKEN_DASHBOARD"

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
	secret := hmac.New(sha256.New, []byte(botToken))
	secret.Write([]byte("WebAppData"))
	computed := hmac.New(sha256.New, secret.Sum(nil))
	computed.Write([]byte(dataCheck))
	values.Set("hash", hex.EncodeToString(computed.Sum(nil)))
	return values.Encode()
}

func newHandler(t *testing.T) (*Handler, int64) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	repo := monitoring_sqlrepo.New(conn)

	pay := payment.NewMockPaymentService("")
	h := NewHandler(pay, testBotToken, repo, nil, nil, "development")
	return h, int64(999)
}

// seedAnalysis сохраняет запись анализа крови для проверки построения метрик.
func seedAnalysis(t *testing.T, h *Handler, uid int64) {
	t.Helper()
	ctx := context.Background()
	if err := h.repo.SaveResult(ctx, &monitoring.HistoryEntry{
		TelegramID: uid,
		Type:       "analysis",
		Title:      "Общий анализ",
		Date:       time.Now(),
		JsonData: `{
			"profile": {"name": "Иван", "age": 30, "composition": 80, "potential": 70},
			"categories": [{"name":"Кровь","indicators":[
				{"name":"Гемоглобин","value":"145","status":"normal"},
				{"name":"Лейкоциты","value":"6.2","status":"normal"}
			]}],
			"recommendations": ["Пить больше воды", "Гулять 30 минут"]
		}`,
	}); err != nil {
		t.Fatalf("seed analysis: %v", err)
	}
}

func TestMetricsUnauthorized(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics?initData=garbage", nil)
	w := httptest.NewRecorder()
	h.Metrics(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401, получили %d", w.Code)
	}
}

func TestMetricsPremiumRequired(t *testing.T) {
	h, uid := newHandler(t)
	initData := buildInitData(testBotToken, uid)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics?initData="+url.QueryEscape(initData), nil)
	w := httptest.NewRecorder()
	h.Metrics(w, req)
	// Онбординг доступен ВСЕМ: не-Premium получает 200 (а не 403), помечен
	// premiumRequired и видит noData — чтобы Mini App показал форму профиля,
	// а не пустой экран.
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 (онбординг для всех), получили %d: %s", w.Code, w.Body.String())
	}
	var resp MetricsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("декод JSON: %v", err)
	}
	if !resp.PremiumRequired {
		t.Error("ожидали PremiumRequired=true для не-Premium")
	}
	if !resp.NoData {
		t.Error("ожидали NoData=true при пустой истории")
	}
}

func TestMetricsOK(t *testing.T) {
	h, uid := newHandler(t)
	// Засеваем анализ крови, чтобы дашборд был непустым.
	seedAnalysis(t, h, uid)
	// Активируем Premium для пользователя.
	h.pay.ActivatePremiumManually(uid, "premium_monthly")

	initData := buildInitData(testBotToken, uid)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics?initData="+url.QueryEscape(initData), nil)
	w := httptest.NewRecorder()
	h.Metrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", w.Code, w.Body.String())
	}

	var resp MetricsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("декод JSON: %v", err)
	}
	if resp.NoData {
		t.Error("NoData=true при наличии истории")
	}
	if resp.UserName != "Иван" {
		t.Errorf("UserName=%q, want Иван", resp.UserName)
	}
	if resp.Blood.Hemoglobin != 145 {
		t.Errorf("Hemoglobin=%d, want 145", resp.Blood.Hemoglobin)
	}
	if resp.HealthIndex <= 0 {
		t.Errorf("HealthIndex=%d, ожидали > 0", resp.HealthIndex)
	}
	if len(resp.Recommendations) != 2 {
		t.Errorf("Recommendations=%v, ожидали 2", resp.Recommendations)
	}
	if len(resp.Trend.Values) != 1 {
		t.Errorf("Trend.Values=%v, ожидали 1 точку", resp.Trend.Values)
	}
}

func TestSaveProfileSeedsDashboard(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")

	initData := buildInitData(testBotToken, uid)

	// До сохранения профиля — NoData=true.
	before := httptest.NewRequest(http.MethodGet, "/api/metrics?initData="+url.QueryEscape(initData), nil)
	wb := httptest.NewRecorder()
	h.Metrics(wb, before)
	var beforeResp MetricsResponse
	_ = json.Unmarshal(wb.Body.Bytes(), &beforeResp)
	if !beforeResp.NoData {
		t.Fatalf("до регистрации ожидали NoData=true")
	}

	// Сохраняем профиль через POST /api/profile.
	body := `{"name":"Анна","age":28,"gender":"Женский","height":168,"weight":60,"goal":"Энергия"}`
	preq := httptest.NewRequest(http.MethodPost, "/api/profile?initData="+url.QueryEscape(initData), strings.NewReader(body))
	preq.Header.Set("Content-Type", "application/json")
	pr := httptest.NewRecorder()
	h.SaveProfile(pr, preq)
	if pr.Code != http.StatusOK {
		t.Fatalf("SaveProfile вернул %d: %s", pr.Code, pr.Body.String())
	}

	// После сохранения профиля — NoData=false и подтянулось имя.
	after := httptest.NewRequest(http.MethodGet, "/api/metrics?initData="+url.QueryEscape(initData), nil)
	wa := httptest.NewRecorder()
	h.Metrics(wa, after)
	if wa.Code != http.StatusOK {
		t.Fatalf("Metrics после регистрации вернул %d", wa.Code)
	}
	var afterResp MetricsResponse
	if err := json.Unmarshal(wa.Body.Bytes(), &afterResp); err != nil {
		t.Fatalf("декод JSON: %v", err)
	}
	if afterResp.NoData {
		t.Error("NoData=true после сохранения профиля")
	}
	if afterResp.UserName != "Анна" {
		t.Errorf("UserName=%q, want Анна", afterResp.UserName)
	}
	if afterResp.UserAge != 28 {
		t.Errorf("UserAge=%d, want 28", afterResp.UserAge)
	}
	if len(afterResp.Recommendations) == 0 {
		t.Error("ожидали рекомендации из профиля")
	}
	if afterResp.HealthIndex <= 0 {
		t.Errorf("HealthIndex=%d, ожидали > 0", afterResp.HealthIndex)
	}
}

func TestSaveProfileValidation(t *testing.T) {
	h, uid := newHandler(t)
	initData := buildInitData(testBotToken, uid)

	// Невалидное имя (слишком короткое).
	bad := httptest.NewRequest(http.MethodPost, "/api/profile?initData="+url.QueryEscape(initData),
		strings.NewReader(`{"name":"А","age":28}`))
	bad.Header.Set("Content-Type", "application/json")
	rb := httptest.NewRecorder()
	h.SaveProfile(rb, bad)
	if rb.Code != http.StatusBadRequest {
		t.Errorf("ожидали 400 на короткое имя, получили %d", rb.Code)
	}

	// Без initData — 401.
	noAuth := httptest.NewRequest(http.MethodPost, "/api/profile", strings.NewReader(`{"name":"Анна","age":28}`))
	noAuth.Header.Set("Content-Type", "application/json")
	ra := httptest.NewRecorder()
	h.SaveProfile(ra, noAuth)
	if ra.Code != http.StatusUnauthorized {
		t.Errorf("ожидали 401 без initData, получили %d", ra.Code)
	}
}

// fakePDF — тестовый конвертер html2pdf: возвращает заданные байты/ошибку.
type fakePDF struct {
	bytes []byte
	err   error
}

func (f fakePDF) ConvertHTML(ctx context.Context, html string) ([]byte, error) {
	return f.bytes, f.err
}

// seedReport сохраняет запись отчёта (analysis/bioscan) с готовым ReportHTML,
// чтобы ReportFile мог отдать её без перерендера через report.Renderer.
func seedReport(t *testing.T, h *Handler, uid int64, typ, html string) int64 {
	t.Helper()
	if err := h.repo.SaveResult(context.Background(), &monitoring.HistoryEntry{
		TelegramID: uid,
		Type:       typ,
		Title:      "Тест-отчёт",
		Date:       time.Now(),
		JsonData:   "{\"title\":\"Тест-отчёт\"}",
		ReportHTML: html,
	}); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	// Достаём сохранённый id через историю.
	entries, _, err := h.repo.ListHistory(context.Background(), uid, typ, 1, 0)
	if err != nil || len(entries) == 0 {
		t.Fatalf("не удалось прочитать сохранённый отчёт: %v", err)
	}
	return entries[0].ID
}

func TestReportFileNonPremiumCanOpen(t *testing.T) {
	h, uid := newHandler(t)
	// Свой сохранённый отчёт открывает ЛЮБОЙ пользователь (Premium-гейт на
	// открытие снят — клик по карточке отчёта в «Мой профиль» должен
	// открывать файл независимо от тарифа). Конвертер отдаёт PDF-байты.
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 fake pdf content"), err: nil}
	id := seedReport(t, h, uid, "analysis", "<html>report</html>")
	initData := buildInitData(testBotToken, uid)
	u := "/api/reports/file?initData=" + url.QueryEscape(initData) +
		"&type=analysis&id=" + strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для своего отчёта (без Premium), получили %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type=%q, want application/pdf", ct)
	}
}

// TestReportFileFreeAccessWindow проверяет гейт окна «3 последние записи»
// для Free: запрос отчёта вне тройки последних возвращает 403, запрос
// одной из 3 последних — 200. После активации Premium ограничение снимается.
func TestReportFileFreeAccessWindow(t *testing.T) {
	h, uid := newHandler(t)
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 fake"), err: nil}

	// Free-пользователь (без Premium). Сохраняем 5 анализов с разными
	// датами (самый свежий — first), чтобы окно топ-3 было детерминированным.
	ctx := context.Background()
	base := time.Now()
	ids := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		entry := &monitoring.HistoryEntry{
			TelegramID: uid,
			Type:       "analysis",
			Title:      "Анализ",
			Date:       base.AddDate(0, 0, -i),
			JsonData:   "{\"title\":\"Тест\"}",
			ReportHTML: "<html>report</html>",
		}
		if err := h.repo.SaveResult(ctx, entry); err != nil {
			t.Fatalf("seed: %v", err)
		}
		ids = append(ids, entry.ID)
	}

	initData := buildInitData(testBotToken, uid)
	reqURL := func(id int64) string {
		return "/api/reports/file?initData=" + url.QueryEscape(initData) +
			"&type=analysis&id=" + strconv.FormatInt(id, 10)
	}

	// Самый свежий отчёт (в тройке) — 200.
	wOK := httptest.NewRecorder()
	h.ReportFile(wOK, httptest.NewRequest(http.MethodGet, reqURL(ids[0]), nil))
	if wOK.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для свежего отчёта в тройке, получили %d: %s", wOK.Code, wOK.Body.String())
	}

	// 4-й по свежести (вне тройки) — 403 для Free.
	w403 := httptest.NewRecorder()
	h.ReportFile(w403, httptest.NewRequest(http.MethodGet, reqURL(ids[3]), nil))
	if w403.Code != http.StatusForbidden {
		t.Fatalf("ожидали 403 для скрытого отчёта Free, получили %d: %s", w403.Code, w403.Body.String())
	}

	// После активации Premium тот же скрытый id открывается (200).
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	wPrem := httptest.NewRecorder()
	h.ReportFile(wPrem, httptest.NewRequest(http.MethodGet, reqURL(ids[3]), nil))
	if wPrem.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для скрытого отчёта после Premium, получили %d", wPrem.Code)
	}
}

func TestReportFileServesPDF(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	// Конвертер возвращает PDF-байты.
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 fake pdf content"), err: nil}
	id := seedReport(t, h, uid, "analysis", "<html><body>Тестовый отчёт</body></html>")
	initData := buildInitData(testBotToken, uid)
	u := "/api/reports/file?initData=" + url.QueryEscape(initData) +
		"&type=analysis&id=" + strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type=%q, want application/pdf", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "inline") {
		t.Errorf("Content-Disposition=%q, want inline", cd)
	}
	if !strings.HasPrefix(w.Body.String(), "%PDF") {
		t.Errorf("тело не начинается с %%PDF: %q", w.Body.String())
	}
}

func TestReportFileHTMLFallback(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	// Конвертер недоступен — откат к HTML.
	h.pdfConverter = fakePDF{err: fmt.Errorf("pdf unavailable")}
	id := seedReport(t, h, uid, "bioscan", "<html><body>Body Scan</body></html>")
	initData := buildInitData(testBotToken, uid)
	u := "/api/reports/file?initData=" + url.QueryEscape(initData) +
		"&type=bioscan&id=" + strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 (HTML fallback), получили %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type=%q, want text/html; charset=utf-8", ct)
	}
	if !strings.Contains(w.Body.String(), "Body Scan") {
		t.Errorf("тело не содержит исходный HTML: %q", w.Body.String())
	}
}

func TestReportFileForbiddenOtherUser(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	h.pdfConverter = fakePDF{bytes: []byte("%PDF"), err: nil}
	id := seedReport(t, h, uid, "analysis", "<html>report</html>")
	// initData другого пользователя (999 != uid) — запись ему не принадлежит.
	otherData := buildInitData(testBotToken, 12345)
	u := "/api/reports/file?initData=" + url.QueryEscape(otherData) +
		"&type=analysis&id=" + strconv.FormatInt(id, 10)
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 403/404 для чужой записи, получили %d", w.Code)
	}
}

// TestReportFileDemoServesHTMLWithoutAuth — демо-отчёт (?demo=1) открывается
// БЕЗ initData и БЕЗ записи в БД: при недоступности PDF-конвертера отдаётся
// синтетический HTML (inline). Это путь кнопки «📄 PDF» в демо-Сводке.
func TestReportFileDemoServesHTMLWithoutAuth(t *testing.T) {
	h, _ := newHandler(t)
	// Имитируем отсутствие ключа html2pdf.app — откат к HTML.
	h.pdfConverter = fakePDF{err: fmt.Errorf("no key")}
	// БЕЗ initData — демо не требует подлинности сессии.
	u := "/api/reports/file?demo=1&type=analysis&id=1"
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для демо-отчёта, получили %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("ожидали text/html, получили %q", ct)
	}
	if !strings.Contains(w.Body.String(), "Демонстрационный отчёт") {
		t.Fatalf("тело не содержит демо-отчёт: %q", w.Body.String()[:min(200, w.Body.Len())])
	}
}

// TestReportFileDemoServesPDF — при наличии PDF-конвертера демо-отчёт
// отдаётся как application/pdf (bioscan, без initData).

// TestReportFileInlineServesHTML — при ?view=inline бэкенд ВСЕГДА отдаёт
// HTML (даже если PDF-конвертер доступен). Это путь встроенного просмотрщика
// (iframe внутри Mini App), который не триггерит окно «посетить сайт».
func TestReportFileInlineServesHTML(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	// Конвертер ДОСТУПЕН — но view=inline должен отдать HTML, не PDF.
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 fake"), err: nil}
	id := seedReport(t, h, uid, "analysis", "<html><body>Встроенный отчёт</body></html>")
	initData := buildInitData(testBotToken, uid)
	u := "/api/reports/file?initData=" + url.QueryEscape(initData) +
		"&type=analysis&id=" + strconv.FormatInt(id, 10) + "&view=inline"
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type=%q, want text/html; charset=utf-8", ct)
	}
	if !strings.Contains(w.Body.String(), "Встроенный отчёт") {
		t.Errorf("тело не содержит HTML отчёта: %q", w.Body.String())
	}
}

// TestReportFileDemoInlineServesHTML — демо-отчёт (?demo=1) при ?view=inline
// отдаёт HTML без initData, даже если PDF-конвертер вернул бы PDF.
func TestReportFileDemoInlineServesHTML(t *testing.T) {
	h, _ := newHandler(t)
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 demo"), err: nil}
	u := "/api/reports/file?demo=1&type=bioscan&id=10&view=inline"
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("ожидали text/html при view=inline, получили %q", ct)
	}
	if !strings.Contains(w.Body.String(), "Демонстрационный отчёт") {
		t.Fatalf("тело не содержит демо-отчёт")
	}
}
func TestReportFileDemoServesPDF(t *testing.T) {
	h, _ := newHandler(t)
	h.pdfConverter = fakePDF{bytes: []byte("%PDF-1.4 demo"), err: nil}
	u := "/api/reports/file?demo=1&type=bioscan&id=10"
	req := httptest.NewRequest(http.MethodGet, u, nil)
	w := httptest.NewRecorder()
	h.ReportFile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для демо PDF, получили %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("ожидали application/pdf, получили %q", ct)
	}
	if !strings.Contains(w.Body.String(), "%PDF") {
		t.Fatalf("тело не содержит PDF")
	}
}

// TestDeleteEntry удаляет запись истории владельцем и проверяет, что
// данные реально исчезают из /api/reports, а чужой пользователь получает
// 403, а запрос без initData - 401.
func TestDeleteEntry(t *testing.T) {
	h, uid := newHandler(t)
	h.pay.ActivatePremiumManually(uid, "premium_monthly")
	id := seedReport(t, h, uid, "analysis", "<html>report</html>")
	initData := buildInitData(testBotToken, uid)

	// Отчёт присутствует до удаления.
	before := httptest.NewRequest(http.MethodGet, "/api/reports?initData="+url.QueryEscape(initData), nil)
	wb := httptest.NewRecorder()
	h.Reports(wb, before)
	var beforeResp ReportsResponse
	if err := json.Unmarshal(wb.Body.Bytes(), &beforeResp); err != nil {
		t.Fatalf("декод Reports: %v", err)
	}
	if beforeResp.Analysis.Count != 1 {
		t.Fatalf("ожидали 1 отчёт до удаления, получили %d", beforeResp.Analysis.Count)
	}

	// Удаляем свою запись - 200.
	del := httptest.NewRequest(http.MethodDelete,
		"/api/reports/delete?id="+strconv.FormatInt(id, 10)+"&initData="+url.QueryEscape(initData), nil)
	wd := httptest.NewRecorder()
	h.DeleteEntry(wd, del)
	if wd.Code != http.StatusOK {
		t.Fatalf("ожидали 200 на удаление своей записи, получили %d: %s", wd.Code, wd.Body.String())
	}

	// После удаления отчёта в группе analysis нет.
	after := httptest.NewRequest(http.MethodGet, "/api/reports?initData="+url.QueryEscape(initData), nil)
	wa := httptest.NewRecorder()
	h.Reports(wa, after)
	var afterResp ReportsResponse
	if err := json.Unmarshal(wa.Body.Bytes(), &afterResp); err != nil {
		t.Fatalf("декод Reports: %v", err)
	}
	if afterResp.Analysis.Count != 0 {
		t.Errorf("ожидали 0 отчётов после удаления, получили %d", afterResp.Analysis.Count)
	}

	// Повторное удаление той же записи - 404 (уже удалена).
	del2 := httptest.NewRequest(http.MethodDelete,
		"/api/reports/delete?id="+strconv.FormatInt(id, 10)+"&initData="+url.QueryEscape(initData), nil)
	wd2 := httptest.NewRecorder()
	h.DeleteEntry(wd2, del2)
	if wd2.Code != http.StatusNotFound {
		t.Errorf("ожидали 404 на повторное удаление, получили %d", wd2.Code)
	}

	// Чужой пользователь (другой initData) не может удалить - 403.
	otherData := buildInitData(testBotToken, 12345)
	delOther := httptest.NewRequest(http.MethodDelete,
		"/api/reports/delete?id="+strconv.FormatInt(id, 10)+"&initData="+url.QueryEscape(otherData), nil)
	wd3 := httptest.NewRecorder()
	h.DeleteEntry(wd3, delOther)
	if wd3.Code != http.StatusForbidden && wd3.Code != http.StatusNotFound {
		t.Errorf("ожидали 403/404 для чужой записи, получили %d", wd3.Code)
	}

	// Без initData - 401.
	delNoAuth := httptest.NewRequest(http.MethodDelete, "/api/reports/delete?id=1", nil)
	wd4 := httptest.NewRecorder()
	h.DeleteEntry(wd4, delNoAuth)
	if wd4.Code != http.StatusUnauthorized {
		t.Errorf("ожидали 401 без initData, получили %d", wd4.Code)
	}
}

// TestServeWebAppAssetVersioning — ServeWebApp должен отдавать актуальные
// встроенные файлы по ВЕРСИОНИРОВАННОМУ пути (app.<ver>.js / style.<ver>.css)
// и по неверсионированному (обратная совместимость), а несуществующий актив
// — 404. Это ключевая логика сброса кэша Telegram WebView по пути файла.
func TestServeWebAppAssetVersioning(t *testing.T) {
	h, _ := newHandler(t)

	cases := []struct {
		path        string
		wantStatus  int
		wantCT      string
		wantBodyHit string
	}{
		{"/dashboard/app.v31.js", http.StatusOK, "application/javascript; charset=utf-8", "Prisma"},
		{"/dashboard/style.v31.css", http.StatusOK, "text/css; charset=utf-8", ""},
		{"/dashboard/app.js", http.StatusOK, "application/javascript; charset=utf-8", "Prisma"},
		{"/dashboard/style.css", http.StatusOK, "text/css; charset=utf-8", ""},
		{"/dashboard/app.v99.js", http.StatusOK, "application/javascript; charset=utf-8", "Prisma"},
		{"/dashboard/app.js?v=v31", http.StatusOK, "application/javascript; charset=utf-8", "Prisma"},
		{"/dashboard/missing.xyz", http.StatusNotFound, "", ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		w := httptest.NewRecorder()
		h.ServeWebApp(w, req)
		if w.Code != c.wantStatus {
			t.Errorf("path=%q: статус %d, хотели %d (body=%q)", c.path, w.Code, c.wantStatus, w.Body.String()[:min(80, w.Body.Len())])
			continue
		}
		if c.wantCT != "" {
			ct := w.Header().Get("Content-Type")
			if ct != c.wantCT {
				t.Errorf("path=%q: Content-Type=%q, хотели %q", c.path, ct, c.wantCT)
			}
			// no-store должен быть, чтобы Telegram не кэшировал HTML/ассеты.
			if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
				t.Errorf("path=%q: Cache-Control=%q, ожидали no-store", c.path, cc)
			}
		}
		if c.wantBodyHit != "" && !strings.Contains(w.Body.String(), c.wantBodyHit) {
			t.Errorf("path=%q: тело не содержит %q", c.path, c.wantBodyHit)
		}
	}
}

// TestReportFileDemoRespectsID — демо-отчёт (?demo=1) отражает выбранную
// запись по id: для разных id отдаются разные даты (а не «последний»
// отчёт). Регрессия на баг «клик по любому архиву показывает последний».
func TestReportFileDemoRespectsID(t *testing.T) {
	h, _ := newHandler(t)
	h.pdfConverter = fakePDF{err: fmt.Errorf("no key")} // откат к HTML

	cases := []struct {
		kind     string
		id       int64
		wantDate string
	}{
		{"analysis", 1, time.Now().Format("2006-01-02")},
		{"analysis", 3, time.Now().AddDate(0, -3, 0).Format("2006-01-02")},
		{"bioscan", 10, time.Now().Format("2006-01-02")},
		{"bioscan", 12, time.Now().AddDate(0, -4, 0).Format("2006-01-02")},
	}
	for _, c := range cases {
		u := fmt.Sprintf("/api/reports/file?demo=1&type=%s&id=%d", c.kind, c.id)
		req := httptest.NewRequest(http.MethodGet, u, nil)
		w := httptest.NewRecorder()
		h.ReportFile(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("kind=%s id=%d: ожидали 200, получили %d", c.kind, c.id, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, c.wantDate) {
			n := len(body)
			if n > 300 {
				n = 300
			}
			t.Errorf("kind=%s id=%d: ожидали дату %q в отчёте, получили: %q", c.kind, c.id, c.wantDate, body[:n])
		}
	}
}
