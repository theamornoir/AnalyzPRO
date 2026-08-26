package httpratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Limiter - простой in-memory rate-limiter по IP для HTTP-эндпоинтов
// дашборда и вебхука YooKassa. Защищает сервер от brute-force / спама
// (например, массовых вызовов /api/reports/file или подбора initData),
// которые иначе вели бы к бессмысленной нагрузке на БД/AI и трате квоты.
//
// Не предназначен для кластера (состояние в памяти процесса). Для
// горизонтального масштабирования понадобился бы внешний счётчик (Redis).
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	max     int
	window  time.Duration
}

type entry struct {
	count    int
	firstHit time.Time
}

// New создаёт лимитер: не более max запросов в окне window с одного IP.
func New(max int, window time.Duration) *Limiter {
	if max <= 0 {
		max = 120
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{
		entries: make(map[string]*entry),
		max:     max,
		window:  window,
	}
}

// allow возвращает true, если запрос с IP укладывается в лимит.
func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[ip]
	if !ok || now.Sub(e.firstHit) > l.window {
		l.entries[ip] = &entry{count: 1, firstHit: now}
		return true
	}
	e.count++
	return e.count <= l.max
}

// Cleanup удаляет устаревшие записи (вызывать периодически из горутины).
func (l *Limiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for ip, e := range l.entries {
		if now.Sub(e.firstHit) > l.window {
			delete(l.entries, ip)
		}
	}
}

// Middleware оборачивает http.Handler дросселированием по IP (без порта из
// RemoteAddr). При превышении лимита отдаёт 429 без тела.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("too many requests"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP извлекает IP из RemoteAddr (убирая порт). Если разбор не
// получается (unix-socket и т.п.) - возвращает строку как есть.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
