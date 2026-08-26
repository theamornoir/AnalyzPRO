package router

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/notifications"
)

// hubMessageKey - ключ в user-data для message_id «блока-хаба» (раздел
// Анализы/Здоровье/Сервис). Блок редактируется на месте (editMessage) при
// переключении разделов, чтобы не плодить сообщения.
const hubMessageKey = "hub_message_id"

// navLevelKey - уровень навигации в едином сообщении: "main" (главное меню),
// "hub" (раздел-хаб), "action" (экран под-действия/инструкция флоу). Нужен
// обработчику «Назад», чтобы понимать, куда возвращаться (в главное меню
// или в хаб раздела), не полагаясь на тип сообщения.
const navLevelKey = "nav_level"

func (r *router) navLevel(chatID int64) string {
	return r.stateManager.GetUserData(chatID, navLevelKey)
}

func (r *router) setNavLevel(chatID int64, level string) {
	r.stateManager.SetUserData(chatID, navLevelKey, level)
}

// hubSection описывает содержимое одного раздела-хаба.
type hubSection struct {
	text    string
	actions models.InlineKeyboardMarkup
}

// hubSections возвращает содержимое каждого раздела-хаба по его коду.
func hubSections() map[string]hubSection {
	return map[string]hubSection{
		"analysis": {text: locales.MsgAnalysisHubIntro, actions: keyboards.AnalysisHubMenu()},
		"health":   {text: locales.MsgHealthHubIntro, actions: keyboards.HealthHubMenu()},
		"service":  {text: locales.MsgServiceHubIntro, actions: keyboards.ServiceHubMenu()},
	}
}

// renderHub - показывает/переключает «хаб» раздела ОДНИМ сообщением
// (main_menu_msg_id). Telegram не позволяет совместить inline-кнопки действий
// и Reply-клавиатуру [Назад] в одном сообщении, поэтому хаб - это единое
// сообщение с inline-кнопками под-действий и inline-кнопкой «Назад».
//
// Если у пользователя уже открыто это сообщение (сохранено main_menu_msg_id),
// раздел перерисовывается прямо в нём (editMessage на месте), иначе
// отправляется новое. Результаты под-действий приходят отдельными сообщениями.
func (r *router) renderHub(ctx context.Context, b *tgbot.Bot, chatID int64, section string) bool {
	sections := hubSections()
	sec, ok := sections[section]
	if !ok {
		section = "analysis"
		sec = sections[section]
	}

	r.setCurrentSection(chatID, section)

	navID := r.navMsgID(chatID)
	if navID > 0 && r.editHubPair(ctx, b, chatID, navID, sec) {
		r.setNavLevel(chatID, "hub")
		return true
	}

	newMsgID, err := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        sec.text + "\n\n👇 Выберите действие:",
		ReplyMarkup: sec.actions,
		ParseMode:   "Markdown",
	})
	if err == nil && newMsgID > 0 {
		r.setNavMsgID(chatID, newMsgID)
		r.setNavLevel(chatID, "hub")
	}
	return true
}

// editHubPair пытается перерисовать существующий хаб на месте: единое
// сообщение. Возвращает true, если редактирование удалось.
func (r *router) editHubPair(ctx context.Context, b *tgbot.Bot, chatID int64, navID int, sec hubSection) bool {
	_, err := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   navID,
		Text:        sec.text + "\n\n👇 Выберите действие:",
		ReplyMarkup: sec.actions,
		ParseMode:   "Markdown",
	})
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		log.Printf("[HUB] не удалось отредактировать хаб msgID=%d chatID=%d: %v", navID, chatID, err)
		return false
	}
	return true
}

// deleteHubBlock - удаляет текущее навигационное сообщение (main_menu_msg_id)
// и сбрасывает сохранённый id. Используется, когда пользователь выбирает
// под-действие, запускающее отдельный поток (биоскан/опросник), или при
// удалении аккаунта - в чате не должно висеть устаревшее меню раздела.
func (r *router) deleteHubBlock(ctx context.Context, b *tgbot.Bot, chatID int64) {
	if id := r.navMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setNavMsgID(chatID, 0)
	}
}

