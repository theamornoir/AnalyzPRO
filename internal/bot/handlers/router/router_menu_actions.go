package router

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// WebAppAssetsVersion — версия статических активов Mini App (Сводка здоровья
// и Мониторинг). Увеличивайте при изменении index.html/app.js/style.css,
// чтобы Telegram WebView перезапросил свежие файлы (иначе отдаёт
// закэшированную старую версию — отсюда «пустой/старый» дашборд после правок).
// Должна совпадать с ?v= в ссылках на активы в webapp_files/index.html.
const WebAppAssetsVersion = "v8"

// WithWebAppVersion добавляет ?v=<version> к URL Mini App, сбрасывая кэш
// Telegram WebView при обновлении активов. Пустой URL не трогает.
func WithWebAppVersion(u string) string {
	if u == "" {
		return u
	}
	if strings.Contains(u, "?") {
		return u + "&v=" + WebAppAssetsVersion
	}
	return u + "?v=" + WebAppAssetsVersion
}

// hubMessageKey — ключ в user-data, в котором хранится message_id текущего
// «блока-хаба» (раздел Анализы/Здоровье/Сервис). Блок редактируется на месте
// (editMessage) при переключении разделов, чтобы не плодить сообщения: один
// блок перерисовывается вкладками, а результаты под-действий приходят
// отдельными сообщениями.
const hubMessageKey = "hub_message_id"

// hubAnchorKey — ключ в user-data для message_id «якорного» сообщения раздела.
// Оно несёт внизу единую Reply-клавиатуру [Назад] (висит на всём протяжении
// раздела). Telegram не позволяет совместить inline-кнопки действий и эту
// Reply-клавиатуру в одном сообщении, поэтому «якорь» и блок — два сообщения.
const hubAnchorKey = "hub_anchor_id"

// lastMsgKey — ключ в user-data для message_id последнего «шагового»
// сообщения раздела/флоу. Используется обработчиком «Назад», чтобы удалить
// именно текущее сообщение раздела перед возвратом в главное меню.
const lastMsgKey = "last_msg_id"

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

// renderHub — показывает/переключает «блок-хаб» раздела ДВУМЯ сообщениями:
//  1. «якорь» — описание раздела + единая Reply-клавиатура [Назад] внизу;
//  2. «блок» — инлайн-кнопки под-действий раздела + подсказка
//     «👇 Выберите действие:». Telegram не позволяет совместить инлайн-кнопки
//     действий и Reply-клавиатуру [Назад] в одном сообщении, поэтому хаб —
//     два сообщения.
//
// Если у пользователя уже открыты оба сообщения хаба (сохранены hub_anchor_id
// и hub_message_id), раздел перерисовывается прямо в них (editMessage на
// месте), иначе отправляются новые. Результаты под-действий (анализ,
// консультация и т.п.) приходят отдельными сообщениями.
func (r *router) renderHub(ctx context.Context, b *tgbot.Bot, chatID int64, section string) bool {
	sections := hubSections()
	sec, ok := sections[section]
	if !ok {
		section = "analysis"
		sec = sections[section]
	}

	// Запоминаем текущий раздел для иерархического «Назад» (подшаг -> хаб).
	r.setCurrentSection(chatID, section)

	anchorID := r.hubAnchorID(chatID)
	msgID := r.hubMessageID(chatID)

	// Оба сообщения на месте — перерисовываем на месте (edit), чтобы не
	// плодить новые сообщения при переключении разделов.
	if anchorID > 0 && msgID > 0 && r.editHubPair(ctx, b, chatID, anchorID, msgID, sec) {
		return true
	}

	// Не удалось отредактировать (сообщения удалены пользователем и т.п.) —
	// чистим остатки и отправляем хаб заново.
	r.deleteHubBlock(ctx, b, chatID)

	// 1) Якорь: описание раздела + единая Reply-клавиатура [Назад].
	newAnchorID, anchorErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        sec.text,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	// 2) Блок: под-действия раздела + подсказка «👇 Выберите действие:».
	newMsgID, blockErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "👇 Выберите действие:",
		ReplyMarkup: sec.actions,
	})
	if anchorErr == nil && newAnchorID > 0 {
		r.setHubAnchorID(chatID, newAnchorID)
	}
	if blockErr == nil && newMsgID > 0 {
		r.setHubMessageID(chatID, newMsgID)
		r.setLastMsg(chatID, newMsgID)
	}
	return true
}

