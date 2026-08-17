package analytics

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType - категория аналитического события.
type EventType string

const (
	EventStart    EventType = "start"
	EventAnalysis EventType = "analysis"
	EventBioscan  EventType = "bioscan"
	EventPremium  EventType = "premium"
	EventError    EventType = "error"
)

// Event - одно аналитическое событие (строка JSONL в файле ANALYTICS_PATH).
type Event struct {
	Type       EventType              `json:"type"`
	TelegramID int64                  `json:"telegram_id"`
	UserID     uint                   `json:"user_id,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

var (
	mu   sync.Mutex
	path string
)

// Init - инициализирует хранилище событий (создаёт директорию при необходимости).
// Вызывается один раз при старте приложения (app.New).
func Init(p string) {
	mu.Lock()
	defer mu.Unlock()
	path = p
	if dir := filepath.Dir(p); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	log.Printf("[ANALYTICS] инициализировано: path=%s", p)
}

// EmitEvent - добавляет событие в JSONL-файл (персистентно между перезапусками).
// Потокобезопасно; при отсутствии Init (path=="") - тихо игнорируется.
func EmitEvent(ctx context.Context, e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	mu.Lock()
	defer mu.Unlock()
	if path == "" {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[ANALYTICS] не удалось открыть файл %q: %v", path, err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		log.Printf("[ANALYTICS] не удалось записать событие: %v", err)
	}
}