// navMsgID / setNavMsgID - чтение/запись message_id единственного
// навигационного сообщения (главное меню / хаб / под-действие / Premium).
// Совпадает с main_menu_msg_id - это то же сообщение, перерисовываемое
// «на месте» при переходах.
func (r *router) navMsgID(chatID int64) int {
	return r.mainMenuMsgID(chatID)
}

func (r *router) setNavMsgID(chatID int64, msgID int) {
	r.setMainMenuMsgID(chatID, msgID)
}

// mainMenuMsgID / setMainMenuMsgID - чтение/запись message_id закреплённого
// сообщения главного меню (оно же navMsgID - единое навигационное сообщение).
func (r *router) mainMenuMsgID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, helpers.MainMenuMsgKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setMainMenuMsgID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, helpers.MainMenuMsgKey, strconv.Itoa(msgID))
}

// deleteMainMenuMessage - удаляет закреплённое сообщение главного меню и
// сбрасывает его id. Безопасно при отсутствии.
func (r *router) deleteMainMenuMessage(ctx context.Context, b *tgbot.Bot, chatID int64) {
	if id := r.mainMenuMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setMainMenuMsgID(chatID, 0)
	}
}

// showMainMenuMessage - показывает/перерисовывает ПЕРСИСТЕНТНОЕ сообщение
// главного меню (inline-кнопки) в едином навигационном сообщении. Если
// сообщение уже открыто - редактирует его на месте (editMessage), иначе
// отправляет новое. Переходы главное меню <-> хаб <-> под-действие не плодят
// новые сообщения в чате.
func (r *router) showMainMenuMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	navID := r.navMsgID(chatID)
	if navID > 0 {
		_, err := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   navID,
			Text:        text,
			ReplyMarkup: keyboards.MainMenuInline(),
			ParseMode:   "Markdown",
		})
		if err == nil {
			r.setNavLevel(chatID, "main")
			return
		}
		// edit не удался (сообщение удалено пользователем) - шлём новое.
	}
	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: keyboards.MainMenuInline(),
		ParseMode:   "Markdown",
	})
	if err == nil && msg != nil {
		r.setNavMsgID(chatID, msg.ID)
		r.setNavLevel(chatID, "main")
	}
}

// editNavMessage - перерисовывает единое навигационное сообщение «на месте»:
// текст + inline-клавиатура. Используется для экранов под-действий, чтобы не
// плодить сообщения в чате.
func (r *router) editNavMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, keyboard models.InlineKeyboardMarkup) {
	// Экран под-действия (не хаб и не главное меню) - отмечаем уровень
	// "action", чтобы «Назад» возвращал именно в хаб раздела, а не сразу в
	// Главное меню (иначе навигация «прыгала» бы через уровень).
	r.setNavLevel(chatID, "action")
	navID := r.navMsgID(chatID)
	if navID > 0 {
		_, err := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   navID,
			Text:        text,
			ReplyMarkup: keyboard,
			ParseMode:   "Markdown",
		})
		if err == nil || strings.Contains(err.Error(), "message is not modified") {
			return
		}
	}
	msg, err := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: keyboard,
		ParseMode:   "Markdown",
	})
	if err == nil && msg > 0 {
		r.setNavMsgID(chatID, msg)
	}
}

// handleFeedbackStart - запускает режим ввода отзыва/предложения.
func (r *router) handleFeedbackStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[FEEDBACK] открытие раздела для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingFeedback)
	r.setCurrentSection(chatID, "service")

	r.editNavMessage(ctx, b, chatID, locales.MsgFeedbackIntro, keyboards.BackCancelInline())
	return true
}

