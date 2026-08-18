package notifications

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/locales"
	monitoring_sqlrepo "github.com/theamornoir/analyzpro/internal/monitoring/sqlrepo"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// openTestDB открывает in-memory SQLite (общий кэш, чтобы пул соединений
// видел одну и ту же БД) и применяет миграции.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("не удалось открыть БД: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("не удалось применить миграции: %v", err)
	}
	return conn
}

// insertUser создаёт пользователя напрямую в БД (обходит репозиторий, чтобы
// тест не зависел от его методов).
func insertUser(t *testing.T, conn *sql.DB, telegramID int64, premium bool, expires time.Time) {
	t.Helper()
	var exp interface{}
	if expires.IsZero() {
		exp = nil
	} else {
		exp = expires
	}
	_, err := conn.Exec(
		`INSERT INTO users (telegram_id, name, is_premium, premium_expires_at, onboarding_completed, created_at)
		 VALUES (?, 'Тест', ?, ?, 1, ?)`,
		telegramID, premium, exp, time.Now(),
	)
	if err != nil {
		t.Fatalf("не удалось вставить пользователя: %v", err)
	}
}

// insertAnalysis сохраняет запись анализа с заданным JSON в историю.
func insertAnalysis(t *testing.T, conn *sql.DB, telegramID int64, jsonData string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO monitoring_history (telegram_id, type, title, date, json_data, report_html)
		 VALUES (?, 'analysis', 'Анализ', ?, ?, '')`,
		telegramID, time.Now(), jsonData,
	)
	if err != nil {
		t.Fatalf("не удалось вставить анализ: %v", err)
	}
}

func TestSubscriptionKindAndDaysUntil(t *testing.T) {
	// Фиксированная база (полдень UTC), чтобы daysUntil не зависел от
	// того, пересекает ли now+offset полночь (иначе тест флейковый).
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		offset time.Duration
		want   int
	}{
		{7 * 24 * time.Hour, 7},
		{3 * 24 * time.Hour, 3},
		{1 * 24 * time.Hour, 1},
		{time.Hour, 0},           // сегодня (меньше суток) -> 0
		{-time.Hour, 0},          // уже истекла -> 0
		{8 * 24 * time.Hour, -1}, // не время
		{2 * 24 * time.Hour, -1}, // между порогами -> не время
	}
	for _, c := range cases {
		got := subscriptionKindFor(daysUntil(now.Add(c.offset), now))
		if got != c.want {
			t.Fatalf("offset %v: ожидали kind=%d, получили %d", c.offset, c.want, got)
		}
	}
}

func TestSubscriptionTexts(t *testing.T) {
	if subscriptionText(7) != locales.MsgNotifSub7d {
		t.Fatal("текст за 7 дней не совпадает")
	}
	if subscriptionText(3) != locales.MsgNotifSub3d {
		t.Fatal("текст за 3 дня не совпадает")
	}
	if subscriptionText(1) != locales.MsgNotifSub1d {
		t.Fatal("текст за 1 день не совпадает")
	}
	if subscriptionText(0) != locales.MsgNotifSubToday {
		t.Fatal("текст в день окончания не совпадает")
	}
	if subscriptionText(99) != "" {
		t.Fatal("неизвестный kind должен давать пустой текст")
	}
}

func TestParseIndicatorsAndOutOfRange(t *testing.T) {
	jsonData := `{
		"categories": [
			{"name":"Биохимия","indicators":[
				{"name":"Глюкоза","value":"6.8","normal":"3.3-5.5","status":"critical"},
				{"name":"Гемоглобин","value":"145","normal":"120-160","status":"normal"}
			]}
		]
	}`
	inds := parseIndicators(jsonData)
	if len(inds) != 2 {
		t.Fatalf("ожидали 2 показателя, получили %d", len(inds))
	}
	var glu, hem indicator
	for _, i := range inds {
		switch i.Name {
		case "Глюкоза":
			glu = i
		case "Гемоглобин":
			hem = i
		}
	}
	if !isOutOfRange(glu) {
		t.Fatal("Глюкоза 6.8 при норме 3.3-5.5 должна быть вне нормы")
	}
	if isOutOfRange(hem) {
		t.Fatal("Гемоглобин 145 при норме 120-160 должен быть в норме")
	}
}

// TestParseIndicatorsAllFormats проверяет гибкий парсинг: поддержку
// Формата 2 (indicators[] на верхнем уровне) и Формата 3 (плоский results{}),
// а также поиск референсного интервала по альтернативным именам полей
// (ref_range/reference/norm/range) и извлечение единицы измерения (unit).
func TestParseIndicatorsAllFormats(t *testing.T) {
	// Формат 2: indicators[] на верхнем уровне, референс в ref_range,
	// единица в unit. Значение - число (float64).
	format2 := `{
		"indicators":[
			{"name":"Глюкоза","value":7.2,"ref_range":"3.9-6.1","status":"critical","unit":"ммоль/л"},
			{"name":"Ферритин","value":450,"reference":"30-400","status":"warning"}
		]
	}`
	inds2 := parseIndicators(format2)
	if len(inds2) != 2 {
		t.Fatalf("Формат 2: ожидали 2 показателя, получили %d", len(inds2))
	}
	var glu2 indicator
	for _, i := range inds2 {
		if i.Name == "Глюкоза" {
			glu2 = i
		}
	}
	if glu2.Normal != "3.9-6.1" {
		t.Fatalf("Формат 2: норма не найдена в ref_range, получили %q", glu2.Normal)
	}
	if glu2.Unit != "ммоль/л" {
		t.Fatalf("Формат 2: единица не найдена в unit, получили %q", glu2.Unit)
	}
	if glu2.Value != "7.2" {
		t.Fatalf("Формат 2: значение float64 должно быть 7.2, получили %q", glu2.Value)
	}

	// Формат 3: плоский results{ имя: значение, норма_имя: интервал }.
	// Референсный ключ ищется по точному совпадению "норма_<имя>" и по
	// распространённым формам русского склонения (норма_глюкозы), см.
	// extractFromResults/findNorm.
	format3 := `{
		"results": {
			"глюкоза": 7.2,
			"норма_глюкозы": "3.9-6.1",
			"креатинин": "90",
			"norm_креатинин": "44-106"
		}
	}`
	inds3 := parseIndicators(format3)
	if len(inds3) != 2 {
		t.Fatalf("Формат 3: ожидали 2 показателя (норма_* игнорируются), получили %d", len(inds3))
	}
	byName := map[string]indicator{}
	for _, i := range inds3 {
		byName[i.Name] = i
	}
	g, ok := byName["глюкоза"]
	if !ok {
		t.Fatal("Формат 3: показатель глюкоза не найден")
	}
	if g.Value != "7.2" || g.Normal != "3.9-6.1" {
		t.Fatalf("Формат 3: глюкоза value=%q normal=%q", g.Value, g.Normal)
	}
	c, ok := byName["креатинин"]
	if !ok {
		t.Fatal("Формат 3: показатель креатинин не найден")
	}
	if c.Value != "90" || c.Normal != "44-106" {
		t.Fatalf("Формат 3: креатинин value=%q normal=%q", c.Value, c.Normal)
	}

	// Альтернативные имена референсного интервала (norm/range) в Формате 2.
	alt := `{"indicators":[
		{"name":"А","value":5,"norm":"1-10"},
		{"name":"Б","value":5,"range":"1-10"}
	]}`
	indsAlt := parseIndicators(alt)
	byAlt := map[string]indicator{}
	for _, i := range indsAlt {
		byAlt[i.Name] = i
	}
	if byAlt["А"].Normal != "1-10" || byAlt["Б"].Normal != "1-10" {
		t.Fatalf("альтернативные поля norm/range не распознаны: %+v", byAlt)
	}
}

// TestFormatDeviationWithUnits проверяет, что единица измерения
// добавляется в текст уведомления (после значения и после нормы), а при
// её отсутствии - не добавляется.
func TestFormatDeviationWithUnits(t *testing.T) {
	withUnit := formatDeviationText("Глюкоза", "7.2", "3.9-6.1", "ммоль/л")
	want := "⚠️ Глюкоза: 7.2 ммоль/л при норме 3.9-6.1 ммоль/л. Рекомендуем обновить анализ."
	if withUnit != want {
		t.Fatalf("с единицей:\nожидали %q\nполучили %q", want, withUnit)
	}

	noUnit := formatDeviationText("Глюкоза", "7.2", "3.9-6.1", "")
	wantNo := "⚠️ Глюкоза: 7.2 при норме 3.9-6.1. Рекомендуем обновить анализ."
	if noUnit != wantNo {
		t.Fatalf("без единицы:\nожидали %q\nполучили %q", wantNo, noUnit)
	}
}

func TestParseRangeFormats(t *testing.T) {
	cases := []struct {
		in      string
		low     float64
		high    float64
		badLow  bool // ожидаем low=-inf
		badHigh bool
	}{
		{"3.3-5.5", 3.3, 5.5, false, false},
		{"до 15", 0, 15, true, false},
		{"≤5.0", 0, 5.0, true, false},
		{">=1.0", 1.0, 0, false, true},
		{"<9.0", 0, 9.0, true, false},
	}
	for _, c := range cases {
		low, high, ok := parseRange(c.in)
		if !ok {
			t.Fatalf("parseRange(%q): ok=false", c.in)
		}
		if c.badLow {
			if !math.IsInf(low, -1) {
				t.Fatalf("parseRange(%q): ожидали low=-inf, получили %v", c.in, low)
			}
		} else if low != c.low {
			t.Fatalf("parseRange(%q): low=%v, хотели %v", c.in, low, c.low)
		}
		if c.badHigh {
			if !math.IsInf(high, 1) {
				t.Fatalf("parseRange(%q): ожидали high=+inf, получили %v", c.in, high)
			}
		} else if high != c.high {
			t.Fatalf("parseRange(%q): high=%v, хотели %v", c.in, high, c.high)
		}
	}
}

// runSubscriptionChecks: Premium, окончание через 7 дней -> шлёт ровно одно
// уведомление (за 7 дней) и записывает факт отправки.
func TestRunSubscriptionChecks(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	insertUser(t, conn, 111, true, time.Now().Add(7*24*time.Hour))

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runSubscriptionChecks(ctx)
	if len(sent) != 1 || sent[0] != locales.MsgNotifSub7d {
		t.Fatalf("ожидали 1 уведомление за 7 дней, получили %v", sent)
	}
	has, _ := svc.repo.hasSubscriptionNotification(ctx, 111, 7)
	if !has {
		t.Fatal("факт отправки за 7 дней не записан")
	}

	// Повторный прогон не должен дублировать.
	sent = nil
	svc.runSubscriptionChecks(ctx)
	if len(sent) != 0 {
		t.Fatalf("повторный прогон не должен слать, получили %v", sent)
	}
}

// runSubscriptionChecks: catch-up. Если бот был выключен ровно в день
// окончания за 7 дней и напоминание не ушло (нет записи в БД), то при
// прогоне на 6-й день (daysLeft=6) оно ВСЁ РАВНО уходит, а не теряется.
func TestRunSubscriptionChecksCatchUp(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	// Осталось 6 дней, но записи об отправке за 7 дней нет (бот был down).
	insertUser(t, conn, 112, true, time.Now().Add(6*24*time.Hour))

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runSubscriptionChecks(ctx)
	if len(sent) != 1 || sent[0] != locales.MsgNotifSub7d {
		t.Fatalf("catch-up: ожидали 1 уведомление за 7 дней, получили %v", sent)
	}
	has, _ := svc.repo.hasSubscriptionNotification(ctx, 112, 7)
	if !has {
		t.Fatal("catch-up: факт отправки за 7 дней не записан")
	}
}

// runSubscriptionChecks: если напоминание за 7 дней УЖЕ отправлялось, на
// 6-й день (между порогами) повторно не шлём (нормальный режим, без спама).
func TestRunSubscriptionChecksNoSpamBetween(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	insertUser(t, conn, 113, true, time.Now().Add(6*24*time.Hour))
	// Имитируем уже отправленное напоминание за 7 дней.
	_ = svc.repo.recordSubscriptionNotification(ctx, 113, 7, time.Now())

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runSubscriptionChecks(ctx)
	if len(sent) != 0 {
		t.Fatalf("между порогами не должно быть отправок, получили %v", sent)
	}
}

// runSubscriptionChecks: истёкшая подписка (бот был выключен в день
// окончания) -> шлёт ровно одно напоминание об окончании (kind=0,
// «сегодня/истекла» = MsgNotifSubToday), а НЕ ложное «за 7 дней до
// окончания». Повторный прогон не дублирует.
func TestRunSubscriptionChecksExpired(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	// Подписка истекла 2 дня назад, но 0-е напоминание не уходило (бот был down).
	insertUser(t, conn, 119, true, time.Now().Add(-2*24*time.Hour))

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runSubscriptionChecks(ctx)
	if len(sent) != 1 || sent[0] != locales.MsgNotifSubToday {
		t.Fatalf("истёкшая подписка: ожидали 1 уведомление об окончании (MsgNotifSubToday), получили %v", sent)
	}
	has, _ := svc.repo.hasSubscriptionNotification(ctx, 119, 0)
	if !has {
		t.Fatal("истёкшая подписка: факт отправки (kind=0) не записан")
	}

	// Повторный прогон: 0-е уже отправлено -> не шлём.
	sent = nil
	svc.runSubscriptionChecks(ctx)
	if len(sent) != 0 {
		t.Fatalf("истёкшая подписка: повторный прогон не должен слать, получили %v", sent)
	}
}

// runSubscriptionChecks: не-Premium и далёкая дата -> ничего не шлёт.
func TestRunSubscriptionChecksSkips(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	insertUser(t, conn, 222, false, time.Now().Add(7*24*time.Hour)) // не Premium
	insertUser(t, conn, 223, true, time.Now().Add(30*24*time.Hour)) // далеко

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}
	svc.runSubscriptionChecks(ctx)
	if len(sent) != 0 {
		t.Fatalf("не должно быть отправок, получили %v", sent)
	}
}

// runAnalyticsChecks: Premium с отклонением -> шлёт по показателю и
// подавляет на 14 дней (повтор не шлёт). Free показатели не трогает.
func TestRunAnalyticsChecks(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	const analysisJSON = `{
		"categories": [
			{"name":"Биохимия","indicators":[
				{"name":"Глюкоза","value":"6.8","normal":"3.3-5.5","status":"critical"},
				{"name":"Гемоглобин","value":"145","normal":"120-160","status":"normal"}
			]}
		]
	}`
	insertUser(t, conn, 333, true, time.Now().Add(24*time.Hour))
	insertAnalysis(t, conn, 333, analysisJSON)
	insertUser(t, conn, 334, false, time.Now().Add(24*time.Hour)) // Free
	insertAnalysis(t, conn, 334, analysisJSON)

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runAnalyticsChecks(ctx)
	// Только Premium-пользователь (333), только Глюкоза (вне нормы).
	if len(sent) != 1 || !strings.Contains(sent[0], "Глюкоза") || !strings.Contains(sent[0], "6.8 при норме 3.3-5.5") {
		t.Fatalf("ожидали 1 уведомление по Глюкозе, получили %v", sent)
	}
	suppressed, _ := svc.repo.isSuppressed(ctx, 333, "Глюкоза", time.Now())
	if !suppressed {
		t.Fatal("подавление по Глюкозе не установлено")
	}

	// Повторный прогон: подавление активно -> не шлём.
	sent = nil
	svc.runAnalyticsChecks(ctx)
	if len(sent) != 0 {
		t.Fatalf("повторный прогон не должен слать (14 дней тишины), получили %v", sent)
	}
}

func TestAnalyticsTestAPI(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	const analysisJSON = `{
		"lab_systems": [
			{"indicators":[
				{"name":"Лейкоциты","value":"11.5","normal":"4.0-9.0","status":"warning"},
				{"name":"Гемоглобин","value":"145","normal":"130-160","status":"normal"}
			]}
		]
	}`
	insertUser(t, conn, 444, true, time.Now().Add(24*time.Hour))
	insertAnalysis(t, conn, 444, analysisJSON)

	// Dry-run: без отправки, возвращает найденное отклонение.
	findings, err := svc.RunAnalyticsDryRun(ctx, 444)
	if err != nil {
		t.Fatalf("RunAnalyticsDryRun: %v", err)
	}
	if len(findings) != 1 || findings[0].Name != "Лейкоциты" {
		t.Fatalf("ожидали отклонение Лейкоциты, получили %v", findings)
	}

	// Реальная отправка: 1 уведомление.
	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}
	n, err := svc.SendAnalyticsTest(ctx, 444)
	if err != nil {
		t.Fatalf("SendAnalyticsTest: %v", err)
	}
	if n != 1 || !strings.Contains(sent[0], "Лейкоциты") {
		t.Fatalf("ожидали 1 отправку по Лейкоцитам, получили %v (n=%d)", sent, n)
	}

	// Повторная реальная отправка: подавление активно -> 0.
	n2, _ := svc.SendAnalyticsTest(ctx, 444)
	if n2 != 0 {
		t.Fatalf("повторная отправка должна быть подавлена, получили n=%d", n2)
	}
}

func TestSendSubscriptionTest(t *testing.T) {
	conn := openTestDB(t)
	svc := NewService(conn, nil, nil, nil, true)
	ctx := context.Background()

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}
	for _, kind := range []int{7, 3, 1, 0} {
		text, err := svc.SendSubscriptionTest(ctx, 555, kind)
		if err != nil {
			t.Fatalf("SendSubscriptionTest(%d): %v", kind, err)
		}
		if text == "" {
			t.Fatalf("пустой текст для kind=%d", kind)
		}
	}
	if len(sent) != 4 {
		t.Fatalf("ожидали 4 тестовых уведомления, получили %d", len(sent))
	}
	if _, err := svc.SendSubscriptionTest(ctx, 555, 99); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного kind")
	}
}

// runAnalyticsChecks: несколько отклонений -> ОДНО объединённое сообщение
// (без спама по одному на показатель), оба показателя подавляются.
func TestRunAnalyticsChecksCombines(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	const analysisJSON = `{
		"categories": [
			{"name":"Биохимия","indicators":[
				{"name":"Глюкоза","value":"6.8","normal":"3.3-5.5","status":"critical"},
				{"name":"Лейкоциты","value":"11.5","normal":"4.0-9.0","status":"warning"},
				{"name":"Гемоглобин","value":"145","normal":"120-160","status":"normal"}
			]}
		]
	}`
	insertUser(t, conn, 445, true, time.Now().Add(24*time.Hour))
	insertAnalysis(t, conn, 445, analysisJSON)

	var sent []string
	svc.sendFn = func(_ context.Context, _ int64, text string) bool {
		sent = append(sent, text)
		return true
	}

	svc.runAnalyticsChecks(ctx)
	// Ровно одно сообщение, содержащее оба отклонения.
	if len(sent) != 1 {
		t.Fatalf("ожидали 1 объединённое сообщение, получили %d: %v", len(sent), sent)
	}
	if !strings.Contains(sent[0], "Глюкоза") || !strings.Contains(sent[0], "Лейкоциты") {
		t.Fatalf("объединённое сообщение должно содержать оба показателя: %q", sent[0])
	}
	if suppressed, _ := svc.repo.isSuppressed(ctx, 445, "Глюкоза", time.Now()); !suppressed {
		t.Fatal("подавление по Глюкозе не установлено")
	}
	if suppressed, _ := svc.repo.isSuppressed(ctx, 445, "Лейкоциты", time.Now()); !suppressed {
		t.Fatal("подавление по Лейкоцитам не установлено")
	}
}

// RunAnalyticsDryRun / SendAnalyticsTest: без сохранённых анализов
// возвращают ErrNoAnalysisData (честное отличие от «анализы в норме»).
func TestAnalyticsNoDataError(t *testing.T) {
	conn := openTestDB(t)
	store := storage.NewSQLStorage(conn)
	svc := NewService(conn, store, nil, monitoring_sqlrepo.New(conn), true)
	ctx := context.Background()

	insertUser(t, conn, 446, true, time.Now().Add(24*time.Hour)) // без анализов

	if _, err := svc.RunAnalyticsDryRun(ctx, 446); !errors.Is(err, ErrNoAnalysisData) {
		t.Fatalf("RunAnalyticsDryRun: ожидали ErrNoAnalysisData, получили %v", err)
	}
	n, err := svc.SendAnalyticsTest(ctx, 446)
	if !errors.Is(err, ErrNoAnalysisData) {
		t.Fatalf("SendAnalyticsTest: ожидали ErrNoAnalysisData, получили %v", err)
	}
	if n != 0 {
		t.Fatalf("SendAnalyticsTest: ожидали 0 отправок, получили %d", n)
	}
}
