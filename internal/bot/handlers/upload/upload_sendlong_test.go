package upload

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	tgbot "github.com/go-telegram/bot"
)

// longChunkCapture - минимальный мок Telegram Bot API: запоминает все
// отправленные sendMessage и их текст. Позволяет детерминированно проверить,
// что sendLongMessagePlain при длинном результате ИИ разбивает текст на
// несколько сообщений (каждое <= 4000), а не шлёт один "огромный" кусок,
// который Telegram отбросил бы с 400.
type longChunkCapture struct {
	mu    sync.Mutex
	texts []string
}

func (m *longChunkCapture) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		method := r.FormValue("__method")
		if method == "" {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			method = parts[len(parts)-1]
		}
		switch method {
		case "sendMessage":
			text := r.FormValue("text")
			m.mu.Lock()
			m.texts = append(m.texts, text)
			m.mu.Unlock()
			writeJSONCapture(w, map[string]any{
				"ok":     true,
				"result": map[string]any{"message_id": 1, "chat": map[string]any{"id": 1}, "text": text},
			})
		default:
			writeJSONCapture(w, map[string]any{"ok": true, "result": true})
		}
	})
}

func writeJSONCapture(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestSendLongMessagePlainChunks(t *testing.T) {
	m := &longChunkCapture{}
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	// Текст длиннее лимита Telegram (4096): 4500 кириллических рун.
	long := strings.Repeat("А", 4500)
	sendLongMessagePlain(context.Background(), b, 1, long)

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.texts) != 2 {
		t.Fatalf("ожидалось 2 сообщения для текста в 4500 рун, получено %d", len(m.texts))
	}
	for i, txt := range m.texts {
		if n := len([]rune(txt)); n > 4000 {
			t.Errorf("кусок %d превышает 4000 рун: %d", i, n)
		}
	}

	joined := strings.Join(m.texts, "")
	if len([]rune(joined)) != 4500 {
		t.Fatalf("склейка сообщений потеряла символы: %d вместо 4500", len([]rune(joined)))
	}
}
