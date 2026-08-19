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
	mu         sync.Mutex
	nextID     int64
	lastSentID int64
	deleted    map[int64]bool
	sent       []string
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
	case "answerCallbackQuery", "editMessageText", "getMe":
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
	pay := payment.NewMockPaymentService(nil)

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