// handleFeedbackMessage - пересылает сообщение пользователя разработчику и
// подтверждает доставку.
func (r *router) handleFeedbackMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[FEEDBACK] ввод сообщения от chatID=%d", chatID)

	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[FEEDBACK] отмена ввода chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	if r.adminChatID == 0 {
		log.Printf("[FEEDBACK] adminChatID не задан, доставка невозможна chatID=%d", chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.editNavMessage(ctx, b, chatID, locales.MsgFeedbackUnavailable, keyboards.BackInline())
		return true
	}

	from := update.Message.From
	fullName := "-"
	username := "-"
	if from != nil {
		name := strings.TrimSpace(from.FirstName + " " + from.LastName)
		if name != "" {
			fullName = name
		}
		if from.Username != "" {
			username = from.Username
		}
	}

	_, metaErr := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    r.adminChatID,
		Text:      fmt.Sprintf(locales.MsgFeedbackMeta, fullName, chatID, username),
		ParseMode: "Markdown",
	})

	if update.Message != nil {
		if _, fErr := b.ForwardMessage(ctx, &tgbot.ForwardMessageParams{
			ChatID:     r.adminChatID,
			FromChatID: chatID,
			MessageID:  update.Message.ID,
		}); fErr != nil {
			log.Printf("[FEEDBACK] ошибка пересылки chatID=%d: %v", chatID, fErr)
		}
	}

	if metaErr != nil {
		log.Printf("[FEEDBACK] ошибка отправки админу chatID=%d: %v", chatID, metaErr)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.editNavMessage(ctx, b, chatID, locales.MsgFeedbackSendError, keyboards.BackInline())
		return true
	}

	r.stateManager.SetState(chatID, states.StateIdle)
	log.Printf("[FEEDBACK] отзыв доставлен админу от chatID=%d", chatID)
	r.editNavMessage(ctx, b, chatID, locales.MsgFeedbackConfirmed, keyboards.MainMenuInline())
	return true
}

// ============================================================================
// Удаление аккаунта (раздел «Сервис»)
// ============================================================================

// handleDeleteAccountStart - открывает экран подтверждения удаления аккаунта.
func (r *router) handleDeleteAccountStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[DELETE-ACCOUNT] подтверждение для chatID=%d", chatID)

	r.setCurrentSection(chatID, "service")

	r.editNavMessage(ctx, b, chatID, locales.MsgDeleteAccountConfirm, keyboards.DeleteAccountMenu())
	return true
}

// handleDeleteAccountConfirm - реально удаляет ВСЕ данные пользователя.
func (r *router) handleDeleteAccountConfirm(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[DELETE-ACCOUNT] удаление всех данных для chatID=%d", chatID)

	r.deleteHubBlock(ctx, b, chatID)

	if r.appStorage != nil {
		if err := r.appStorage.DeleteAccount(ctx, chatID); err != nil {
			log.Printf("[DELETE-ACCOUNT] ошибка удаления профиля chatID=%d: %v", chatID, err)
		}
	}
	if r.monitorRepo != nil {
		if err := r.monitorRepo.DeleteByUser(ctx, chatID); err != nil {
			log.Printf("[DELETE-ACCOUNT] ошибка удаления мониторинга chatID=%d: %v", chatID, err)
		}
	}
	if svc := r.getNotificationsSvc(); svc != nil {
		if err := svc.DeleteUser(ctx, chatID); err != nil {
			log.Printf("[DELETE-ACCOUNT] ошибка удаления уведомлений chatID=%d: %v", chatID, err)
		}
	}
	if r.agreementStorage != nil {
		r.agreementStorage.Reset(chatID)
	}
	menu.ClearPremiumScreen(ctx, b, r.stateManager, chatID)
	r.stateManager.Reset(chatID)

	// Подтверждение удаления - без reply-клавиатуры: после сброса
	// пользователь «как новый» и обязан нажать /start (онбординг), поэтому
	// меню показывать нельзя (оно сломано без соглашения/онбординга).
	// Навигация вообще только inline - reply-меню не используем. Снимаем
	// возможную «висящую» reply-клавиатуру (могла остаться от потока
	// ввода до удаления), чтобы после /start пользователь видел ТОЛЬКО
	// инлайн-онбординг, без дублирующей reply-клавиатуры.
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgAccountDeleted,
		ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		ParseMode:   "Markdown",
	})
	return true
}

// handleDeleteAccountCancel - отмена удаления аккаунта: возврат в хаб Сервис.
func (r *router) handleDeleteAccountCancel(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[DELETE-ACCOUNT] отмена для chatID=%d", chatID)
	r.renderHub(ctx, b, chatID, "service")
	return true
}

