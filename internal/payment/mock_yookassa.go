package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
)

// PaymentRequest - запрос на создание платежа.
type PaymentRequest struct {
	UserID   int64  `json:"user_id"`
	TariffID string `json:"tariff_id"`
}

// PaymentResponse - ответ с ссылкой на оплату.
type PaymentResponse struct {
	PaymentID string `json:"payment_id"`
	URL       string `json:"url"`
	Amount    int    `json:"amount"`
}

// YookassaWebhook - входящий вебхук от YooKassa.
type YookassaWebhook struct {
	Type   string      `json:"type"`
	Object WebhookData `json:"object"`
}

// WebhookData - данные события вебхука.
type WebhookData struct {
	ID        string    `json:"id"`
	Status    string    `json:"succeeded"`
	Amount    int64     `json:"amount"`
	UserID    int64     `json:"user_id"` // кастомное поле
	TariffID  string    `json:"tariff_id"`
	CreatedAt time.Time `json:"created_at"`
}

// MockPaymentService - мок-сервис платежей (замена YooKassa).
//
// Состояние Premium хранится в БД (источник истины, переживает перезапуск
// бота и защищено от конкурентных записей SQLite), а в памяти держится
// быстрый кэш (users) для горячих проверок гейтов. При каждой активации/
// сбросе состояние пишется и в БД (через usersRepo), и в кэш. При чтении
// (IsPremium/GetPremiumInfo) при промахе кэша данные подтягиваются из БД.
//
// Ранее состояние дублировалось в JSON-файл (premium_users.json), что было
// хрупко: файл перезаписывался целиком, не был concurrent-safe при
// нескольких инстансах и рассинхронизировался с БД. Теперь файл не
// используется.
type MockPaymentService struct {
	mu          sync.RWMutex
	payments    map[string]*MockPayment
	users       map[int64]*PremiumUser // кэш Premium-статуса (key: telegram_id)
	webhookFunc func(payload []byte) error // callback для обработки вебхуков
	usersRepo   interfaces.UserRepository // БД как источник истины (может быть nil - тогда только in-memory кэш)
}

// PremiumUser - информация о премиум-пользователе.
type PremiumUser struct {
	UserID           int64     `json:"user_id"`
	IsPremium        bool      `json:"is_premium"`
	PremiumSince     time.Time `json:"premium_since"`
	PremiumExpiresAt time.Time `json:"premium_expires_at"`
	TariffID         string    `json:"tariff_id"`
}

// NewMockPaymentService создаёт мок-сервис платежей.
// usersRepo - репозиторий пользователей (БД) как источник истины для
// Premium-статуса. Может быть nil - тогда сервис работает только в
// in-memory кэше (используется в тестах и как безопасный fallback, если
// репозиторий недоступен).
func NewMockPaymentService(usersRepo interfaces.UserRepository) *MockPaymentService {
	return &MockPaymentService{
		payments:  make(map[string]*MockPayment),
		users:     make(map[int64]*PremiumUser),
		usersRepo: usersRepo,
	}
}

// getInfo возвращает кэш PremiumUser для пользователя. При промахе кэша
// (и наличии usersRepo) подтягивает состояние из БД и кладёт в кэш.
// Если пользователь не найден и в БД - возвращает nil (без кэширования
// результата, чтобы транзитный сбой БД не помечал реального Premium
// пользователя как не-Premium навсегда).
func (s *MockPaymentService) getInfo(userID int64) *PremiumUser {
	s.mu.RLock()
	u, ok := s.users[userID]
	s.mu.RUnlock()
	if ok {
		return u
	}

	if s.usersRepo != nil {
		if stored, err := s.usersRepo.GetUserByTelegramID(context.Background(), userID); err == nil && stored != nil {
			pu := &PremiumUser{
				UserID:           userID,
				IsPremium:        stored.IsPremium,
				PremiumExpiresAt: stored.PremiumExpiresAt,
				TariffID:         stored.TariffID,
			}
			s.mu.Lock()
			// Повторная проверка: кэш мог заполниться за время
			// обращения к БД из другой горутины.
			if existing, exists := s.users[userID]; exists {
				u = existing
			} else {
				s.users[userID] = pu
				u = pu
			}
			s.mu.Unlock()
			return u
		}
	}

	return nil
}

// MockPayment - мок-запись платежа.
type MockPayment struct {
	PaymentID string
	UserID    int64
	TariffID  string
	Amount    int
	Status    string // "pending", "succeeded", "cancelled"
	CreatedAt time.Time
}

// CreatePayment - создаёт платеж (мокирует YooKassa).
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
	// Для мока - возвращаем тестовую ссылку
	return &PaymentResponse{
		PaymentID: paymentID,
		URL:       fmt.Sprintf("https://pay.test/checkout/%s", paymentID),
		Amount:    tariff.Price,
	}, nil
}

// HandleWebhook - обрабатывает вебхук от YooKassa.
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

	// Активируем Premium (пишется и в кэш, и в БД - источник истины).
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

