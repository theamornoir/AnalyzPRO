package userdata

import (
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// UserDataCollector - собирает данные о пользователе.
type UserDataCollector struct {
	stateManager states.StateManager
}

// NewUserDataCollector создаёт новый коллектор данных пользователя.
func NewUserDataCollector(stateManager states.StateManager) *UserDataCollector {
	return &UserDataCollector{
		stateManager: stateManager,
	}
}
