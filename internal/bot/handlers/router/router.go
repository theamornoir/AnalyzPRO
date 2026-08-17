package router

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/upload"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// router - маршрутизатор сообщений с зависимостями.
type router struct {
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	pdfConverter     pdfservice.Converter
	uploadDir        string
	stickerID        string
	adminChatID      int64
	agreementStorage *storage.AgreementStorage
	paymentService   *payment.MockPaymentService
	appStorage       *storage.Storage
	monitorRepo      monitoring.Repository
	webAppURL        string
	dashboardURL     string
}

// MessageRouter - главный маршрутизатор сообщений бота.
func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.MockPaymentService,
	appStorage *storage.Storage,
	monitorRepo monitoring.Repository,
	webAppURL string,
	dashboardURL string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	r := &router{
		stateManager:     stateManager,
		analysisService:  analysisService,
		reportRenderer:   reportRenderer,
		pdfConverter:     pdfConverter,
		uploadDir:        uploadDir,
		stickerID:        stickerID,
		adminChatID:      adminChatID,
		agreementStorage: agreementStorage,
		paymentService:   paymentService,
		appStorage:       appStorage,
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

	// Режим «Быстрая консультация (с ИИ)»: перехватываем ЛЮБОЕ сообщение
	// пользователя (текстовый вопрос или фото), чтобы отправить его ИИ.
	// Перехват идёт ДО режима отзыва и ДО обычной загрузки файлов (иначе
	// фото «проглотилось» бы загрузчиком анализов). Нажатие кнопки главного
	// меню во время консультации выходит из режима и навигирует как обычно.
	if r.stateManager.GetState(chatID) == states.StateWaitingConsultation {
		if text != "" && r.isMainMenuButton(text) {
			r.stateManager.SetState(chatID, states.StateIdle)
		} else if r.handleConsultationMessage(ctx, b, chatID, text, update) {
			return
		}
	}

	// Режим ввода отзыва: перехватываем ЛЮБОЕ сообщение пользователя
	// (текст/фото/документ), чтобы переслать его разработчику. Перехват
	// идёт до приоритета кнопок главного меню — иначе отзыв, случайно
	// совпадающий по тексту с кнопкой меню, «проглотился» бы как команда.
	// Исключение: нажатие самой кнопки главного меню во время ввода отзыва
	// выходит из режима отзыва и навигирует как обычно (иначе меню
	// «залипло» бы в режиме ввода).
	if r.stateManager.GetState(chatID) == states.StateWaitingFeedback {
		if text != "" && r.isMainMenuButton(text) {
			r.stateManager.SetState(chatID, states.StateIdle)
		} else if r.handleFeedbackMessage(ctx, b, chatID, text, update) {
			return
		}
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

	// «Назад» из любого раздела/флоу (блок-хаб или сообщение под-действия
	// вроде Сводки/Мониторинга). Удаляем само сообщение с inline-кнопкой, а
	// дальше — иерархический возврат (на уровень выше: в хаб раздела, либо из
	// хаба — в Главное меню). Поведение совпадает с reply-кнопкой «⬅️ Назад»
	// (handleBack) — единый UX на всех этапах.
	if callbackData == "hub_back" || callbackData == "msg_back" {
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, callbackData)

		// Удаляем именно это сообщение (id сообщения под-действия).
		if mm := update.CallbackQuery.Message; mm.Message != nil {
			helpers.DeleteMessage(ctx, b, chatID, mm.Message.ID)
		}
		r.backToParent(ctx, b, chatID)

		_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	// Под-действия из карточек разделов-хабов («Анализы», «Здоровье»,
	// «Сервис»). Диспетчеризуем на существующие обработчики; сам
	// callback-запрос отвечается в конце функции (спиннер кнопки).
	switch callbackData {
	case "section_analysis":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_analysis")
		r.renderHub(ctx, b, chatID, "analysis")
	case "section_health":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_health")
		r.renderHub(ctx, b, chatID, "health")
	case "section_service":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_service")
		r.renderHub(ctx, b, chatID, "service")
	case "section_diag_regular":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_diag_regular")
		r.handleRegularAnalysis(ctx, b, chatID)
	case "section_diag_extended":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_diag_extended")
		r.handleExtendedAnalysis(ctx, b, chatID)
	case "section_bioscan_basic":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_bioscan_basic")
		r.handleBioscanBasicStart(ctx, b, chatID)
	case "section_bioscan_extended":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_bioscan_extended")
		r.handleBioscanExtendedStart(ctx, b, chatID)
	case "section_diag_regular_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_diag_regular_demo")
		r.handleRegularAnalysisDemo(ctx, b, chatID)
	case "section_diag_extended_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_diag_extended_demo")
		r.handleExtendedAnalysisDemo(ctx, b, chatID)
	case "section_bioscan_basic_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_bioscan_basic_demo")
		r.handleBioscanBasicDemo(ctx, b, chatID)
	case "section_bioscan_extended_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_bioscan_extended_demo")
		r.handleBioscanExtendedDemo(ctx, b, chatID)
	case "section_diag_extended_demo2":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_diag_extended_demo2")
		r.handleExtendedAnalysisRepeatDemo(ctx, b, chatID)
	case "section_bioscan_extended_demo2":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_bioscan_extended_demo2")
		r.handleBioscanExtendedRepeatDemo(ctx, b, chatID)
	case "section_health_summary":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_health_summary")
		r.handleDashboard(ctx, b, chatID, false)
	case "section_health_summary_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_health_summary_demo")
		r.handleDashboard(ctx, b, chatID, true)
	case "section_health_monitoring":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_health_monitoring")
		r.handleMonitoring(ctx, b, chatID, false)
	case "section_health_monitoring_demo":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_health_monitoring_demo")
		r.handleMonitoring(ctx, b, chatID, true)
	case "section_consult_start":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_consult_start")
		r.handleConsultationStart(ctx, b, chatID)
	case "section_feedback_start":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_feedback_start")
		r.handleFeedbackStart(ctx, b, chatID)
	case "section_about":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "section_about")
		// Выбрано под-действие — убираем блок-хаб.
		r.deleteHubBlock(ctx, b, chatID)
		r.setCurrentSection(chatID, "service")
		menu.AboutHandler()(ctx, b, update)

	// Подтверждение загрузки файлов (inline-кнопки «Обработать/Отмена»).
	case "upload_process":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "upload_process")
		upload.StartAnalysis(ctx, b, r.stateManager, r.analysisService, r.reportRenderer, r.pdfConverter, r.uploadDir, r.stickerID, chatID, r.appStorage, r.monitorRepo)
	case "upload_cancel":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "upload_cancel")
		upload.CancelUpload(ctx, b, r.stateManager, chatID)

	// Подтверждение/перезапуск Bioscan (inline-кнопки на экране фото).
	case "bioscan_confirm":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "bioscan_confirm")
		bioscan.ProcessBioscanWithPhotos(ctx, b, r.stateManager, r.analysisService, r.pdfConverter, r.uploadDir, r.stickerID, chatID, r.appStorage, r.monitorRepo)
	case "bioscan_restart":
		log.Printf(locales.LogRouterCallbackDispatch, chatID, callbackData, "bioscan_restart")
		bioscan.StartBioscanFlow(ctx, b, r.stateManager, chatID)
	}

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}
