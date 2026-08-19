package ratelimit

import (
	"sync"
	"time"
)

// Limiter - простой in-memory rate-limiter (скользящее окно) на основе
// счётчика событий по chatID. Защищает бота от спама одного пользователя
// (многократные фото/документы/сообщения), который иначе породил бы
// неограниченное число горутин и очередь ИИ-запросов.
//
// Не предназначен для распределённых систем (несколько инстансов бота) -
// состояние хранится в памяти процесса. Для горизонтального масштабирования
// потребовался бы внешний счётчик (Redis), но для одного инстанса достаточно.
type Limiter struct {
	mu       sync.Mutex
	entries  map[int64]*entry
	maxCount int
	window   time.Duration
}

type entry struct {
	count    int
	firstHit time.Time
	warned   bool
}

// New создаёт лимитер: разрешено не более maxCount событий в пределах
// одного окна window на каждый chatID.
func New(maxCount int, window time.Duration) *Limiter {
	if maxCount <= 0 {
		maxCount = 20
	}
	if window <= 0 {
		window = 10 * time.Second
	}
	return &Limiter{
		entries:  make(map[int64]*entry),
		maxCount: maxCount,
		window:   window,
	}
}

// Allow возвращает true, если событие для chatID укладывается в лимит.
// Счётчик скользящий: при выходе за пределы window начинается новый отсчёт.
func (l *Limiter) Allow(chatID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.entries[chatID]
	if !ok || now.Sub(e.firstHit) > l.window {
		l.entries[chatID] = &entry{count: 1, firstHit: now}
		return true
	}
	e.count++
	return e.count <= l.maxCount
}

// ShouldWarn сообщает, нужно ли отправить пользователю уведомление «сбавьте
// темп» - ровно один раз за окно (чтобы не спамить при каждом лишнем
// событии).
func (l *Limiter) ShouldWarn(chatID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[chatID]; ok && !e.warned {
		e.warned = true
		return true
	}
	return false
}

// Cleanup удаляет устаревшие записи (вызывать периодически из горутины).
func (l *Limiter) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for id, e := range l.entries {
		if now.Sub(e.firstHit) > l.window {
			delete(l.entries, id)
		}
	}
}
