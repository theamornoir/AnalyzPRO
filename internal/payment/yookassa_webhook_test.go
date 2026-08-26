package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/storage/mock"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// signBody вычисляет подпись так же, как реальная YooKassa: HMAC-SHA256 от
// тела ключом webhook-секрета, закодированный в Base64 (заголовок
// X-YooKassa-Signature).
func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func webhookBody(t *testing.T, id, userID, tariffID, status string) []byte {
	t.Helper()
	payload := map[string]any{
		"event": "payment.succeeded",
		"object": map[string]any{
			"id":     id,
			"status": status,
			"metadata": map[string]string{
				"user_id":   userID,
				"tariff_id": tariffID,
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestYooKassaWebhookRealMode проверяет реальный путь вебхука YooKassa:
// валидная подпись -> активация Premium (в БД и кэше), дедуп по object.id,
// невалидная подпись -> 400 (защита от бесплатного Premium).
func TestYooKassaWebhookRealMode(t *testing.T) {
	repo := mock.NewMockUserRepository()
	if err := repo.CreateUser(t.Context(), &sm.User{TelegramID: 12345, Name: "Payer"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	chatID := int64(12345)

	svc := NewPaymentService(repo, YooKassaConfig{
		ShopID:    "shop-123",
		SecretKey: "super-secret",
	})

	// 1) Валидный вебхук с корректной подписью.
	body := webhookBody(t, "yk_1", "12345", "premium_yearly", "succeeded")
	req := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-YooKassa-Signature", signBody("super-secret", body))
	rec := httptest.NewRecorder()
	svc.HandleWebhook(rec, req)
	respBody, _ := io.ReadAll(rec.Result().Body)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, respBody)
	}
	if !svc.IsPremium(chatID) {
		t.Fatal("после валидного вебхука Premium должен быть активен")
	}
	stored, err := repo.GetUserByTelegramID(t.Context(), chatID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if !stored.IsPremium || stored.TariffID != "premium_yearly" {
		t.Fatalf("БД не обновлена: is_premium=%v tariff=%q", stored.IsPremium, stored.TariffID)
	}

	// 2) Повторный вебхук с тем же object.id - дедуп, без ошибки (200).
	req2 := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", strings.NewReader(string(body)))
	req2.Header.Set("X-YooKassa-Signature", signBody("super-secret", body))
	rec2 := httptest.NewRecorder()
	svc.HandleWebhook(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("повторный вебхук: ожидали 200, получили %d", rec2.Code)
	}

	// 3) Невалидная подпись -> 400 (нельзя подделать успешный платёж).
	bad := webhookBody(t, "yk_2", "12345", "premium_yearly", "succeeded")
	req3 := httptest.NewRequest(http.MethodPost, "/api/payment/webhook", strings.NewReader(string(bad)))
	req3.Header.Set("X-YooKassa-Signature", "deadbeef")
	rec3 := httptest.NewRecorder()
	svc.HandleWebhook(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при плохой подписи, получили %d", rec3.Code)
	}
}

// TestYooKassaSignatureVerify проверяет verifySignature напрямую.
func TestYooKassaSignatureVerify(t *testing.T) {
	svc := NewPaymentService(nil, YooKassaConfig{SecretKey: "k"})
	body := []byte(`{"event":"payment.succeeded"}`)
	if !svc.verifySignature(body, signBody("k", body)) {
		t.Fatal("валидная подпись должна проходить")
	}
	if svc.verifySignature(body, signBody("wrong", body)) {
		t.Fatal("подпись чужим ключом не должна проходить")
	}
}
