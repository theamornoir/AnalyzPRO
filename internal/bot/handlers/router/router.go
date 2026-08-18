package router

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/dashboard"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/onboarding"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/upload"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/notifications"
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
	// notifSvc - указатель на ПОЛЕ notificationsService бота (а не сам
	// сервис). Бот строит роутер внутри New(), ДО того как снаружи
	// вызывается SetNotificationsService (в app.go). Поэтому на момент
	// построения роутера поле ещё nil. Чтобы роутер видел сервис, заданный
	// позже, храним указатель на поле и разыменовываем его в момент
	// отправки (getNotificationsSvc). Без этого тест-уведомления из
	// инлайн-меню («🧪 Тест уведомлений») падали бы с «неизвестный тип»,
	// т.к. r.notificationsSvc оставался nil.
	notifSvc     **notifications.Service
	webAppURL    string
	dashboardURL string
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
	notifSvcPtr **notifications.Service,
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
		notifSvc:         notifSvcPtr,
		webAppURL:        webAppURL,
		dashboardURL:     dashboardURL,
	}

	return r.handle
}

// getNotificationsSvc возвращает сервис уведомлений, разыменовывая
// указатель на ПОЛЕ бота, заданное позже через SetNotificationsService.
// Если поле ещё не задано (nil) - возвращает nil (роутер не должен падать).
func (r *router) getNotificationsSvc() *notifications.Service {
	if r.notifSvc == nil {
		return nil
	}
	return *r.notifSvc
}

// handle - точка входа маршрутизатора.
func (r *router) handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	// Ловим панику на уровне всего обработчика сообщения: tgbot иногда
	// «глотает» панику внутри горутины апдейта, из-за чего сообщение
	// теряется и не обрабатывается повторно. Логируем явно (как в
	// handleCallback), чтобы видеть причину в логах вместо тишины.
	var panicChatID int64
	if update.CallbackQuery != nil {
		panicChatID = update.CallbackQuery.From.ID
	} else if update.Message != nil {
		panicChatID = update.Message.Chat.ID
	}
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("🔥 PANIC in handle (chatID=%s): %v", dashboard.MaskID(panicChatID), rec)
		}
	}()

	// Обновляем дату последнего взаимодействия (нужно системе напоминаний
	// об неактивности - напоминание о повторном анализе). Делаем до
	// основной обработки, чтобы любое сообщение/нажатие кнопки считалось
	// активностью. Ошибки не фатальны - просто логируем.
	if r.appStorage != nil {
		var activityChatID int64
		if update.CallbackQuery != nil {
			activityChatID = update.CallbackQuery.From.ID
		} else if update.Message != nil {
			activityChatID = update.Message.Chat.ID
		}
		if activityChatID != 0 {
			_ = r.appStorage.TouchActivity(ctx, activityChatID)
		}
	}

	// Аналитика PostHog: фиксируем КАЖДОЕ взаимодействие пользователя как
	// событие «interaction» (нажатие inline/reply-кнопки, команда, сообщение).
	// Это гарантирует полный clickstream в дашборде, независимо от того,
	// какая ветка обработки дальше сработает. Предметные события
	// (user_started, analysis_processed и т.д.) остаются и дополняют его.
	if update.CallbackQuery != nil {
		analytics.TrackInteraction(update.CallbackQuery.From.ID, "callback", update.CallbackQuery.Data, nil)
	} else if update.Message != nil {
		text := strings.TrimSpace(update.Message.Text)
		source := "message"
		if strings.HasPrefix(text, "/") {
			source = "command"
		}
		media := "text"
		switch {
		case update.Message.Photo != nil:
			media = "photo"
		case update.Message.Document != nil:
			media = "document"
		case update.Message.Voice != nil:
			media = "voice"
		case update.Message.Video != nil:
			media = "video"
		}
		analytics.TrackInteraction(update.Message.Chat.ID, source, text, map[string]interface{}{"media": media})
	}

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
		dashboard.MaskID(chatID), text, currentState, agreed, analysisType, analysisSubtype)

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
	// идёт до приоритета кнопок главного меню - иначе отзыв, случайно
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
	// пропускаем - их обрабатывают шаги потока/загрузки.
	if text != "" && r.isMainMenuButton(text) {
		if r.handleMenuButtons(ctx, b, chatID, text, update) {
			// Правило «кнопка/выбор удаляется после ответа»: нажата
			// reply-кнопка меню (хаб Анализы/Здоровье/Сервис/Premium или
			// под-действие) - убираем текстовое сообщение пользователя.
			r.deleteUserMessageAfterReply(ctx, b, update)
			return
		}
	}

	// Обработка состояний Bioscan
	if r.handleBioscanStates(ctx, b, chatID, text, update) {
		return
	}

	// Обработка кнопки "Назад"
	if r.handleBack(ctx, b, chatID, text) {
		// Правило «кнопка/выбор удаляется после ответа»: убираем сообщение
		// пользователя с кнопкой «⬅️ Назад».
		r.deleteUserMessageAfterReply(ctx, b, update)
		return
	}

	// Обработка кнопки "Отмена" внутри анкеты/опросника (выход без сохранения)
	if r.handleCancel(ctx, b, chatID, text) {
		// Правило «кнопка/выбор удаляется после ответа»: убираем сообщение
		// пользователя с кнопкой «❌ Отмена».
		r.deleteUserMessageAfterReply(ctx, b, update)
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
		// Правило «кнопка/выбор удаляется после ответа»: резервный вызов
		// (не main-menu-кнопка, но всё же кнопка меню) - убираем сообщение.
		r.deleteUserMessageAfterReply(ctx, b, update)
		return
	}

	// Обработка обычного текста
	r.handleText(ctx, b, chatID, text)
}

