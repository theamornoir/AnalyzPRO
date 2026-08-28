package bot

import (
	"context"
	"testing"

	"github.com/theamornoir/analyzpro/internal/storage"
)

// newBlockTestBot строит минимальный Bot с мок-хранилищем для проверки
// логики блокировки/кэша без реального Telegram-клиента.
func newBlockTestBot(admin int64) *Bot {
	return &Bot{
		adminChatID:     admin,
		appStorage:      storage.NewMockStorage(),
		blockedUsers:    make(map[int64]bool),
		blockedNotified: make(map[int64]bool),
	}
}

func TestBlockCacheLifecycle(t *testing.T) {
	ctx := context.Background()
	const admin int64 = 1
	const target int64 = 4242
	b := newBlockTestBot(admin)

	if b.isBlocked(target) {
		t.Fatal("изначально не должен быть заблокирован")
	}

	if err := b.blockUser(ctx, target, "спам"); err != nil {
		t.Fatalf("blockUser: %v", err)
	}
	if !b.isBlocked(target) {
		t.Error("после blockUser isBlocked должен быть true")
	}
	if !b.appStorage.Users.IsBlocked(ctx, target) {
		t.Error("блокировка не дошла до хранилища")
	}

	if b.isBlocked(admin) {
		t.Error("админ не должен быть заблокирован")
	}

	if err := b.unblockUser(ctx, target); err != nil {
		t.Fatalf("unblockUser: %v", err)
	}
	if b.isBlocked(target) {
		t.Error("после unblockUser isBlocked должен быть false")
	}
	if b.appStorage.Users.IsBlocked(ctx, target) {
		t.Error("разблокировка не дошла до хранилища")
	}
}

func TestLoadBlockedUsers(t *testing.T) {
	ctx := context.Background()
	const admin int64 = 1
	const target int64 = 777
	b := newBlockTestBot(admin)

	if err := b.appStorage.Users.BlockUser(ctx, target, "тест"); err != nil {
		t.Fatalf("seed block: %v", err)
	}
	b.loadBlockedUsers(ctx)

	if !b.isBlocked(target) {
		t.Error("loadBlockedUsers должен подгрузить заблокированного из хранилища в кэш")
	}
}

func TestAdminNeverBlocked(t *testing.T) {
	const admin int64 = 5
	b := newBlockTestBot(admin)
	if b.isBlocked(admin) {
		t.Error("админ не должен быть заблокированным")
	}
}
