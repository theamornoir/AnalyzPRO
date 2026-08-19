package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/storage/interfaces"
)

// YooKassaConfig - конфигурация реального платёжного шлюза YooKassa.
// Пустые ShopID/SecretKey означают режим симуляции (для локальной
// разработки/тестов без реальных ключей): CreatePayment возвращает тестовую
// ссылку, активация Premium идёт только через кнопку «Оплатил (симуляция)».
type YooKassaConfig struct {
	ShopID        string
	SecretKey     string
	APIURL        string // напр. https://api.yookassa.ru/v3
	ReturnURL     string // куда вернуть пользователя после оплаты
	WebhookSecret string // секрет для проверки подписи X-YooKassa-Signature (если пуст - SecretKey)
}

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

// PremiumUser - информация о премиум-пользователе.
type PremiumUser struct {
	UserID           int64     `json:"user_id"`
	IsPremium        bool      `json:"is_premium"`
	PremiumSince     time.Time `json:"premium_since"`
	PremiumExpiresAt time.Time `json:"premium_expires_at"`
	TariffID         string    `json:"tariff_id"`
}

// Payment - запись платежа (для режима симуляции и отладки).
type Payment struct {
	PaymentID string
	UserID    int64
	TariffID  string
	Amount    int
	Status    string // "pending", "succeeded", "cancelled"
	CreatedAt time.Time
}

// PaymentService - сервис платежей на базе YooKassa.
//
// Состояние Premium хранится в БД (источник истины, переживает перезапуск
// бота и защищено от конкурентных записей), а в памяти держится быстрый
// кэш (users) для горячих проверок гейтов. При каждой активации/сбросе
// состояние пишется и в БД (через usersRepo), и в кэш. При чтении
// (IsPremium/GetPremiumInfo) при промахе кэша данные подтягиваются из БД.
type PaymentService struct {
	mu          sync.RWMutex
	payments    map[string]*Payment
	users       map[int64]*PremiumUser // кэш Premium-статуса (key: telegram_id)
	webhookFunc func(payload []byte) error
	usersRepo   interfaces.UserRepository

	// YooKassa
	shopID        string
	secretKey     string
	apiURL        string
	returnURL     string
	webhookSecret string
	httpClient    *http.Client

	processedMu sync.Mutex
	processed   map[string]bool // дедуп по object.id вебхуков
}

// NewPaymentService создаёт сервис платежей. usersRepo - БД как источник
// истины для Premium-статуса (может быть nil - тогда только in-memory кэш,
// сценарий тестов/fallback).
func NewPaymentService(usersRepo interfaces.UserRepository, cfg YooKassaConfig) *PaymentService {
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" {
		apiURL = "https://api.yookassa.ru/v3"
	}
	return &PaymentService{
		payments:      make(map[string]*Payment),
		users:         make(map[int64]*PremiumUser),
		usersRepo:     usersRepo,
		shopID:        cfg.ShopID,
		secretKey:     cfg.SecretKey,
		apiURL:        apiURL,
		returnURL:     cfg.ReturnURL,
		webhookSecret: cfg.WebhookSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		processed:     make(map[string]bool),
	}
}

// IsRealMode - true, если заданы реквизиты YooKassa (реальные платежи).
func (s *PaymentService) IsRealMode() bool {
	return s.shopID != "" && s.secretKey != ""
}