// activatePremium - активирует Premium для пользователя. Обновляет in-memory
// кэш и синхронно пишет состояние в БД (usersRepo) - источник истины,
// переживающий перезапуск бота. Берёт write-lock только на время мутации
// карты; обращение к БД идёт УЖЕ ПОСЛЕ снятия лока, чтобы не держать
// sync.RWMutex во время I/O (и не блокировать горячие чтения гейтов).
func (s *MockPaymentService) activatePremium(userID int64, tariffID string) error {
	tariff := GetTariffByID(tariffID)
	if tariff == nil {
		return fmt.Errorf(locales.ErrTariffNotFound, tariffID)
	}

	now := time.Now()

	s.mu.Lock()
	// Создаём или обновляем запись пользователя в кэше.
	s.users[userID] = &PremiumUser{
		UserID:           userID,
		IsPremium:        true,
		PremiumSince:     now,
		PremiumExpiresAt: now.Add(tariff.Duration),
		TariffID:         tariffID,
	}

	// Обновляем статус платежа в кэше.
	for _, payment := range s.payments {
		if payment.UserID == userID && payment.TariffID == tariffID && payment.Status == "pending" {
			payment.Status = "succeeded"
			break
		}
	}
	s.mu.Unlock()

	log.Printf(locales.LogPaymentPremiumActivated, userID, tariff.Name, now.Add(tariff.Duration).Format("2006-01-02"))

	// Синхронизируем с БД (источник истины). При отсутствии usersRepo
	// (nil) состояние живёт только в кэше (сценарий тестов/fallback).
	if s.usersRepo != nil {
		if u, err := s.usersRepo.GetUserByTelegramID(context.Background(), userID); err == nil {
			if derr := s.usersRepo.UpdateUserPremiumStatus(context.Background(), u.ID, true, now.Add(tariff.Duration), tariffID); derr != nil {
				log.Printf("⚠️ Failed to persist premium to DB (user=%d, tariff=%s): %v", userID, tariffID, derr)
			}
		} else {
			log.Printf("⚠️ Cannot sync premium to DB: user %d not found in repository: %v", userID, err)
		}
	}

	return nil
}

// IsPremium - проверка, является ли пользователь Premium в данный момент
// (флаг активен И подписка не истекла). Читает из кэша (с ленивой
// подгрузкой из БД при промахе). НЕ мутирует состояние - истечение срока
// проверяется на лету, чтобы не портить флаг, нужный напоминаниям об
// окончании подписки (см. notifications.Service.rawPremium).
func (s *MockPaymentService) IsPremium(userID int64) bool {
	info := s.getInfo(userID)
	if info == nil || !info.IsPremium {
		return false
	}
	return time.Now().Before(info.PremiumExpiresAt)
}

// GetPremiumInfo - информация о Premium пользователя (флаг, тариф, дата
// окончания). Флаг IsPremium отражает ФАКТ активации (до сброса) и НЕ
// гасится при истечении срока - это нужно напоминаниям об окончании
// подписки. Для проверки «активен сейчас» используйте IsPremium.
func (s *MockPaymentService) GetPremiumInfo(userID int64) *PremiumUser {
	info := s.getInfo(userID)
	if info == nil {
		return nil
	}

	// Копируем под RLock, чтобы вызывающий не мутировал внутренний объект.
	s.mu.RLock()
	cp := *info
	s.mu.RUnlock()
	return &cp
}

// SetWebhookHandler - устанавливает callback для обработки вебхуков.
func (s *MockPaymentService) SetWebhookHandler(fn func(payload []byte) error) {
	s.webhookFunc = fn
}

// ActivatePremiumManually - ручная активация Premium (для симуляции оплаты).
func (s *MockPaymentService) ActivatePremiumManually(userID int64, tariffID string) error {
	return s.activatePremium(userID, tariffID)
}

// IsUserPremium - проверка, является ли пользователь Premium.
func (s *MockPaymentService) IsUserPremium(userID int64) bool {
	return s.IsPremium(userID)
}

// ResetPremium сбрасывает Premium-статус пользователя (IsPremium=false и
// срок действия - в ноль). Используется админ-командой для тестирования
// полного цикла покупки. Синхронно пишется и в кэш, и в БД.
func (s *MockPaymentService) ResetPremium(userID int64) {
	s.mu.Lock()
	s.users[userID] = &PremiumUser{
		UserID:   userID,
		IsPremium: false,
		TariffID: "",
	}
	s.mu.Unlock()

	// Синхронизируем с БД.
	if s.usersRepo != nil {
		if u, err := s.usersRepo.GetUserByTelegramID(context.Background(), userID); err == nil {
			if derr := s.usersRepo.UpdateUserPremiumStatus(context.Background(), u.ID, false, time.Time{}, ""); derr != nil {
				log.Printf("⚠️ Failed to reset premium in DB (user=%d): %v", userID, derr)
			}
		} else {
			log.Printf("⚠️ Cannot reset premium in DB: user %d not found in repository: %v", userID, err)
		}
	}
}

// SimulatePaymentSuccess - симулирует успешную оплату по ID платежа.
// Находит платеж по ID и меняет его статус на "succeeded".
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

	// Активируем Premium для пользователя (activatePremium сам берёт лок и
	// синхронизирует БД).
	if err := s.activatePremium(payment.UserID, payment.TariffID); err != nil {
		return err
	}

	log.Printf(locales.LogPaymentSimulatedSuccess, paymentID, payment.UserID)

	return nil
}