// editHubPair пытается перерисовать существующий хаб на месте: якорь
// (описание раздела) и блок (подсказка + инлайн-кнопки действий). Возвращает
// true, если редактирование удалось (в т.ч. при «message is not modified»).
func (r *router) editHubPair(ctx context.Context, b *tgbot.Bot, chatID int64, anchorID, msgID int, sec hubSection) bool {
	// Якорь: описание раздела.
	_, aErr := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: anchorID,
		Text:      sec.text,
		ParseMode: "Markdown",
	})
	if aErr != nil && !strings.Contains(aErr.Error(), "message is not modified") {
		log.Printf("[HUB] не удалось отредактировать якорь msgID=%d chatID=%d: %v", anchorID, chatID, aErr)
		return false
	}
	// Блок: подсказка + инлайн-кнопки действий.
	_, bErr := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   msgID,
		Text:        "👇 Выберите действие:",
		ReplyMarkup: sec.actions,
	})
	if bErr != nil && !strings.Contains(bErr.Error(), "message is not modified") {
		log.Printf("[HUB] не удалось отредактировать блок msgID=%d chatID=%d: %v", msgID, chatID, bErr)
		return false
	}
	log.Printf("[HUB] блок переключён на вкладку (anchor=%d block=%d chatID=%d)", anchorID, msgID, chatID)
	return true
}

// deleteHubBlock — удаляет текущий «блок-хаб» (раздел Анализы/Здоровье/
// Сервис) из чата вместе с «якорем» (Reply-клавиатура [Назад]) и сбрасывает
// сохранённые id. Используется, когда пользователь выбирает под-действие
// (анализ/консультация и т.п.) или нажимает «Назад» из раздела: в чате не
// должно висеть устаревшее меню раздела (иначе — «куча непонятно чего»).
// Безопасно, если блока/якоря нет.
func (r *router) deleteHubBlock(ctx context.Context, b *tgbot.Bot, chatID int64) {
	msgID := r.hubMessageID(chatID)
	anchorID := r.hubAnchorID(chatID)
	if msgID > 0 {
		helpers.DeleteMessage(ctx, b, chatID, msgID)
	}
	if anchorID > 0 {
		helpers.DeleteMessage(ctx, b, chatID, anchorID)
	}
	if msgID > 0 || anchorID > 0 {
		log.Printf("[HUB] блок удалён (msgID=%d anchorID=%d chatID=%d)", msgID, anchorID, chatID)
	}
	r.setHubAnchorID(chatID, 0)
	r.setHubMessageID(chatID, 0)
}

// hubMessageID / setHubMessageID — чтение/запись message_id текущего блока-хаба.
func (r *router) hubMessageID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, hubMessageKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setHubMessageID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, hubMessageKey, strconv.Itoa(msgID))
}

// hubAnchorID / setHubAnchorID — чтение/запись message_id «якоря» раздела
// (сообщение с Reply-клавиатурой [Назад]).
func (r *router) hubAnchorID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, hubAnchorKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setHubAnchorID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, hubAnchorKey, strconv.Itoa(msgID))
}

// lastMsgID / setLastMsg — чтение/запись message_id последнего «шагового»
// сообщения раздела/флоу (для удаления при нажатии «Назад»).
func (r *router) lastMsgID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, lastMsgKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setLastMsg(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, lastMsgKey, strconv.Itoa(msgID))
}