// deleteUserMessageAfterReply - реализует глобальное правило «кнопка/выбор
// удаляется после того, как бот ответил» для нажатий reply-кнопок меню
// (включая под-действия разделов), «⬅️ Назад» и «❌ Отмена». Удаляет
// текстовое сообщение пользователя с кнопкой спустя небольшую задержку
// (чтобы он успел увидеть свой выбор), не засоряя историю чата.
//
// Фото/документы/вложения не трогаем (у них Text пустой) - их удаление могло
// бы помешать обработке загрузки или оставить пользователя без видимого
// следа отправленного файла.
func (r *router) deleteUserMessageAfterReply(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	m := update.Message
	if m == nil || m.ID == 0 {
		return
	}
	if strings.TrimSpace(m.Text) == "" {
		return
	}
	helpers.DeleteAfterReply(ctx, b, m.Chat.ID, m.ID)
}

// deleteCallbackMessageAfterReply - реализует глобальное правило «кнопка/
// выбор удаляется после того, как бот ответил» для inline-нажатий, где бот
// отправляет НОВОЕ сообщение (а не редактирует текущее). Удаляет исходное
// сообщение с inline-клавиатурой спустя небольшую задержку после ответа.
//
// Используется для семейства Premium (выбор тарифа / подтверждение оплаты /
// смена тарифа) - во всех этих случаях бот шлёт новое сообщение, а старое с
// кнопками становится «мусором» в истории.
//
// НЕ применяется там, где хендлер РЕДАКТИРУЕТ то же сообщение (переключение
// вкладок хаба, подтверждение загрузки/биоскана) - там удалять нельзя, иначе
// исчезнет сам ответ.
func (r *router) deleteCallbackMessageAfterReply(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil || cq.Message.Message == nil {
		return
	}
	m := cq.Message.Message
	if m.ID == 0 {
		return
	}
	helpers.DeleteAfterReply(ctx, b, cq.From.ID, m.ID)
}

// isHubTabSwitch - true для inline-кнопок переключения вкладок раздела-хаба
// (Анализы/Здоровье/Сервис). Эти хендлеры РЕДАКТИРУЮТ исходное сообщение на
// месте (renderHub → editHubPair), поэтому его нельзя удалять по глобальному
// правилу «кнопка/выбор удаляется после ответа» - иначе исчезнет сам ответ.
// Все прочие inline-нажатия отправляют НОВОЕ сообщение, и их исходное
// сообщение с клавиатурой должно быть убрано.
func isHubTabSwitch(callbackData string) bool {
	switch callbackData {
	case "section_analysis", "section_health", "section_service":
		return true
	}
	return false
}

// handleOnboarding обрабатывает callback-запросы онбординга (префикс
// onboarding_). Удаляет сообщение с нажатой кнопкой (чтобы не плодить
// историю чата) и отправляет следующий шаг / соглашение / финал.
// Возвращает true, если callback принадлежит онбордингу и обработан.
func (r *router) handleOnboarding(ctx context.Context, b *tgbot.Bot, chatID int64, callbackData string, update *models.Update) bool {
	if !strings.HasPrefix(callbackData, "onboarding_") {
		return false
	}

	// Удаляем сообщение с нажатой кнопкой - онбординг не должен
	// оставлять «мусор» в истории чата.
	if m := update.CallbackQuery.Message; m.Message != nil {
		helpers.DeleteMessage(ctx, b, chatID, m.Message.ID)
	}

	switch {
	case callbackData == "onboarding_agreement":
		// Шаг 4 → соглашение.
		onboarding.SendAgreement(ctx, b, chatID)
	case callbackData == "onboarding_accept":
		// Финал: фиксируем соглашение + онбординг, показываем главное меню.
		r.agreementStorage.SetAgreed(chatID)
		if r.appStorage != nil {
			_ = r.appStorage.SetOnboardingCompleted(ctx, chatID, true)
		}
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgOnboardingDone,
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
	default:
		// Переход к следующему шагу: onboarding_step_N (N от 2 до 4).
		var nextStep int
		if _, err := fmt.Sscanf(callbackData, "onboarding_step_%d", &nextStep); err == nil {
			if nextStep >= 2 && nextStep <= len(onboarding.Steps) {
				onboarding.SendStep(ctx, b, chatID, nextStep)
			}
		}
	}

	_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	return true
}