// handleRegularAnalysis - запускает обычный анализ.
func (r *router) handleRegularAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterRegular, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "regular")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "regular")
	r.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	r.setCurrentSection(chatID, "analysis")

	r.editNavMessage(ctx, b, chatID, locales.MsgRegularAnalysisIntro, keyboards.BackCancelInline())
	return true
}

// handleExtendedAnalysis - запускает расширенный анализ (с опросником).
func (r *router) handleExtendedAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterExtended, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	if !r.paymentService.IsUserPremium(chatID) {
		r.editNavMessage(ctx, b, chatID, locales.MsgExtendedAnalysisPremiumRequired, keyboards.BackInline())
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "extended")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")
	r.stateManager.SetState(chatID, states.StateWaitingName)
	r.setCurrentSection(chatID, "analysis")

	r.deleteHubBlock(ctx, b, chatID)
	userdata.NewUserDataCollector(r.stateManager).SendStep(ctx, b, chatID, states.StateWaitingName, locales.MsgExtendedAnalysisIntro)
	return true
}

// handleBioscanBasicStart - запускает БАЗОВЫЙ (бесплатный) Bioscan.
func (r *router) handleBioscanBasicStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[BIOSCAN] запуск БАЗОВОГО для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	r.setCurrentSection(chatID, "analysis")
	r.deleteHubBlock(ctx, b, chatID)
	bioscan.StartBioscanBasicFlow(ctx, b, r.stateManager, chatID)
	return true
}

// handleBioscanExtendedStart - запускает РАСШИРЕННЫЙ (Premium) Bioscan PRO.
func (r *router) handleBioscanExtendedStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[BIOSCAN] запуск РАСШИРЕННОГО (PRO) для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	r.setCurrentSection(chatID, "analysis")

	if !r.paymentService.IsUserPremium(chatID) {
		// Премиум-заглушка: перерисовываем единое навигационное сообщение
		// «на месте» (edit-in-place). «Назад» (hub_back) возвращает в хаб
		// «Анализы». Не шлём отдельное сообщение с reply-клавиатурой - иначе
		// нарушается единообразие навигации с прочими экранами-заглушками.
		r.editNavMessage(ctx, b, chatID, locales.MsgBioscanExtendedPremiumRequired, keyboards.BackInline())
		return true
	}

	// Запуск опросника Bioscan PRO: интро редактирует навигационное сообщение
	// «на месте» (как у Обычного анализа / Консультации), чтобы «Назад»
	// возвращал в хаб «Анализы», а не плодил новые сообщения. Дальше опросник
	// шлёт вопросы отдельными сообщениями (это ввод данных, не навигация - по
	// согласованному дизайну они остаются новыми сообщениями).
	r.stateManager.SetState(chatID, states.StateWaitingBioscanName)
	log.Printf(locales.LogRouterForceBioscan, chatID)

	r.editNavMessage(ctx, b, chatID, locales.MsgBioscanIntro, keyboards.BackInline())
	return true
}

// handleDashboard - открывает веб-дашборд.
func (r *router) handleDashboard(ctx context.Context, b *tgbot.Bot, chatID int64, demo bool) bool {
	log.Printf(locales.LogRouterDashboard, chatID)

	r.setCurrentSection(chatID, "health")

	isPremium := r.paymentService.IsUserPremium(chatID)
	log.Printf(locales.LogDashboardPremiumCheck, chatID, isPremium)

	webAppTarget := keyboards.WithWebAppVersion(r.webAppURL)
	if demo {
		if strings.Contains(webAppTarget, "?") {
			webAppTarget += "&demo=1"
		} else {
			webAppTarget += "?demo=1"
		}
	}
	if webAppTarget == "" {
		r.editNavMessage(ctx, b, chatID,
			"⚠️ URL дашборда не настроен. Задайте WEBAPP_URL или запустите `make mini`.",
			keyboards.BackInline())
		return true
	}

	text := locales.MsgHealthSummaryIntro + "\n\n"
	if !isPremium {
		text += "📝 Профиль можно заполнить бесплатно - после этого Мой профиль оживёт. " +
			"Полный доступ к показателям крови и динамике - по Premium-подписке.\n\n"
	}

	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "Открыть", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		},
		{
			{Text: locales.BtnBack, CallbackData: "msg_back"},
		},
	}

	r.editNavMessage(ctx, b, chatID, text, models.InlineKeyboardMarkup{InlineKeyboard: rows})
	return true
}

