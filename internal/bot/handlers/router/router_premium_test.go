package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// mockTelegram - минимальный мок Telegram Bot API: отвечает success на
// sendMessage (возвращая инкрементный message_id), запоминает все удалённые
// message_id и последний отправленный message_id. Позволяет детерминированно
// проверить, что при выходе из Premium бот действительно удаляет экран
// тарифов (а не оставляет его «висеть»).
type mockTelegram struct {
	mu             sync.Mutex
	nextID         int64
	lastSentID     int64
	deleted        map[int64]bool
	sent           []string
	lastEditMarkup string
	lastSendMarkup string
}

func newMockTelegram() *mockTelegram {
	return &mockTelegram{nextID: 100, deleted: make(map[int64]bool)}
}

func (m *mockTelegram) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		method := r.FormValue("__method")
		if method == "" {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			method = parts[len(parts)-1]
		}
		m.serve(w, r, method)
	})
}

func (m *mockTelegram) serve(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case "sendMessage":
		id := atomic.AddInt64(&m.nextID, 1)
		text := r.FormValue("text")
		m.mu.Lock()
		m.lastSentID = id
		m.lastSendMarkup = r.FormValue("reply_markup")
		m.sent = append(m.sent, text)
		m.mu.Unlock()
		writeJSON(w, map[string]any{
			"ok":     true,
			"result": map[string]any{"message_id": id, "chat": map[string]any{"id": 1}, "text": text},
		})
	case "deleteMessage":
		mid, _ := strconv.ParseInt(r.FormValue("message_id"), 10, 64)
		m.mu.Lock()
		m.deleted[mid] = true
		m.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "result": true})
	case "answerCallbackQuery", "getMe":
		writeJSON(w, map[string]any{"ok": true, "result": true})
	case "editMessageText":
		markup := r.FormValue("reply_markup")
		m.mu.Lock()
		m.lastEditMarkup = markup
		m.mu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "result": true})
	default:
		writeJSON(w, map[string]any{"ok": true, "result": true})
	}
}

func (m *mockTelegram) lastID() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastSentID
}

func (m *mockTelegram) lastSendMarkupStr() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastSendMarkup
}

func (m *mockTelegram) isDeleted(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleted[id]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestPremiumScreenCleanedOnBack(t *testing.T) {
	chatID := int64(555001)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm,
		nil,      // analysisService (в premium-флоу не нужен)
		nil,      // reportRenderer
		nil,      // pdfConverter
		"",       // uploadDir
		"",       // stickerID
		int64(0), // adminChatID
		agr,
		pay,
		nil, // appStorage (TouchActivity защищён nil-проверкой)
		nil, // monitorRepo
		nil, // notificationsSvc (в premium-флоу не нужен)
		"https://app.example/dashboard",
		"https://app.example/dashboard",
		"development",
	)

	sleep := func() { time.Sleep(700 * time.Millisecond) }

	pressReply := func(text string) {
		handle(context.Background(), b, &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: chatID},
				Text: text,
				ID:   1,
			},
		})
		sleep()
	}

	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   "cb_" + data,
				From: models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}},
				},
				Data: data,
			},
		})
		sleep()
	}

	tariffID := payment.AvailableTariffs[0].ID

	pressReply(locales.BtnPremium)
	listAfterStep1 := sm.GetPremiumScreenID(chatID, "premium_msg_id")
	if listAfterStep1 == "" || listAfterStep1 == "0" {
		t.Fatalf("шаг 1: premium_msg_id не установлен в список тарифов (=%q)", listAfterStep1)
	}

	pressCallback("premium_" + tariffID)

	pressCallback("premium_confirm_" + tariffID)
	if !pay.IsUserPremium(chatID) {
		t.Fatalf("шаг 3: Premium не активирован")
	}

	pressReply(locales.BtnPremium)

	pressCallback("premium_change")
	listAfterChange := sm.GetPremiumScreenID(chatID, "premium_msg_id")
	listAfterChangeID, _ := strconv.ParseInt(listAfterChange, 10, 64)
	if listAfterChangeID <= 0 {
		t.Fatalf("шаг 5: после смены тарифа premium_msg_id не указывает на новый список (=%q)", listAfterChange)
	}

	pressReply(locales.BtnBack)

	if !mt.isDeleted(listAfterChangeID) {
		t.Errorf("БАГ ВОСПРОИЗВЕДЁН: список тарифов (message_id=%d) НЕ удалён при выходе из Premium", listAfterChangeID)
	}

	if got := sm.GetPremiumScreenID(chatID, "premium_msg_id"); got != "" {
		t.Errorf("premium_msg_id не сброшен после выхода из Premium: %q", got)
	}

	if got := sm.GetPremiumScreenID(chatID, "premium_anchor_id"); got != "" {
		t.Errorf("premium_anchor_id не сброшен после выхода из Premium: %q", got)
	}
}

