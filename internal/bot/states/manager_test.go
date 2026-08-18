package states

import (
	"path/filepath"
	"testing"
)

// TestPremiumScreenSurvivesReset - ключевой регрессионный тест: id экрана
// Premium НЕ должны теряться при stateManager.Reset (который вызывается в
// /start и /resetme). Раньше id лежали в m.data и стирались Reset'ом - экран
// Premium после /start "висел" в чате навсегда. Теперь они в отдельном map.
func TestPremiumScreenSurvivesReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "states.json")
	sm := NewMemoryStateManager(path)

	chatID := int64(777888)
	sm.SetPremiumScreenID(chatID, "premium_anchor_id", "11")
	sm.SetPremiumScreenID(chatID, "premium_msg_id", "12")

	// Reset (как в /start) не должен стереть id экрана Premium.
	sm.Reset(chatID)

	if got := sm.GetPremiumScreenID(chatID, "premium_anchor_id"); got != "11" {
		t.Fatalf("после Reset premium_anchor_id потерян: %q", got)
	}
	if got := sm.GetPremiumScreenID(chatID, "premium_msg_id"); got != "12" {
		t.Fatalf("после Reset premium_msg_id потерян: %q", got)
	}

	// Очистка должна удалять id.
	sm.ClearPremiumScreenIDs(chatID)
	if got := sm.GetPremiumScreenID(chatID, "premium_anchor_id"); got != "" {
		t.Fatalf("ClearPremiumScreenIDs не очистил premium_anchor_id: %q", got)
	}
}

// TestPremiumScreenPersistedAcrossReload - id экрана Premium должны
// переживать перезапуск бота (синхронная запись в файл), иначе после kill
// старое сообщение Premium в чате остаётся "висящим" навсегда.
func TestPremiumScreenPersistedAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "states.json")

	sm1 := NewMemoryStateManager(path)
	chatID := int64(999111)
	sm1.SetPremiumScreenID(chatID, "premium_anchor_id", "21")
	sm1.SetPremiumScreenID(chatID, "premium_msg_id", "22")

	// "Перезапуск" - новый менеджер читает тот же файл.
	sm2 := NewMemoryStateManager(path)
	if got := sm2.GetPremiumScreenID(chatID, "premium_anchor_id"); got != "21" {
		t.Fatalf("после перезапуска premium_anchor_id не восстановлен: %q", got)
	}
	if got := sm2.GetPremiumScreenID(chatID, "premium_msg_id"); got != "22" {
		t.Fatalf("после перезапуска premium_msg_id не восстановлен: %q", got)
	}
}