// ============================================================================
// Быстрая консультация (с ИИ)
// ============================================================================

const freeConsultationLimit = 3

const consultUserKey = "ai_consult_count"

func (r *router) consultCount(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, consultUserKey))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (r *router) consultSetCount(chatID int64, n int) {
	r.stateManager.SetUserData(chatID, consultUserKey, strconv.Itoa(n))
}

func consultationTextPrompt(question string) string {
	return "Вопрос пользователя о здоровье: " + strings.TrimSpace(question) +
		"\n\nТвоя задача - дать медицинскую консультацию: ответь на вопрос, " +
		"объясни возможные причины, дай практические рекомендации по облегчению " +
		"состояния. В конце ОБЯЗАТЕЛЬНО напомни, что это не является диагнозом, " +
		"а лишь информационные рекомендации; при ухудшении состояния нужно " +
		"обратиться к врачу."
}

func consultationImageContext(question string) string {
	base := "Это фото травмы или проблемной зоны пользователя. Пожалуйста, дай " +
		"медицинскую консультацию по фото: опиши, что видишь, возможные причины, " +
		"рекомендации по облегчению состояния и напомни, что это не заменяет " +
		"очный визит к врачу."
	question = strings.TrimSpace(question)
	if question != "" {
		base += "\n\nВопрос пользователя к фото: " + question
	}
	return base
}

// handleConsultationStart - запускает режим консультации.
func (r *router) handleConsultationStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[CONSULT] запуск для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
		})
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		r.editNavMessage(ctx, b, chatID, locales.MsgConsultationPremiumRequired, keyboards.BackInline())
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingConsultation)
	r.setCurrentSection(chatID, "health")
	// Новая сессия консультации - сбрасываем накопленные id ответов ИИ
	// (защита на случай повторного входа в консультацию без /start).
	r.consultClearResultMsgIDs(chatID)

	r.editNavMessage(ctx, b, chatID, locales.MsgConsultationStart, keyboards.BackCancelInline())
	return true
}

