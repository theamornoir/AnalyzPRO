package payment

import (
	"context"
	"testing"

	sm "github.com/theamornoir/analyzpro/internal/storage/models"
	"github.com/theamornoir/analyzpro/internal/storage/mock"
)

// TestPremiumPersistedToDB проверяет P1-фикс: состояние Premium теперь
// дублируется в БД (источник истины) при активации/сбросе и
// восстанавливается из БД после перезапуска бота (кэш пуст).
func TestPremiumPersistedToDB(t *testing.T) {
	repo := mock.NewMockUserRepository()
	// Пользователь создан (как при /start).
	if err := repo.CreateUser(context.Background(), &sm.User{TelegramID: 777, Name: "Payer"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	chatID := int64(777)

	pay := NewPaymentService(repo, YooKassaConfig{})

	if pay.IsPremium(chatID) {
		t.Fatal("до активации не должно быть Premium")
	}

	if err := pay.ActivatePremiumManually(chatID, "premium_yearly"); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Кэш внутри сервиса отражает активацию.
	if !pay.IsPremium(chatID) {
		t.Fatal("после активации IsPremium должен быть true (кэш)")
	}

	// И БД обновлена (источник истины).
	stored, err := repo.GetUserByTelegramID(context.Background(), chatID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !stored.IsPremium {
		t.Error("БД: is_premium не выставлен")
	}
	if stored.TariffID != "premium_yearly" {
		t.Errorf("БД: tariff_id=%q, want premium_yearly", stored.TariffID)
	}
	if stored.PremiumExpiresAt.IsZero() {
		t.Error("БД: premium_expires_at пуст")
	}

	// Новый экземпляр сервиса того же репозитория (имитация перезапуска
	// бота): Premium должен восстановиться из БД (кэш пуст).
	pay2 := NewPaymentService(repo, YooKassaConfig{})
	if !pay2.IsPremium(chatID) {
		t.Error("после перезапуска Premium не восстановлен из БД")
	}
	info := pay2.GetPremiumInfo(chatID)
	if info == nil || info.TariffID != "premium_yearly" {
		t.Errorf("после перезапуска tariff_id не восстановлен: %+v", info)
	}

	// Сброс -> БД тоже сбрасывается.
	pay.ResetPremium(chatID)
	stored2, _ := repo.GetUserByTelegramID(context.Background(), chatID)
	if stored2.IsPremium {
		t.Error("после сброса БД: is_premium всё ещё true")
	}
	if stored2.TariffID != "" {
		t.Errorf("после сброса БД: tariff_id=%q, want empty", stored2.TariffID)
	}
	if pay.IsPremium(chatID) {
		t.Error("после сброса IsPremium должен быть false")
	}
}

// TestPremiumNilRepoInMemory проверяет, что при nil-репозитории сервис
// работает только в in-memory кэше (сценарий тестов/fallback) и не падает.
func TestPremiumNilRepoInMemory(t *testing.T) {
	pay := NewPaymentService(nil, YooKassaConfig{})
	chatID := int64(999)

	if pay.IsPremium(chatID) {
		t.Fatal("nil-репо: до активации не должно быть Premium")
	}
	if err := pay.ActivatePremiumManually(chatID, "premium_monthly"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !pay.IsPremium(chatID) {
		t.Fatal("nil-репо: после активации IsPremium должен быть true")
	}
	info := pay.GetPremiumInfo(chatID)
	if info == nil || info.TariffID != "premium_monthly" {
		t.Errorf("nil-репо: tariff_id не сохранён в кэше: %+v", info)
	}
	pay.ResetPremium(chatID)
	if pay.IsPremium(chatID) {
		t.Error("nil-репо: после сброса IsPremium должен быть false")
	}
}
