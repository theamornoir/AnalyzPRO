package router

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// router - маршрутизатор сообщений с зависимостями.
type router struct {
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	uploadDir        string
	stickerID        string
	adminChatID      int64
	agreementStorage *storage.AgreementStorage
	paymentService   *payment.MockPaymentService
	monitorRepo      monitoring.HistorySaver
	webAppURL        string
	dashboardURL     string
}

// MessageRouter - главный маршрутизатор сообщений бота.
func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.MockPaymentService,
	monitorRepo monitoring.HistorySaver,
	webAppURL string,
	dashboardURL string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	r := &router{
		stateManager:     stateManager,
		analysisService:  analysisService,
		reportRenderer:   reportRenderer,
		uploadDir:        uploadDir,
		stickerID:        stickerID,
		adminChatID:      adminChatID,
		agreementStorage: agreementStorage,
		paymentService:   paymentService,
		monitorRepo:      monitorRepo,
		webAppURL:        webAppURL,
		dashboardURL:     dashboardURL,
	}

	return r.handle
}

// handle - точка входа маршрутизатора.
func (r *router) handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	// Обработка callback-запросов (inline buttons)
	if update.CallbackQuery != nil {
		r.handleCallback(ctx, b, update)
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	currentState := r.stateManager.GetState(chatID)
	agreed := r.agreementStorage.IsAgreed(chatID)
	analysisType := r.stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := r.stateManager.GetUserData(chatID, "analysis_subtype")

	log.Printf(locales.LogDiagnosticInfo,
		chatID, text, currentState, agreed, analysisType, analysisSubtype)

	// Проверка соглашения
	if r.handleAgreement(ctx, b, chatID, text) {
		return
	}

	// Кнопки главного меню имеют приоритет над «зависшим» состоянием потока
	// (например, сохранённое в states.json состояние bioscan не должно
	// «проглатывать» нажатие «📊 Мой Дашборд»). Фото/документы (text=="")
	// пропускаем — их обрабатывают шаги потока/загрузки.
	if text != "" && r.isMainMenuButton(text) {
		if r.handleMenuButtons(ctx, b, chatID, text, update) {
			return
		}
	}

	// Обработка состояний Bioscan
	if r.handleBioscanStates(ctx, b, chatID, text, update) {
		return
	}

	// Обработка кнопки "Назад"
	if r.handleBack(ctx, b, chatID, text) {
		return
	}

	// Обработка состояний опросника
	if r.handleQuestionnaireStates(ctx, b, chatID, text) {
		return
	}

	// Обработка загрузки файлов
	if r.handleUpload(ctx, b, chatID, text, update) {
		return
	}

	// Обработка кнопок меню
	if r.handleMenuButtons(ctx, b, chatID, text, update) {
		return
	}

	// Обработка обычного текста
	r.handleText(ctx, b, chatID, text)
}

// handleCallback — обработка callback-запросов от inline-кнопок.
func (r *router) handleCallback(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	callbackData := update.CallbackQuery.Data

	// У callback-запросов верхнеуровневый update.Message почти всегда nil —
	// брать chatID оттуда нельзя (nil-pointer panic). В приватном чате
	// ChatID пользователя совпадает с ID отправителя.
	chatID := update.CallbackQuery.From.ID

	log.Printf(locales.LogRouterCallback, chatID, callbackData)

	// Ловим панику в обработчике: tgbot иногда «глотает» панику, и тогда
	// сообщение не отправляется, а в логах — тишина. Логируем явно.
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("🔥 PANIC in handleCallback (chatID=%d, data=%q): %v", chatID, callbackData, rec)
		}
	}()

	// Premium confirm (симуляция оплаты → активация Premium).
	// Проверяем ДО «premium_», иначе данные вида
	// «premium_confirm_<tariffID>» попадут в HandlePremiumCallback и
	// после отрезания префикса «premium_» дадут «confirm_<tariffID>».
	if strings.HasPrefix(callbackData, "premium_confirm_") {
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "premium_confirm")
		menu.HandlePremiumConfirm(r.stateManager, r.paymentService, r.webAppURL, r.dashboardURL)(ctx, b, update, callbackData)
		log.Printf(locales.LogRouterCallbackDone, chatID, callbackData)
		return
	}

	// Premium change tariff (для уже активного Premium) — проверяем ДО
	// общего «premium_», иначе «premium_change» попал бы в выбор тарифа.
	if callbackData == "premium_change" {
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "premium_change")
		if menu.HandleChangeTariff(r.stateManager, r.paymentService)(ctx, b, update, callbackData) {
			log.Printf(locales.LogRouterCallbackDone, chatID, callbackData)
		}
		return
	}

	// Premium callback (выбор тарифа → экран оплаты)
	if strings.HasPrefix(callbackData, "premium_") {
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "premium")
		menu.HandlePremiumCallback(r.stateManager, r.paymentService)(ctx, b, update, callbackData)
		log.Printf(locales.LogRouterCallbackDone, chatID, callbackData)
		return
	}

	// Back callback
	if callbackData == "back_main" {
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "back_main")
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgStartWelcome,
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
		_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}