// handleConsultationMessage - обрабатывает сообщение пользователя в режиме
// StateWaitingConsultation.
func (r *router) handleConsultationMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[CONSULT] ввод сообщения от chatID=%d", chatID)

	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[CONSULT] отмена chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	hasPhoto := update.Message != nil && len(update.Message.Photo) > 0

	if strings.TrimSpace(text) == "" && !hasPhoto {
		r.editNavMessage(ctx, b, chatID, locales.MsgConsultationEmpty, keyboards.BackCancelInline())
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)

	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		r.stateManager.SetState(chatID, states.StateIdle)
		r.editNavMessage(ctx, b, chatID, locales.MsgConsultationPremiumRequired, keyboards.BackInline())
		return true
	}

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, r.stickerID, locales.ConsultationLoadingSteps)

	var (
		result string
		err    error
	)

	if hasPhoto {
		photos := update.Message.Photo
		largest := photos[len(photos)-1]
		data, mimeType, dlErr := helpers.DownloadFileByID(ctx, b, largest.FileID)
		if dlErr != nil {
			log.Printf("[CONSULT] ошибка загрузки фото chatID=%d: %v", chatID, dlErr)
			r.stateManager.SetState(chatID, states.StateIdle)
			r.editNavMessage(ctx, b, chatID, locales.MsgConsultationError, keyboards.BackInline())
			helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
			return true
		}
		result, err = r.analysisService.HandleConsultationImage(ctx, data, mimeType, consultationImageContext(text))
	} else {
		result, err = r.analysisService.HandleAnalysis(ctx, consultationTextPrompt(text))
	}

	if err != nil || strings.TrimSpace(result) == "" {
		log.Printf("[CONSULT] ошибка генерации chatID=%d: %v", chatID, err)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.editNavMessage(ctx, b, chatID, locales.MsgConsultationError, keyboards.BackInline())
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		return true
	}

	if !isPremium {
		r.consultSetCount(chatID, r.consultCount(chatID)+1)
	}
	// Переходим в «финиш»-состояние: ответ показан, но из флоу можно выйти
	// ТОЛЬКО по кнопке «Закончить консультацию» (см. handleConsultationFinish).
	r.stateManager.SetState(chatID, states.StateWaitingConsultationFinish)

	full := locales.MsgConsultationResultIntro + result
	if !isPremium {
		freeLeft := freeConsultationLimit - r.consultCount(chatID)
		if freeLeft < 0 {
			freeLeft = 0
		}
		if freeLeft > 0 {
			full += fmt.Sprintf(locales.MsgConsultationQuotaLeft, freeLeft)
		}
	}

	helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)

	// Ответ ИИ - отдельным(и) сообщением(и) БЕЗ клавиатуры, чтобы результат
	// не «прятался» под inline-меню и не плодил висящую навигацию. id
	// запоминаем (sendConsultationResult), чтобы убрать ответ целиком при
	// завершении консультации (иначе «мусорный» ответ ИИ остаётся в чате).
	r.sendConsultationResult(ctx, b, chatID, full)

	// ОТДЕЛЬНОЕ сообщение с reply-кнопкой «Закончить консультацию» (+ «Задать
	// ещё вопрос»). Внизу БОЛЬШЕ НЕТ главного меню - только этот флоу, поэтому
	// пользователь не может случайно «выйти» в раздел до явного «Закончить».
	finishMsg, ferr := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgConsultationFinishHint,
		ReplyMarkup: keyboards.ConsultFinishMenu(),
	})
	if ferr == nil && finishMsg != nil {
		r.setConsultFinishMsgID(chatID, finishMsg.ID)
	}

	// Нейтрализуем навигационное сообщение (инлайн «Назад/Отмена»
	// консультации): удаляем его, чтобы из флоу нельзя было выйти иначе,
	// кроме как по «Закончить консультацию» (его перерисовка/удаление
	// происходит в finishConsultation / continueConsultation).
	r.deleteMainMenuMessage(ctx, b, chatID)
	return true
}

// consultFinishMsgKey - ключ в user-data для message_id сообщения с
// reply-кнопкой «Закончить консультацию» (финиш-состояние). Хранится в
// user-data (а не в выделенном map, как Premium), т.к. сбрасывается вместе с
// состоянием через stateManager.Reset - это и нужно.
const consultFinishMsgKey = "consult_finish_msg_id"

func (r *router) consultFinishMsgID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, consultFinishMsgKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setConsultFinishMsgID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, consultFinishMsgKey, strconv.Itoa(msgID))
}

// consultResultMsgKey - ключ в user-data для message_id ВСЕХ сообщений ответа
// ИИ в рамках одной консультации (результат из-за лимита Telegram 4096 байт
// может быть разбит на несколько сообщений). Хранится через запятую. Нужно,
// чтобы при завершении консультации («Закончить консультацию») убрать из
// чата ВЕСЬ ответ, а не только подсказку-кнопку - иначе «мусорный» ответ ИИ
// остаётся висеть после возврата в главное меню (жалоба пользователя).
const consultResultMsgKey = "consult_result_msg_ids"

func (r *router) consultResultMsgIDs(chatID int64) []int {
	raw := r.stateManager.GetUserData(chatID, consultResultMsgKey)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ids []int
	for _, p := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
			ids = append(ids, n)
		}
	}
	return ids
}

func (r *router) consultAppendResultMsgID(chatID int64, msgID int) {
	if msgID <= 0 {
		return
	}
	ids := r.consultResultMsgIDs(chatID)
	ids = append(ids, msgID)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	r.stateManager.SetUserData(chatID, consultResultMsgKey, strings.Join(parts, ","))
}

func (r *router) consultClearResultMsgIDs(chatID int64) {
	r.stateManager.SetUserData(chatID, consultResultMsgKey, "")
}