// handleFeedbackStart - запускает режим ввода отзыва/предложения: описывает
// раздел и переводит пользователя в StateWaitingFeedback, ожидая следующее
// сообщение (текст/фото/документ), которое будет переслано разработчику.
func (r *router) handleFeedbackStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[FEEDBACK] открытие раздела для chatID=%d", chatID)

	// Выбрано под-действие — убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingFeedback)
	r.setCurrentSection(chatID, "service")

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении ввода отзыва.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgFeedbackIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleFeedbackMessage - пересылает сообщение пользователя (текст/фото/
// документ) разработчику (adminChatID) и подтверждает доставку. Срабатывает
// при любом сообщении в режиме StateWaitingFeedback. Возвращает true, если
// сообщение обработано как отзыв.
func (r *router) handleFeedbackMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[FEEDBACK] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат — на уровень выше (хаб Сервис), а не в главное меню.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[FEEDBACK] отмена ввода chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	// Получатель не настроен — отзыв некуда доставлять.
	if r.adminChatID == 0 {
		log.Printf("[FEEDBACK] adminChatID не задан, доставка невозможна chatID=%d", chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgFeedbackUnavailable,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	from := update.Message.From
	fullName := "—"
	username := "—"
	if from != nil {
		name := strings.TrimSpace(from.FirstName + " " + from.LastName)
		if name != "" {
			fullName = name
		}
		if from.Username != "" {
			username = from.Username
		}
	}

	// Служебная «шапка» админу перед пересланным сообщением.
	_, metaErr := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    r.adminChatID,
		Text:      fmt.Sprintf(locales.MsgFeedbackMeta, fullName, chatID, username),
		ParseMode: "Markdown",
	})

	// Пересылаем само сообщение (текст/фото/документ) как есть — так
	// разработчик видит оригинал, включая вложения.
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
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgFeedbackSendError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateIdle)
	log.Printf("[FEEDBACK] отзыв доставлен админу от chatID=%d", chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgFeedbackConfirmed,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleRegularAnalysis - запускает обычный анализ.
func (r *router) handleRegularAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterRegular, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "regular")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "regular")
	r.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	r.setCurrentSection(chatID, "analysis")

	// Выбрано под-действие — убираем блок-хаб, чтобы в чате не висело меню
	// раздела поверх начатого анализа.
	r.deleteHubBlock(ctx, b, chatID)

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении анализа.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgRegularAnalysisIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleExtendedAnalysis - запускает расширенный анализ (с опросником).
func (r *router) handleExtendedAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterExtended, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "extended")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")

	r.stateManager.SetState(chatID, states.StateWaitingName)
	r.setCurrentSection(chatID, "analysis")

	// Выбрано под-действие — убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении опросника.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgExtendedAnalysisIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleBioscanStart - запускает bioscan.
func (r *router) handleBioscanStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterBioscanStart, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		log.Printf(locales.LogRouterAgreeNotDone, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		log.Printf(locales.LogRouterUserBusy, chatID, currentState)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	// Принудительно устанавливаем состояние
	r.stateManager.SetState(chatID, states.StateWaitingBioscanName)
	log.Printf(locales.LogRouterForceBioscan, chatID)
	r.setCurrentSection(chatID, "analysis")

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении Bioscan.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleDashboard — открывает веб-дашборд (только для Premium).
func (r *router) handleDashboard(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterDashboard, chatID)

	// Выбрано под-действие — убираем блок-хаб, чтобы в чате не висело меню
	// раздела поверх открытого дашборда.
	r.deleteHubBlock(ctx, b, chatID)
	r.setCurrentSection(chatID, "health")

	isPremium := r.paymentService.IsUserPremium(chatID)
	log.Printf(locales.LogDashboardPremiumCheck, chatID, isPremium)

	// Версия в URL сбрасывает кэш Telegram WebView (см. WebAppAssetsVersion).
	webAppTarget := WithWebAppVersion(r.webAppURL)
	if webAppTarget == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ URL дашборда не настроен. Задайте WEBAPP_URL или запустите `make mini`.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	text := locales.MsgHealthSummaryIntro + "\n\n"
	if !isPremium {
		// Полный доступ к показателям — по Premium, но профиль заполнить
		// можно бесплатно (онбординг доступен всем).
		text += "📝 Профиль можно заполнить бесплатно — после этого сводка оживёт. " +
			"Полный доступ к показателям крови и динамике — по Premium-подписке.\n\n"
	}

	// Только Mini App — без ссылок и «открыть в браузере».
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "Открыть", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		},
		{
			{Text: locales.BtnBack, CallbackData: "msg_back"},
		},
	}

	msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: rows},
		ParseMode:   "Markdown",
	})
	if sendErr != nil {
		log.Printf(locales.LogDashboardSendErr, chatID, sendErr)
	} else {
		log.Printf(locales.LogDashboardSent, chatID, msgID, webAppTarget, len(rows))
	}
	return true
}