// getInfo возвращает кэш PremiumUser для пользователя. При промахе кэша
// (и наличии usersRepo) подтягивает состояние из БД и кладёт в кэш.
func (s *PaymentService) getInfo(userID int64) *PremiumUser {
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

// CreatePayment - создаёт платёж. В реальном режиме вызывает YooKassa API
// и возвращает настоящую ссылку на оплату (confirmation_url). В режиме
// симуляции (нет ключей) возвращает тестовую ссылку.
func (s *PaymentService) CreatePayment(req PaymentRequest) (*PaymentResponse, error) {
	log.Printf(locales.LogPaymentCreate, req.UserID, req.TariffID)

	tariff := GetTariffByID(req.TariffID)
	if tariff == nil {
		return nil, fmt.Errorf(locales.ErrTariffNotFound, req.TariffID)
	}

	if s.IsRealMode() {
		return s.createYooKassaPayment(req, tariff)
	}

	// Режим симуляции (нет ключей YooKassa): мок-платёж + тестовая ссылка.
	paymentID := fmt.Sprintf("pay_%d_%d", req.UserID, time.Now().Unix())
	s.mu.Lock()
	s.payments[paymentID] = &Payment{
		PaymentID: paymentID,
		UserID:    req.UserID,
		TariffID:  req.TariffID,
		Amount:    tariff.Price,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	s.mu.Unlock()
	log.Printf(locales.LogPaymentCreated, paymentID, tariff.Name, tariff.Price/100)
	return &PaymentResponse{
		PaymentID: paymentID,
		URL:       fmt.Sprintf("https://pay.test/checkout/%s", paymentID),
		Amount:    tariff.Price,
	}, nil
}

// ykAmount / ykConfirmation / ykCreateRequest / ykCreateResponse - тело
// запроса и ответа YooKassa API создания платежа.
type ykAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}
type ykConfirmation struct {
	Type      string `json:"type"`
	ReturnURL string `json:"return_url"`
}
type ykCreateRequest struct {
	Amount      ykAmount           `json:"amount"`
	Confirmation ykConfirmation    `json:"confirmation"`
	Capture      bool              `json:"capture"`
	Description  string            `json:"description"`
	Metadata     map[string]string `json:"metadata"`
}
type ykCreateResponse struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
}

func basicAuth(shopID, secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(shopID + ":" + secret))
}