// sendConsultationResult - отправляет результат ИИ консультации (возможно,
// несколькими сообщениями из-за лимита Telegram) БЕЗ клавиатуры и
// запоминает их message_id, чтобы при завершении консультации (finishConsultation)
// убрать ответ из чата целиком. Аналог helpers.SendLongMessagePlain, но с
// трекингом id сообщений.
func (r *router) sendConsultationResult(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	for _, chunk := range helpers.SplitLongMessage(text, helpers.MaxMessageChunk) {
		msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: chunk})
		if err == nil && msg != nil {
			r.consultAppendResultMsgID(chatID, msg.ID)
		}
	}
}

// handleConsultationFinish - обработка сообщений в режиме
// StateWaitingConsultationFinish. Доступны ТОЛЬКО две reply-кнопки:
// «Закончить консультацию» (выход в главное меню) и «Задать ещё вопрос»
// (продолжить диалог). Любые прочие сообщения/кнопки игнорируются - иначе
// меню «прыгало» бы и дублировалось.
func (r *router) handleConsultationFinish(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	switch strings.TrimSpace(text) {
	case locales.BtnConsultFinish:
		r.finishConsultation(ctx, b, chatID, update)
	case locales.BtnConsultAgain:
		r.continueConsultation(ctx, b, chatID, update)
	default:
		// Команды (/start, /resetme, /promo и т.п.) пропускаем дальше по
		// цепочке обработки, чтобы бот оставался отзывчивым; всё иное
		// (свободный текст, фото) игнорируем, пока не нажата «Закончить».
		if strings.HasPrefix(strings.TrimSpace(text), "/") {
			return false
		}
		return true
	}
	return true
}

// finishConsultation - завершение флоу консультации по кнопке «Закончить
// консультацию»: сброс состояния, удаление сообщения с кнопкой «Закончить» и
// показ обычного главного меню (инлайн; reply-клавиатура флоу снимается).
func (r *router) finishConsultation(ctx context.Context, b *tgbot.Bot, chatID int64, update *models.Update) {
	r.stateManager.SetState(chatID, states.StateIdle)

	// Удаляем ВСЕ сообщения ответа ИИ (возможно несколько, если результат
	// длинный и был разбит на части из-за лимита Telegram), чтобы после
	// завершения консультации в чате не висел «мусорный» ответ - пользователь
	// возвращается к чистому главному меню (жалоба: ответ оставался после
	// «Закончить консультацию»).
	for _, id := range r.consultResultMsgIDs(chatID) {
		helpers.DeleteMessage(ctx, b, chatID, id)
	}
	r.consultClearResultMsgIDs(chatID)

	// Удаляем сообщение, несшее reply-кнопку «Закончить» (+ «Задать ещё
	// вопрос»), чтобы убрать нижнюю клавиатуру флоу.
	if id := r.consultFinishMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setConsultFinishMsgID(chatID, 0)
	}
	// И «тап» пользователя по кнопке (правило «кнопка/выбор удаляется после
	// ответа») - чтобы в истории не висел «✅ Закончить консультацию».
	if update != nil && update.Message != nil && update.Message.ID != 0 {
		helpers.DeleteAfterReply(ctx, b, chatID, update.Message.ID)
	}

	// Обычное главное меню (инлайн). Reply-клавиатура флоу снята удалением
	// несущих её сообщений выше, поэтому внизу не остаётся «Закончить».
	r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
}

// continueConsultation - возврат в режим ожидания вопроса после нажатия
// «Задать ещё вопрос» из финиш-состояния: убирает reply-кнопку «Закончить» и
// показывает инлайн-подсказку ввода вопроса (без главного меню снизу).
func (r *router) continueConsultation(ctx context.Context, b *tgbot.Bot, chatID int64, update *models.Update) {
	// Убираем reply-кнопку «Закончить» (сообщение + «тап» пользователя).
	if id := r.consultFinishMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setConsultFinishMsgID(chatID, 0)
	}
	if update != nil && update.Message != nil && update.Message.ID != 0 {
		helpers.DeleteAfterReply(ctx, b, chatID, update.Message.ID)
	}
	r.enterConsultationWaiting(ctx, b, chatID)
}

