package payment

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// PaymentRequest — запрос на создание платежа.
type PaymentRequest struct {
	UserID   int64  `json:"user_id"`
	TariffID string `json:"tariff_id"`
}

// PaymentResponse — ответ с ссылкой на оплату.
type PaymentResponse struct {
	PaymentID string `json:"payment_id"`
	URL       string `json:"url"`
	Amount    int    `json:"amount"`
}

// YookassaWebhook — входящий вебхук от YooKassa.
type YookassaWebhook struct {
	Type   string      `json:"type"`
	Object WebhookData `json:"object"`
}

// WebhookData — данные события вебхука.
type WebhookData struct {
	ID        string    `json:"id"`
	Status    string    `json:"succeeded"`
	Amount    int64     `json:"amount"`
	UserID    int64     `json:"user_id"` // кастомное поле
	TariffID  string    `json:"tariff_id"`
	CreatedAt time.Time `json:"created_at"`
}

// MockPaymentService — мок-сервис платежей (замена YooKassa).
type MockPaymentService struct {
	mu          sync.RWMutex
	payments    map[string]*MockPayment
	users       map[int64]*PremiumUser
	webhookFunc func(payload []byte) error // callback для обработки вебхуков
	dataFile    string                     // файл для сохранения состояния Premium
}

// PremiumUser — информация о премиум-пользователе.
type PremiumUser struct {
	UserID           int64     `json:"user_id"`
	IsPremium        bool      `json:"is_premium"`
	PremiumSince     time.Time `json:"premium_since"`
	PremiumExpiresAt time.Time `json:"premium_expires_at"`
	TariffID         string    `json:"tariff_id"`
}

// NewMockPaymentService создаёт мок-сервис платежей.
// dataFile — путь к JSON-файлу для сохранения состояния Premium-пользователей.
func NewMockPaymentService(dataFile string) *MockPaymentService {
	s := &MockPaymentService{
		payments: make(map[string]*MockPayment),
		users:    make(map[int64]*PremiumUser),
		dataFile: dataFile,
	}

	// Загружаем сохранённых пользователей из файла
	s.loadUsers()

	return s
}

// loadUsers — загружает пользователей из JSON-файла.
func (s *MockPaymentService) loadUsers() {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		// Файл не существует — это нормально
		return
	}

	if err := json.Unmarshal(data, &s.users); err != nil {
		log.Printf("⚠️ Failed to load premium users: %v", err)
		return
	}

	log.Printf("📂 Loaded %d premium user(s) from %s", len(s.users), s.dataFile)
}

// saveUsers — сериализует пользователей под RLock и ПОСЛЕ снятия лока
// пишет файл. ВАЖНО: не держит блокировку во время записи на диск и не
// вызывается из-под уже взятого s.mu (иначе self-deadlock на не-
// реентерабельном sync.RWMutex).
func (s *MockPaymentService) saveUsers() {
	if s.dataFile == "" {
		return
	}

	s.mu.RLock()
	data, err := json.Marshal(s.users)
	s.mu.RUnlock()
	if err != nil {
		log.Printf("⚠️ Failed to marshal premium users: %v", err)
		return
	}

	if err := os.WriteFile(s.dataFile, data, 0644); err != nil {
		log.Printf("⚠️ Failed to save premium users: %v", err)
	}
}

// MockPayment — мок-запись платежа.
type MockPayment struct {
	PaymentID string
	UserID    int64
	TariffID  string
	Amount    int
	Status    string // "pending", "succeeded", "cancelled"
	CreatedAt time.Time
}

