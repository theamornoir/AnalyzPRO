package states

import "sync"

type StateManager interface {
	SetState(chatID int64, state State)
	GetState(chatID int64) State
	Reset(chatID int64)
}

type MemoryStateManager struct {
	mu     sync.RWMutex
	states map[int64]State
}

func NewMemoryStateManager() *MemoryStateManager {
	return &MemoryStateManager{
		states: make(map[int64]State),
	}
}

func (m *MemoryStateManager) SetState(chatID int64, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states[chatID] = state
}

func (m *MemoryStateManager) GetState(chatID int64) State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state, ok := m.states[chatID]; ok {
		return state
	}

	return StateIdle
}

func (m *MemoryStateManager) Reset(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, chatID)
}