// monitoringWebAppURL подменяет суффикс /dashboard на /monitoring в базовом
// URL, сохраняя хост/протокол (туннель и локальный сервер обслуживают оба
// пути). Если суффикса нет — возвращает URL как есть.
func monitoringWebAppURL(base string) string {
	if base == "" {
		return ""
	}
	const dash = "/dashboard"
	if i := strings.LastIndex(base, dash); i >= 0 {
		return base[:i] + "/monitoring" + base[i+len(dash):]
	}
	return base
}

// handleMonitoring — открывает веб-приложение «Мониторинг» (Premium-функция).
func (r *router) handleMonitoring(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[MONITORING] открытие для chatID=%d", chatID)

	// Выбрано под-действие — убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)
	r.setCurrentSection(chatID, "health")

	isPremium := r.paymentService.IsUserPremium(chatID)
	if !isPremium {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgMonitoringPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	// Версия в URL сбрасывает кэш Telegram WebView (см. WebAppAssetsVersion).
	webAppTarget := WithWebAppVersion(monitoringWebAppURL(r.webAppURL))
	if webAppTarget == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ URL мониторинга не настроен. Задайте WEBAPP_URL или запустите `make mini`.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	text := "📊 **Мониторинг**\n\n" +
		"Создавайте проекты и отслеживайте показатели во времени: курсы препаратов, диабет, похудение, здоровье."

	// Только Mini App — без ссылок и «открыть в браузере».
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "Открыть", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		},
		{
			{Text: locales.BtnBack, CallbackData: "msg_back"},
		},
	}

	msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: rows},
		ParseMode:   "Markdown",
	})
	if sendErr != nil {
		log.Printf("[MONITORING] ошибка отправки chatID=%d: %v", chatID, sendErr)
	} else {
		log.Printf("[MONITORING] сообщение отправлено chatID=%d msgID=%d url=%s кнопок=%d", chatID, msgID, webAppTarget, len(rows))
	}
	return true
}

// ============================================================================
// Быстрая консультация (с ИИ)
// ============================================================================

// freeConsultationLimit — сколько бесплатных консультаций доступно
// не-Premium пользователю. Premium — безлимит.
const freeConsultationLimit = 3

// consultUserKey — ключ счётчика использованных бесплатных консультаций
// в user-data состояния.
const consultUserKey = "ai_consult_count"

// consultCount — сколько бесплатных консультаций уже использовано.
func (r *router) consultCount(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, consultUserKey))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// consultSetCount — сохраняет счётчик использованных бесплатных консультаций.
func (r *router) consultSetCount(chatID int64, n int) {
	r.stateManager.SetUserData(chatID, consultUserKey, strconv.Itoa(n))
}

// consultationTextPrompt — формирует запрос к ИИ для текстовой консультации:
// задаёт роль консультанта и просит дать рекомендации + дисклеймер.
func consultationTextPrompt(question string) string {
	return "Вопрос пользователя о здоровье: " + strings.TrimSpace(question) +
		"\n\nТвоя задача — дать медицинскую консультацию: ответь на вопрос, " +
		"объясни возможные причины, дай практические рекомендации по облегчению " +
		"состояния. В конце обязательно напомни, что это информационная " +
		"консультация и не заменяет очный визит к врачу."
}

// consultationImageContext — формирует контекст к ИИ для консультации по
// фотографии (травма/проблемная зона). При наличии добавляет текстовый
// вопрос пользователя к фото.
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