// CreatePayment — создаёт платеж (мокирует YooKassa).
func (s *MockPaymentService) CreatePayment(req PaymentRequest) (*PaymentResponse, error) {
	log.Printf(locales.LogPaymentCreate, req.UserID, req.TariffID)

	tariff := GetTariffByID(req.TariffID)
	if tariff == nil {
		return nil, fmt.Errorf(locales.ErrTariffNotFound, req.TariffID)
	}

	paymentID := fmt.Sprintf("pay_%d_%d", req.UserID, time.Now().Unix())

	payment := &MockPayment{
		PaymentID: paymentID,
		UserID:    req.UserID,
		TariffID:  req.TariffID,
		Amount:    tariff.Price,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.payments[paymentID] = payment
	s.mu.Unlock()

	log.Printf(locales.LogPaymentCreated, paymentID, tariff.Name, tariff.Price/100)

	// В реальном YooKassa здесь была бы генерация ссылки на оплату
	// Для мока — возвращаем тестовую ссылку
	return &PaymentResponse{
		PaymentID: paymentID,
		URL:       fmt.Sprintf("https://pay.test/checkout/%s", paymentID),
		Amount:    tariff.Price,
	}, nil
}

// HandleWebhook — обрабатывает вебхук от YooKassa.
func (s *MockPaymentService) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var webhook YookassaWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf(locales.LogPaymentWebhookReceived, webhook.Type, webhook.Object.ID)

	if webhook.Type != "payment.succeeded" && webhook.Type != "payment.waiting_for_completion" {
		log.Printf(locales.LogPaymentWebhookIgnored, webhook.Type)
		http.Error(w, "Ignored", http.StatusOK)
		return
	}

	if webhook.Object.Status != "succeeded" {
		log.Printf(locales.LogPaymentNotSucceeded, webhook.Object.ID)
		http.Error(w, "Not succeeded", http.StatusOK)
		return
	}

	// Активируем Premium
	if err := s.activatePremium(webhook.Object.UserID, webhook.Object.TariffID); err != nil {
		log.Printf(locales.LogPaymentActivateFailed, webhook.Object.UserID, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf(locales.LogPaymentActivated, webhook.Object.UserID, webhook.Object.TariffID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// activatePremium — активирует Premium для пользователя.
// Берёт write-lock только на время мутации map, затем СНИМАЕТ его перед
// saveUsers(), иначе saveUsers() (который берёт RLock) вызвал бы
// self-deadlock на не-реентерабельном sync.RWMutex и заморозил бы весь
// сервис платежей (и дашборд, который тоже берёт лок).
func (s *MockPaymentService) activatePremium(userID int64, tariffID string) error {
	tariff := GetTariffByID(tariffID)
	if tariff == nil {
		return fmt.Errorf(locales.ErrTariffNotFound, tariffID)
	}

	now := time.Now()

	s.mu.Lock()
	// Создаём или обновляем запись пользователя
	if _, exists := s.users[userID]; !exists {
		s.users[userID] = &PremiumUser{}
	}

	user := s.users[userID]
	user.IsPremium = true
	user.PremiumSince = now
	user.PremiumExpiresAt = now.Add(tariff.Duration)
	user.TariffID = tariffID

	// Обновляем статус платежа
	for _, payment := range s.payments {
		if payment.UserID == userID && payment.TariffID == tariffID && payment.Status == "pending" {
			payment.Status = "succeeded"
			break
		}
	}

	log.Printf(locales.LogPaymentPremiumActivated, userID, tariff.Name, user.PremiumExpiresAt.Format("2006-01-02"))
	s.mu.Unlock()

	// Сохраняем состояние в файл (saveUsers берёт свой собственный RLock —
	// write-lock уже снят, поэтому deadlock не возникает).
	s.saveUsers()

	return nil
}

// IsPremium — проверка, является ли пользователь Premium.
// Используем write-lock: при истечении срока здесь происходит мутация
// (user.IsPremium = false), и брать RLock для записи — data race.
func (s *MockPaymentService) IsPremium(userID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return false
	}

	if !user.IsPremium {
		return false
	}

	// Проверка срока действия
	if time.Now().After(user.PremiumExpiresAt) {
		log.Printf(locales.LogPaymentExpired, userID)
		user.IsPremium = false
		return false
	}

	return true
}

// GetPremiumInfo — информация о Premium пользователя.
func (s *MockPaymentService) GetPremiumInfo(userID int64) *PremiumUser {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil
	}

	// Копируем
	cp := *user
	return &cp
}

// SetWebhookHandler — устанавливает callback для обработки вебхуков.
func (s *MockPaymentService) SetWebhookHandler(fn func(payload []byte) error) {
	s.webhookFunc = fn
}

// ActivatePremiumManually — ручная активация Premium (для симуляции оплаты).
func (s *MockPaymentService) ActivatePremiumManually(userID int64, tariffID string) error {
	return s.activatePremium(userID, tariffID)
}

// IsUserPremium — проверка, является ли пользователь Premium.
func (s *MockPaymentService) IsUserPremium(userID int64) bool {
	return s.IsPremium(userID)
}

// SimulatePaymentSuccess — симулирует успешную оплату по ID платежа.
// Находит платеж по ID и меняет его статус на "succeeded".
// Лок снимается ДО вызова activatePremium/saveUsers, чтобы избежать
// вложенной блокировки (self-deadlock на sync.RWMutex).
func (s *MockPaymentService) SimulatePaymentSuccess(paymentID string) error {
	s.mu.Lock()
	payment, exists := s.payments[paymentID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("payment %s not found", paymentID)
	}

	if payment.Status == "succeeded" {
		s.mu.Unlock()
		return nil
	}

	payment.Status = "succeeded"
	s.mu.Unlock()

	// Активируем Premium для пользователя (activatePremium сам берёт лок).
	if err := s.activatePremium(payment.UserID, payment.TariffID); err != nil {
		return err
	}

	log.Printf(locales.LogPaymentSimulatedSuccess, paymentID, payment.UserID)

	// Сохраняем состояние (saveUsers берёт свой собственный RLock).
	s.saveUsers()

	return nil
}