// TestPremiumOpenFromInlineMenu проверяет, что кнопка «💎 Premium» из
// inline-главного меню (callback premium_open) действительно открывает экран
// Premium (список тарифов), а не является «мёртвой» (регрессия рефакторинга
// навигации: раньше premium_open не имел обработчика, и Premium из inline-
// меню был недоступен, а PremiumHandler паниковал на nil update.Message).
func TestPremiumOpenFromInlineMenu(t *testing.T) {
	chatID := int64(555002)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm,
		nil, nil, nil, "", "", int64(0), agr, pay, nil, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)

	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}
	pressBack := func() {
		handle(context.Background(), b, &models.Update{
			Message: &models.Message{Chat: models.Chat{ID: chatID}, Text: locales.BtnBack, ID: 1},
		})
		sleep()
	}

	// Кнопка «💎 Premium» из inline-главного меню должна открыть экран
	// Premium (список тарифов), а не быть «мёртвой».
	pressCallback("premium_open")

	msgID := sm.GetPremiumScreenID(chatID, "premium_msg_id")
	if msgID == "" || msgID == "0" {
		t.Fatalf("premium_open не открыл экран Premium (premium_msg_id=%q)", msgID)
	}

	// «Назад» из Premium должен вернуть в Главное меню и очистить экран.
	pressBack()
	if got := sm.GetPremiumScreenID(chatID, "premium_msg_id"); got != "" {
		t.Errorf("premium_msg_id не сброшен после выхода из Premium: %q", got)
	}
}

// TestBackFromSubActionReturnsToHub проверяет, что «Назад» с экрана
// под-действия (премиум-заглушки расширенного анализа) возвращает именно в
// хаб раздела «Анализы» (nav_level=hub, current_section=analysis), а не
// сразу в Главное меню. Регрессия рефакторинга навигации: раньше BackInline
// ошибочно использовал callback back_to_main и прыгал в Главное меню, тогда
// как единообразный с BackCancelInline «Назад» должен вести в хаб раздела.
func TestBackFromSubActionReturnsToHub(t *testing.T) {
	chatID := int64(555003)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	// Не-Premium: расширенный анализ покажет заглушку с кнопкой BackInline.
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm, nil, nil, nil, "", "", int64(0), agr, pay, nil, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)

	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressReply := func(text string) {
		handle(context.Background(), b, &models.Update{
			Message: &models.Message{Chat: models.Chat{ID: chatID}, Text: text, ID: 1},
		})
		sleep()
	}
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	// Входим в хаб «Анализы».
	pressReply(locales.BtnAnalysisHub)
	if got := sm.GetUserData(chatID, "current_section"); got != "analysis" {
		t.Fatalf("не вошли в хаб Анализы (current_section=%q)", got)
	}
	if got := sm.GetUserData(chatID, "nav_level"); got != "hub" {
		t.Fatalf("хаб не на уровне hub (nav_level=%q)", got)
	}

	// «Расширенный анализ» для не-Premium - экран-заглушка с кнопкой
	// «Назад» (BackInline). Кнопка должна нести callback hub_back, а не
	// back_to_main (регрессия: раньше прыгала в Главное меню).
	pressCallback("section_diag_extended")
	if got := sm.GetUserData(chatID, "nav_level"); got != "action" {
		t.Fatalf("экран под-действия не на уровне action (nav_level=%q)", got)
	}
	mt.mu.Lock()
	markup := mt.lastEditMarkup
	mt.mu.Unlock()
	if !strings.Contains(markup, "hub_back") {
		t.Fatalf("БАГ: кнопка «Назад» заглушки не содержит hub_back (markup=%s)", markup)
	}
	if strings.Contains(markup, "back_to_main") {
		t.Fatalf("БАГ ВОСПРОИЗВЕДЁН: кнопка «Назад» заглушки содержит back_to_main (прыжок в Главное меню), markup=%s", markup)
	}

	// «Назад» из заглушки должен вернуть в хаб «Анализы», а не в Главное меню.
	pressCallback("hub_back")
	if got := sm.GetUserData(chatID, "nav_level"); got != "hub" {
		t.Errorf("БАГ ВОСПРОИЗВЕДЁН: «Назад» из под-действия не вернул в хаб (nav_level=%q), ожидался hub", got)
	}
	if got := sm.GetUserData(chatID, "current_section"); got != "analysis" {
		t.Errorf("«Назад» из под-действия сменил раздел (current_section=%q), ожидался analysis", got)
	}
}