// createYooKassaPayment - реальный вызов YooKassa API: создаёт платёж с
// редиректом на checkout и мета-данными (user_id, tariff_id), которые
// эхо-вернутся в вебхуке (оттуда мы узнаем, кого активировать).
func (s *PaymentService) createYooKassaPayment(req PaymentRequest, tariff *Tariff) (*PaymentResponse, error) {
	body := ykCreateRequest{
		Amount: ykAmount{
			Value:    fmt.Sprintf("%.2f", float64(tariff.Price)/100.0),
			Currency: "RUB",
		},
		Confirmation: ykConfirmation{
			Type:      "redirect",
			ReturnURL: s.returnURL,
		},
		Capture:     true,
		Description: "Prisma Premium: " + tariff.Name,
		Metadata: map[string]string{
			"user_id":   strconv.FormatInt(req.UserID, 10),
			"tariff_id": tariff.ID,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, s.apiURL+"/payments", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotence-Key", uuid.NewString())
	httpReq.Header.Set("Authorization", "Basic "+basicAuth(s.shopID, s.secretKey))

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("yookassa create: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("yookassa create: http %d: %s", resp.StatusCode, string(respBody))
	}

	var ykResp ykCreateResponse
	if err := json.Unmarshal(respBody, &ykResp); err != nil {
		return nil, fmt.Errorf("yookassa decode: %w", err)
	}
	if ykResp.ID == "" {
		return nil, fmt.Errorf("yookassa: empty payment id")
	}
	log.Printf(locales.LogPaymentCreated, ykResp.ID, tariff.Name, tariff.Price/100)
	return &PaymentResponse{
		PaymentID: ykResp.ID,
		URL:       ykResp.Confirmation.ConfirmationURL,
		Amount:    tariff.Price,
	}, nil
}

// ykObject / ykWebhook - тело реального вебхука YooKassa.
type ykObject struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Amount   ykAmount          `json:"amount"`
	Metadata map[string]string `json:"metadata"`
}
type ykWebhook struct {
	Type   string   `json:"type"`
	Event  string   `json:"event"`
	Object ykObject `json:"object"`
}

// HandleWebhook - обработчик вебхука от YooKassa (POST /api/payment/webhook).
// Проверяет подпись X-YooKassa-Signature (HMAC-SHA256 от тела ключом
// WebhookSecret/SecretKey), дедуплицирует по object.id и на успешном
// платеже активирует Premium (с записью в БД - источник истины).
func (s *PaymentService) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if s.IsRealMode() {
		sig := r.Header.Get("X-YooKassa-Signature")
		if !s.verifySignature(body, sig) {
			log.Printf("[PAYMENT] вебхук YooKassa: неверная подпись - отклонён")
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
	}

	var wh ykWebhook
	if err := json.Unmarshal(body, &wh); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	event := wh.Event
	if event == "" {
		event = wh.Type
	}

	// Игнорируем всё, кроме успешного платежа.
	if event != "payment.succeeded" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	if wh.Object.Status != "succeeded" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	// Дедуп: один платёж не должен активировать Premium дважды (YooKassa
	// может повторно слать вебхук при сбоях доставки).
	s.processedMu.Lock()
	if s.processed[wh.Object.ID] {
		s.processedMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	s.processed[wh.Object.ID] = true
	s.processedMu.Unlock()

	userID, perr := strconv.ParseInt(wh.Object.Metadata["user_id"], 10, 64)
	if perr != nil {
		log.Printf("[PAYMENT] вебхук: некорректный user_id (metadata=%v): %v", wh.Object.Metadata, perr)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	tariffID := wh.Object.Metadata["tariff_id"]
	if tariffID == "" {
		log.Printf("[PAYMENT] вебхук: пустой tariff_id (user=%d)", userID)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := s.activatePremium(userID, tariffID); err != nil {
		log.Printf(locales.LogPaymentActivateFailed, userID, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf(locales.LogPaymentActivated, userID, tariffID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// verifySignature проверяет HMAC-SHA256 подпись вебхука YooKassa.
func (s *PaymentService) verifySignature(body []byte, sig string) bool {
	secret := s.webhookSecret
	if secret == "" {
		secret = s.secretKey
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// activatePremium - активирует Premium для пользователя. Обновляет in-memory
// кэш и синхронно пишет состояние в БД (usersRepo) - источник истины,
// переживающий перезапуск бота. Берёт write-lock только на время мутации
// карты; обращение к БД идёт УЖЕ ПОСЛЕ снятия лока, чтобы не держать
// sync.RWMutex во время I/O.
func (s *PaymentService) activatePremium(userID int64, tariffID string) error {
	tariff := GetTariffByID(tariffID)
	if tariff == nil {
		return fmt.Errorf(locales.ErrTariffNotFound, tariffID)
	}

	now := time.Now()

	s.mu.Lock()
	s.users[userID] = &PremiumUser{
		UserID:           userID,
		IsPremium:        true,
		PremiumSince:     now,
		PremiumExpiresAt: now.Add(tariff.Duration),
		TariffID:         tariffID,
	}

	for _, payment := range s.payments {
		if payment.UserID == userID && payment.TariffID == tariffID && payment.Status == "pending" {
			payment.Status = "succeeded"
			break
		}
	}
	s.mu.Unlock()

	log.Printf(locales.LogPaymentPremiumActivated, userID, tariff.Name, now.Add(tariff.Duration).Format("2006-01-02"))

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
// подгрузкой из БД при промахе). НЕ мутирует состояние.
func (s *PaymentService) IsPremium(userID int64) bool {
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
func (s *PaymentService) GetPremiumInfo(userID int64) *PremiumUser {
	info := s.getInfo(userID)
	if info == nil {
		return nil
	}

	s.mu.RLock()
	cp := *info
	s.mu.RUnlock()
	return &cp
}

// SetWebhookHandler - устанавливает callback для обработки вебхуков.
func (s *PaymentService) SetWebhookHandler(fn func(payload []byte) error) {
	s.webhookFunc = fn
}

// ActivatePremiumManually - ручная активация Premium (промокоды и
// симуляция оплаты в development).
func (s *PaymentService) ActivatePremiumManually(userID int64, tariffID string) error {
	return s.activatePremium(userID, tariffID)
}

// IsUserPremium - проверка, является ли пользователь Premium.
func (s *PaymentService) IsUserPremium(userID int64) bool {
	return s.IsPremium(userID)
}

// ResetPremium сбрасывает Premium-статус пользователя. Используется
// админ-командой для тестирования полного цикла покупки. Синхронно пишется
// и в кэш, и в БД.
func (s *PaymentService) ResetPremium(userID int64) {
	s.mu.Lock()
	s.users[userID] = &PremiumUser{
		UserID:   userID,
		IsPremium: false,
		TariffID: "",
	}
	s.mu.Unlock()

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

// SimulatePaymentSuccess - симулирует успешную оплату по ID платежа
// (режим симуляции).
func (s *PaymentService) SimulatePaymentSuccess(paymentID string) error {
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

	if err := s.activatePremium(payment.UserID, payment.TariffID); err != nil {
		return err
	}

	log.Printf(locales.LogPaymentSimulatedSuccess, paymentID, payment.UserID)

	return nil
}
