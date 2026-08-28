package userdata

import (
	"testing"

	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// TestProgressHeaderRelativeWithSkip проверяет, что при подстановке профиля
// прогресс-бар считается относительно РЕАЛЬНО оставшихся вопросов, а не полного
// списка: демография (имя/пол/возраст/рост+вес) пропущена, опросник
// «Общая оценка здоровья» начинается с цели -> «Вопрос 1 из 3», а не «5 из 7».
func TestProgressHeaderRelativeWithSkip(t *testing.T) {
	sm := states.NewMemoryStateManager("")
	c := NewUserDataCollector(sm)
	chatID := int64(12345)

	// Имитируем подстановку профиля: пропущено 4 ведущих вопроса.
	sm.SetUserData(chatID, HealthSkipKey, "4")

	cases := []struct {
		state states.State
		want  string
	}{
		{states.StateWaitingGoal, "📋 Вопрос 1 из 3\n\n"},
		{states.StateWaitingLifestyle, "📋 Вопрос 2 из 3\n\n"},
		{states.StateWaitingHabits, "📋 Вопрос 3 из 3\n\n"},
	}
	for _, tc := range cases {
		got := c.progressHeader(chatID, tc.state)
		if got != tc.want {
			t.Errorf("progressHeader(%s) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

// TestProgressHeaderAbsoluteNoSkip проверяет классический (абсолютный) счётчик
// при чистом старте опросника без подстановки профиля: «Вопрос N из 7».
func TestProgressHeaderAbsoluteNoSkip(t *testing.T) {
	sm := states.NewMemoryStateManager("")
	c := NewUserDataCollector(sm)
	chatID := int64(12345)

	cases := []struct {
		state states.State
		want  string
	}{
		{states.StateWaitingName, "📋 Вопрос 1 из 7\n\n"},
		{states.StateWaitingGender, "📋 Вопрос 2 из 7\n\n"},
		{states.StateWaitingHabits, "📋 Вопрос 7 из 7\n\n"},
	}
	for _, tc := range cases {
		got := c.progressHeader(chatID, tc.state)
		if got != tc.want {
			t.Errorf("progressHeader(%s) = %q, want %q", tc.state, got, tc.want)
		}
	}
}

// TestSkippedSteps проверяет чтение смещения прогресса из user-data.
func TestSkippedSteps(t *testing.T) {
	sm := states.NewMemoryStateManager("")
	chatID := int64(99)

	if got := SkippedSteps(sm, chatID); got != 0 {
		t.Errorf("SkippedSteps (unset) = %d, want 0", got)
	}
	sm.SetUserData(chatID, HealthSkipKey, "4")
	if got := SkippedSteps(sm, chatID); got != 4 {
		t.Errorf("SkippedSteps (4) = %d, want 4", got)
	}
	sm.SetUserData(chatID, HealthSkipKey, "не число")
	if got := SkippedSteps(sm, chatID); got != 0 {
		t.Errorf("SkippedSteps (invalid) = %d, want 0", got)
	}
}