// TestBioscanExtendedGateReturnsToHub проверяет, что премиум-заглушка
// Bioscan PRO (не-Premium пользователь нажимает «✨ Bioscan PRO» в хабе
// «Анализы») перерисовывается «на месте» (edit-in-place), а не шлёт
// отдельное сообщение с reply-клавиатурой BackMenu(). Кнопка «Назад» должна
// нести callback hub_back и возвращать именно в хаб «Анализы»
// (nav_level=hub, current_section=analysis). Регрессия рефакторинга: раньше
// гейт использовал reply BackMenu() - отдельный режим навигации, не
// совпадающий с прочими экранами-заглушками.
func TestBioscanExtendedGateReturnsToHub(t *testing.T) {
	chatID := int64(555004)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	// Не-Premium: Bioscan PRO покажет заглушку с кнопкой BackInline.
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm, nil, nil, nil, "", "", int64(0), agr, pay, nil, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)

	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressReply := func(text string) {
		handle(context.Background(), b, &models.Update{
			Message: &models.Message{Chat: models.Chat{ID: chatID}, Text: text, ID: 1},
		})
		sleep()
	}
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	// Входим в хаб «Анализы».
	pressReply(locales.BtnAnalysisHub)
	if got := sm.GetUserData(chatID, "current_section"); got != "analysis" {
		t.Fatalf("не вошли в хаб Анализы (current_section=%q)", got)
	}
	if got := sm.GetUserData(chatID, "nav_level"); got != "hub" {
		t.Fatalf("хаб не на уровне hub (nav_level=%q)", got)
	}

	// «✨ Bioscan PRO» для не-Premium - экран-заглушка. Должен
	// перерисоваться «на месте» (edit-in-place), «Назад» - callback hub_back.
	pressCallback("section_bioscan_extended")
	if got := sm.GetUserData(chatID, "nav_level"); got != "action" {
		t.Fatalf("экран под-действия не на уровне action (nav_level=%q)", got)
	}
	mt.mu.Lock()
	markup := mt.lastEditMarkup
	mt.mu.Unlock()
	if !strings.Contains(markup, "hub_back") {
		t.Fatalf("БАГ: кнопка «Назад» заглушки Bioscan PRO не содержит hub_back (markup=%s)", markup)
	}
	if strings.Contains(markup, "back_to_main") {
		t.Fatalf("БАГ ВОСПРОИЗВЕДЁН: кнопка «Назад» заглушки содержит back_to_main, markup=%s", markup)
	}

	// «Назад» из заглушки должен вернуть в хаб «Анализы».
	pressCallback("hub_back")
	if got := sm.GetUserData(chatID, "nav_level"); got != "hub" {
		t.Errorf("БАГ ВОСПРОИЗВЕДЁН: «Назад» из под-действия не вернул в хаб (nav_level=%q), ожидался hub", got)
	}
	if got := sm.GetUserData(chatID, "current_section"); got != "analysis" {
		t.Errorf("«Назад» из под-действия сменил раздел (current_section=%q), ожидался analysis", got)
	}
}

// TestOnboardingAcceptShowsInlineMenu проверяет, что после принятия
// пользовательского соглашения в конце онбординга бот показывает ТОЛЬКО
// inline-главное меню (MainMenuInline, edit-in-place), а не reply-
// клавиатуру MainMenu(). Раньше handleOnboarding слал reply MainMenu() - и
// из-за этого после соглашения в чате висело «меню в реплай плюс меню в
// инлайн» (навигация не единообразна). Регрессионный тест: ловит откат к
// reply-клавиатуре.
func TestOnboardingAcceptShowsInlineMenu(t *testing.T) {
	chatID := int64(555005)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm, nil, nil, nil, "", "", int64(0), agr, pay, nil, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)

	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	// Финал онбординга: пользователь нажимает «✅ Принять» (onboarding_accept).
	pressCallback("onboarding_accept")

	// Соглашение зафиксировано.
	if !agr.IsAgreed(chatID) {
		t.Fatalf("соглашение не зафиксировано после onboarding_accept")
	}

	// Главное меню показано как inline: навигационный message_id установлен.
	navID := sm.GetUserData(chatID, "main_menu_msg_id")
	if navID == "" || navID == "0" {
		t.Fatalf("после onboarding_accept navMsgID не установлен (=%q): меню не стало единым навигационным сообщением", navID)
	}

	// Последнее отправленное сообщение (главное меню) - inline, БЕЗ
	// reply-клавиатуры. Inline-меню содержит section_analysis; reply-
	// клавиатура содержит resize_keyboard/keyboard.
	markup := mt.lastSendMarkupStr()
	if !strings.Contains(markup, "section_analysis") {
		t.Fatalf("БАГ: главное меню после онбординга не содержит inline-кнопки (markup=%s)", markup)
	}
	if strings.Contains(markup, "resize_keyboard") || strings.Contains(markup, `"keyboard"`) {
		t.Fatalf("БАГ ВОСПРОИЗВЕДЁН: после онбординга показана reply-клавиатура вместо inline (markup=%s)", markup)
	}
}