// handleCallback - обработка callback-запросов от inline-кнопок.
func (r *router) handleCallback(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	callbackData := update.CallbackQuery.Data

	// У callback-запросов верхнеуровневый update.Message почти всегда nil -
	// брать chatID оттуда нельзя (nil-pointer panic). В приватном чате
	// ChatID пользователя совпадает с ID отправителя.
	chatID := update.CallbackQuery.From.ID

	// Глобальное правило «кнопка/выбор удаляется после ответа»: после того
	// как бот ответил на inline-нажатие, исходное сообщение с inline-
	// клавиатурой убирается из чата (не плодит «мусор» в истории).
	// Исключение - переключение вкладок хаба (section_analysis/health/
	// service): эти хендлеры РЕДАКТИРУЮТ то же сообщение на месте
	// (renderHub → editHubPair), поэтому удалять его нельзя - иначе
	// исчезнет сам ответ.
	if !isHubTabSwitch(callbackData) {
		r.deleteCallbackMessageAfterReply(ctx, b, update)
	}

	log.Printf(locales.LogRouterCallback, dashboard.MaskID(chatID), callbackData)

	// Ловим панику в обработчике: tgbot иногда «глотает» панику, и тогда
	// сообщение не отправляется, а в логах - тишина. Логируем явно.
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("🔥 PANIC in handleCallback (chatID=%s, data=%q): %v", dashboard.MaskID(chatID), callbackData, rec)
		}
	}()

	// Онбординг (callback'и onboarding_*): обрабатываем до прочих, чтобы
	// не конфликтовали с премиум/хабами и не доходили до спиннера в конце.
	if strings.HasPrefix(callbackData, "onboarding_") {
		if r.handleOnboarding(ctx, b, chatID, callbackData, update) {
			return
		}
	}

	// Premium confirm (симуляция оплаты → активация Premium).
	// Проверяем ДО «premium_», иначе данные вида
	// «premium_confirm_<tariffID>» попадут в HandlePremiumCallback и
	// после отрезания префикса «premium_» дадут «confirm_<tariffID>».
	if strings.HasPrefix(callbackData, "premium_confirm_") {
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "premium_confirm")
		// Исходное сообщение со ссылкой на оплату убирается общим правилом
		// «кнопка/выбор удаляется после ответа» (см. начало handleCallback).
		menu.HandlePremiumConfirm(r.stateManager, r.paymentService, r.webAppURL, r.dashboardURL)(ctx, b, update, callbackData)
		log.Printf(locales.LogRouterCallbackDone, dashboard.MaskID(chatID), callbackData)
		return
	}

	// Premium change tariff (для уже активного Premium) - проверяем ДО
	// общего «premium_», иначе «premium_change» попал бы в выбор тарифа.
	if callbackData == "premium_change" {
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "premium_change")
		// Экран активного Premium убирается общим правилом «кнопка/выбор
		// удаляется после ответа» (см. начало handleCallback).
		if menu.HandleChangeTariff(r.stateManager, r.paymentService)(ctx, b, update, callbackData) {
			log.Printf(locales.LogRouterCallbackDone, dashboard.MaskID(chatID), callbackData)
		}
		return
	}

	// Premium callback (выбор тарифа → экран оплаты)
	if strings.HasPrefix(callbackData, "premium_") {
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "premium")
		// Исходное сообщение со списком тарифов убирается общим правилом
		// «кнопка/выбор удаляется после ответа» (см. начало handleCallback).
		menu.HandlePremiumCallback(r.stateManager, r.paymentService)(ctx, b, update, callbackData)
		log.Printf(locales.LogRouterCallbackDone, dashboard.MaskID(chatID), callbackData)
		return
	}

	// «Назад» из любого раздела/флоу (блок-хаб или сообщение под-действия
	// вроде Сводки/Мониторинга). Удаляем само сообщение с inline-кнопкой, а
	// дальше - иерархический возврат (на уровень выше: в хаб раздела, либо из
	// хаба - в Главное меню). Поведение совпадает с reply-кнопкой «⬅️ Назад»
	// (handleBack) - единый UX на всех этапах. "test_notify_back" - «Назад»
	// из под-меню теста уведомлений (возврат в хаб «Сервис»).
	if callbackData == "hub_back" || callbackData == "msg_back" || callbackData == "test_notify_back" {
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, callbackData)

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

	// «← Назад» из раздела-хаба (inline-кнопка блока меню хаба): удаляем
	// само сообщение и возвращаем пользователя в Главное меню.
	if callbackData == "back_to_main" {
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "back_to_main")
		if mm := update.CallbackQuery.Message; mm.Message != nil {
			helpers.DeleteMessage(ctx, b, chatID, mm.Message.ID)
		}
		r.stateManager.SetState(chatID, states.StateIdle)
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
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
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_analysis")
		r.renderHub(ctx, b, chatID, "analysis")
	case "section_health":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_health")
		r.renderHub(ctx, b, chatID, "health")
	case "section_service":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_service")
		r.renderHub(ctx, b, chatID, "service")
	case "section_diag_regular":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_diag_regular")
		r.handleRegularAnalysis(ctx, b, chatID)
	case "section_diag_extended":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_diag_extended")
		r.handleExtendedAnalysis(ctx, b, chatID)
	case "section_bioscan_basic":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_bioscan_basic")
		r.handleBioscanBasicStart(ctx, b, chatID)
	case "section_bioscan_extended":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_bioscan_extended")
		r.handleBioscanExtendedStart(ctx, b, chatID)
	case "section_diag_regular_demo":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_diag_regular_demo")
		r.handleRegularAnalysisDemo(ctx, b, chatID)
	case "section_diag_extended_demo":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_diag_extended_demo")
		r.handleExtendedAnalysisDemo(ctx, b, chatID)
	case "section_bioscan_basic_demo":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_bioscan_basic_demo")
		r.handleBioscanBasicDemo(ctx, b, chatID)
	case "section_bioscan_extended_demo":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_bioscan_extended_demo")
		r.handleBioscanExtendedDemo(ctx, b, chatID)
	case "section_health_summary":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_health_summary")
		r.handleDashboard(ctx, b, chatID, false)
	case "section_health_summary_demo":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_health_summary_demo")
		r.handleDashboard(ctx, b, chatID, true)
	case "section_consult_start":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_consult_start")
		r.handleConsultationStart(ctx, b, chatID)
	case "section_feedback_start":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_feedback_start")
		r.handleFeedbackStart(ctx, b, chatID)
	case "section_about":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_about")
		// Выбрано под-действие - убираем блок-хаб.
		r.deleteHubBlock(ctx, b, chatID)
		r.setCurrentSection(chatID, "service")
		menu.AboutHandler()(ctx, b, update)

	// Под-меню теста уведомлений (раздел «Сервис» → 🧪 Тест уведомлений,
	// только в development): вывод меню и планирование тестовых
	// уведомлений через 10 секунд (подписка: за 7/3/1/0 дней; анализы:
	// проверка или реальная отправка по отклонениям).
	case "section_test_notify":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "section_test_notify")
		r.handleTestNotifyMenu(ctx, b, chatID)
	case "test_sub_7d":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_sub_7d")
		r.handleTestNotifyAction(ctx, b, chatID, "sub_7d")
	case "test_sub_3d":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_sub_3d")
		r.handleTestNotifyAction(ctx, b, chatID, "sub_3d")
	case "test_sub_1d":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_sub_1d")
		r.handleTestNotifyAction(ctx, b, chatID, "sub_1d")
	case "test_sub_today":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_sub_today")
		r.handleTestNotifyAction(ctx, b, chatID, "sub_today")
	case "test_analytics_check":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_analytics_check")
		r.handleTestNotifyAction(ctx, b, chatID, "analytics_check")
	case "test_analytics_send":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "test_analytics_send")
		r.handleTestNotifyAction(ctx, b, chatID, "analytics_send")

	// Подтверждение загрузки файлов (inline-кнопки «Обработать/Отмена»).
	case "upload_process":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "upload_process")
		upload.StartAnalysis(ctx, b, r.stateManager, r.analysisService, r.reportRenderer, r.pdfConverter, r.uploadDir, r.stickerID, chatID, r.appStorage, r.monitorRepo, r.webAppURL)
	case "upload_cancel":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "upload_cancel")
		upload.CancelUpload(ctx, b, r.stateManager, chatID)

	// Подтверждение/перезапуск Bioscan (inline-кнопки на экране фото).
	case "bioscan_confirm":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "bioscan_confirm")
		bioscan.ProcessBioscanWithPhotos(ctx, b, r.stateManager, r.analysisService, r.pdfConverter, r.uploadDir, r.stickerID, chatID, r.appStorage, r.monitorRepo, r.webAppURL)
	case "bioscan_restart":
		log.Printf(locales.LogRouterCallbackDispatch, dashboard.MaskID(chatID), callbackData, "bioscan_restart")
		bioscan.StartBioscanFlow(ctx, b, r.stateManager, chatID)
	}

	// Answer callback query
	_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
}
