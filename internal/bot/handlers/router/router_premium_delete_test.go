package router

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// mockTelegramClient - поддельный HttpClient библиотеки go-telegram/bot.
// Запоминает все вызовы sendMessage (возвращает растущий message_id) и
// deleteMessage (запоминает удалённые message_id), чтобы тест мог
// проверить, что экран Premium действительно удаляется при выходе.
type mockTelegramClient struct {
	mu      sync.Mutex
	nextID  int
	sent    []int
	deleted []int
	chatID  int64
}

func (m *mockTelegramClient) Do(req *http.Request) (*http.Response, error) {
	// tgbot шлёт multipart/form-data; читаем поля формы (не сырой JSON).
	_ = req.ParseMultipartForm(1 << 20)

	method := ""
	if u, err := url.Parse(req.URL.String()); err == nil {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			method = parts[len(parts)-1]
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch method {
	case "sendMessage":
		m.nextID++
		id := m.nextID
		m.sent = append(m.sent, id)
		payload := `{"ok":true,"result":{"message_id":` + strconv.Itoa(id) + `,"chat":{"id":` + strconv.FormatInt(m.chatID, 10) + `},"text":""}}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	case "deleteMessage":
		mid, _ := strconv.Atoi(req.FormValue("message_id"))
		m.deleted = append(m.deleted, mid)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
		}, nil
	default:
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"id":1,"is_bot":true}}`)),
		}, nil
	}
}

func newMockBot(t *testing.T, chatID int64) (*tgbot.Bot, *mockTelegramClient) {
	t.Helper()
	mc := &mockTelegramClient{chatID: chatID}
	b, err := tgbot.New("test-token", tgbot.WithHTTPClient(0, mc), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать mock-бота: %v", err)
	}
	return b, mc
}

// TestBackToParentDeletesPremiumScreen - проверяет, что при выходе из
// раздела Premium (кнопка «Назад») бот удаляет И якорь, И список тарифов,
// и не оставляет экран Premium в истории чата.
func TestBackToParentDeletesPremiumScreen(t *testing.T) {
	chatID := int64(111222333)
	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	b, mc := newMockBot(t, chatID)

	r := &router{
		stateManager:     sm,
		agreementStorage: agr,
		paymentService:   payment.NewMockPaymentService(""),
	}

	// Имитируем вход в Premium: переключаем раздел и шлём экран (якорь +
	// список тарифов) через реальный обработчик.
	r.setCurrentSection(chatID, "premium")
	update := &models.Update{Message: &models.Message{Chat: models.Chat{ID: chatID}}}
	menu.PremiumHandler(r.stateManager, r.paymentService)(context.Background(), b, update)

	anchorID, _ := strconv.Atoi(sm.GetPremiumScreenID(chatID, "premium_anchor_id"))
	msgID, _ := strconv.Atoi(sm.GetPremiumScreenID(chatID, "premium_msg_id"))
	if anchorID == 0 || msgID == 0 {
		t.Fatalf("не удалось настроить экран Premium: anchor=%d msg=%d", anchorID, msgID)
	}
	if r.currentSection(chatID) != "premium" {
		t.Fatalf("currentSection не premium: %q", r.currentSection(chatID))
	}

	mcBeforeSent := len(mc.sent)
	mcBeforeDeleted := len(mc.deleted)

	// Нажимаем «Назад» из Premium.
	r.backToParent(context.Background(), b, chatID)

	// Экран Premium должен быть полностью удалён (якорь + список тарифов).
	deletedSet := map[int]bool{}
	for _, id := range mc.deleted[mcBeforeDeleted:] {
		deletedSet[id] = true
	}
	if !deletedSet[anchorID] {
		t.Errorf("якорь Premium (msgID=%d) не удалён при выходе", anchorID)
	}
	if !deletedSet[msgID] {
		t.Errorf("список тарифов Premium (msgID=%d) не удалён при выходе", msgID)
	}
	if sm.GetPremiumScreenID(chatID, "premium_anchor_id") != "" || sm.GetPremiumScreenID(chatID, "premium_msg_id") != "" {
		t.Errorf("ключи premium_anchor_id/premium_msg_id не сброшены")
	}
	// При выходе из Premium не должно появляться «висячих» сообщений помимо
	// одного главного меню.
	if len(mc.sent) != mcBeforeSent+1 {
		t.Errorf("ожидалось ровно 1 сообщение (главное меню) после выхода, отправлено %d", len(mc.sent)-mcBeforeSent)
	}
}