// enterConsultationWaiting - перевод пользователя в режим ожидания вопроса
// консультации (StateWaitingConsultation) и показ инлайн-подсказки ввода.
// Главное меню снизу при этом НЕ показывается (только инлайн «Назад/Отмена»).
func (r *router) enterConsultationWaiting(ctx context.Context, b *tgbot.Bot, chatID int64) {
	isPremium := r.paymentService.IsUserPremium(chatID)
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		r.stateManager.SetState(chatID, states.StateIdle)
		r.editNavMessage(ctx, b, chatID, locales.MsgConsultationPremiumRequired, keyboards.BackInline())
		return
	}
	r.stateManager.SetState(chatID, states.StateWaitingConsultation)
	r.setCurrentSection(chatID, "health")
	r.editNavMessage(ctx, b, chatID, locales.MsgConsultationStart, keyboards.BackCancelInline())
}

// ============================================================================
// Тест уведомлений (раздел «Сервис» → 🧪 Тест уведомлений)
// ============================================================================

const testNotifyDelay = 10 * time.Second

func (r *router) handleTestNotifyMenu(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[TEST-NOTIFY] открытие меню для chatID=%d", chatID)

	r.setCurrentSection(chatID, "service")

	r.editNavMessage(ctx, b, chatID, locales.MsgTestNotifyIntro, keyboards.TestNotifyMenu())
	return true
}

func (r *router) handleTestNotifyAction(ctx context.Context, b *tgbot.Bot, chatID int64, kind string) bool {
	log.Printf("[TEST-NOTIFY] планирование уведомления kind=%s для chatID=%d", kind, chatID)

	go func() {
		select {
		case <-time.After(testNotifyDelay):
			r.runTestNotification(context.Background(), b, chatID, kind)
		}
	}()

	r.editNavMessage(ctx, b, chatID, locales.MsgTestNotifyScheduled, keyboards.TestNotifyMenu())
	return true
}

func (r *router) runTestNotification(ctx context.Context, b *tgbot.Bot, chatID int64, kind string) {
	svc := r.getNotificationsSvc()
	if svc == nil {
		r.editNavMessage(ctx, b, chatID, locales.MsgNotifTestInvalid, keyboards.BackInline())
		return
	}
	switch kind {
	case "sub_7d", "sub_3d", "sub_1d", "sub_today":
		var days int
		switch kind {
		case "sub_7d":
			days = 7
		case "sub_3d":
			days = 3
		case "sub_1d":
			days = 1
		case "sub_today":
			days = 0
		}
		if _, err := svc.SendSubscriptionTest(ctx, chatID, days); err != nil {
			r.editNavMessage(ctx, b, chatID, locales.MsgNotifTestInvalid, keyboards.BackInline())
			return
		}
	case "analytics_check":
		findings, err := svc.RunAnalyticsDryRun(ctx, chatID)
		if errors.Is(err, notifications.ErrNoAnalysisData) {
			r.editNavMessage(ctx, b, chatID, locales.MsgNotifAnalyticsNoData, keyboards.BackInline())
			return
		}
		if err != nil {
			r.editNavMessage(ctx, b, chatID, locales.MsgNotifTestInvalid, keyboards.BackInline())
			return
		}
		r.editNavMessage(ctx, b, chatID, svc.DryRunMessage(findings), keyboards.BackInline())
	case "analytics_send":
		n, err := svc.SendAnalyticsTest(ctx, chatID)
		if errors.Is(err, notifications.ErrNoAnalysisData) {
			r.editNavMessage(ctx, b, chatID, locales.MsgNotifAnalyticsNoData, keyboards.BackInline())
			return
		}
		if err != nil {
			r.editNavMessage(ctx, b, chatID, locales.MsgNotifTestInvalid, keyboards.BackInline())
			return
		}
		r.editNavMessage(ctx, b, chatID, fmt.Sprintf(locales.MsgNotifAnalyticsSent, n), keyboards.BackInline())
	default:
		r.editNavMessage(ctx, b, chatID, locales.MsgNotifTestInvalid, keyboards.BackInline())
	}
}