// handleConsultationStart — запускает режим консультации: проверяет
// соглашение, Premium и оставшуюся бесплатную квоту. Если квота исчерпана и
// Premium нет — предлагает оформить подписку. Иначе переводит пользователя
// в StateWaitingConsultation, ожидая вопрос (текст) или фото.
func (r *router) handleConsultationStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[CONSULT] запуск для chatID=%d", chatID)

	// Выбрано под-действие — убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingConsultation)
	r.setCurrentSection(chatID, "health")

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении консультации.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgConsultationStart,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleConsultationMessage — обрабатывает сообщение пользователя в режиме
// StateWaitingConsultation: текстовый вопрос или фото травмы. Отправляет его
// ИИ (GenerateAnalysisSummary / анализ фото) и возвращает консультацию.
// Возвращает true, если сообщение обработано как консультация.
func (r *router) handleConsultationMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[CONSULT] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат — на уровень выше (хаб Здоровье), а не в главное меню.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[CONSULT] отмена chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	hasPhoto := update.Message != nil && len(update.Message.Photo) > 0

	// Пустое/неподдерживаемое сообщение (стикер, голос, ничего) — просим
	// прислать вопрос или фото.
	if strings.TrimSpace(text) == "" && !hasPhoto {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationEmpty,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)

	// Повторная проверка квоты на случай «залипшего» состояния.
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationPremiumRequired,
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	// Индикатор «ИИ думает».
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   locales.MsgConsultationProcessing,
	})

	var (
		result string
		err    error
	)

	if hasPhoto {
		photos := update.Message.Photo
		largest := photos[len(photos)-1]
		data, mimeType, dlErr := helpers.DownloadFileByID(ctx, b, largest.FileID, r.uploadDir)
		if dlErr != nil {
			log.Printf("[CONSULT] ошибка загрузки фото chatID=%d: %v", chatID, dlErr)
			r.stateManager.SetState(chatID, states.StateIdle)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgConsultationError,
				ReplyMarkup: keyboards.MainMenu(),
			})
			return true
		}
		result, err = r.analysisService.HandleAnalysisFromFileWithContext(ctx, data, mimeType, consultationImageContext(text))
	} else {
		result, err = r.analysisService.HandleAnalysis(ctx, consultationTextPrompt(text))
	}

	if err != nil || strings.TrimSpace(result) == "" {
		log.Printf("[CONSULT] ошибка генерации chatID=%d: %v", chatID, err)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	// Успех: считаем бесплатную квоту только для не-Premium пользователей.
	if !isPremium {
		r.consultSetCount(chatID, r.consultCount(chatID)+1)
	}
	r.stateManager.SetState(chatID, states.StateIdle)

	// Собираем итоговый текст (без Markdown — результат ИИ неконтролируем,
	// чтобы не сломать разметку). Клавиатуру (главное меню) крепим к
	// последнему куску через sendLongMessage.
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

	sendLongMessage(ctx, b, chatID, full, keyboards.MainMenu())
	return true
}

// sendLongMessage — отправляет текст, разбивая его на куски ≤ 4000 символов
// по границам строк, чтобы не упереться в лимит Telegram (4096). Клавиатура
// (keyboard) крепится только к последнему куску.
func sendLongMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, keyboard models.ReplyKeyboardMarkup) {
	const maxChunk = 4000
	runes := []rune(text)
	n := len(runes)
	if n <= maxChunk {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		return
	}

	chunks := []string{}
	for start := 0; start < n; {
		end := start + maxChunk
		if end > n {
			end = n
		}
		chunk := string(runes[start:end])
		if end < n {
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				end = start + idx + 1
				chunk = string(runes[start:end])
			}
		}
		chunks = append(chunks, chunk)
		start = end
	}

	for i, chunk := range chunks {
		kb := models.ReplyKeyboardMarkup{}
		if i == len(chunks)-1 {
			kb = keyboard
		}
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        chunk,
			ReplyMarkup: kb,
		})
	}
}
