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
	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleAnalysisHub - раздел-хаб «Анализы» (верхняя кнопка главного меню):
// показывает описание раздела и под-действия (Обычный/Расширенный анализ,
// Bioscan). Проверка соглашения остаётся внутри под-действий.
func (r *router) handleAnalysisHub(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterDiagnostics, chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgAnalysisHubIntro,
		ReplyMarkup: keyboards.AnalysisHubMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleServiceHub - раздел-хаб «Сервис» (верхняя кнопка главного меню):
// описание раздела и под-действия (Отзывы и предложения / О сервисе).
func (r *router) handleServiceHub(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[SERVICE-HUB] открытие хаба для chatID=%d", chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgServiceHubIntro,
		ReplyMarkup: keyboards.ServiceHubMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleFeedbackStart - запускает режим ввода отзыва/предложения: описывает
// раздел и переводит пользователя в StateWaitingFeedback, ожидая следующее
// сообщение (текст/фото/документ), которое будет переслано разработчику.
func (r *router) handleFeedbackStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[FEEDBACK] открытие раздела для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingFeedback)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgFeedbackIntro,
		ReplyMarkup: keyboards.FeedbackMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleFeedbackMessage - пересылает сообщение пользователя (текст/фото/
// документ) разработчику (adminChatID) и подтверждает доставку. Срабатывает
// при любом сообщении в режиме StateWaitingFeedback. Возвращает true, если
// сообщение обработано как отзыв.
func (r *router) handleFeedbackMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[FEEDBACK] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат из режима ввода отзыва.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[FEEDBACK] отмена ввода chatID=%d", chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgFeedbackCancelled,
			ReplyMarkup: keyboards.MainMenu(),
		})
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

// handleHealthHub - раздел-хаб «Здоровье» (верхняя кнопка главного меню):
// описание раздела и под-действия (Сводка здоровья / Мониторинг /
// Консультация ИИ). Проверка Premium остаётся внутри под-действий
// (handleDashboard / handleMonitoring), запуск консультации — внутри
// handleConsultationStart (section_consult_start).
func (r *router) handleHealthHub(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[HEALTH-HUB] открытие хаба для chatID=%d", chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgHealthHubIntro,
		ReplyMarkup: keyboards.HealthHubMenu(),
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

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgRegularAnalysisIntro,
		ReplyMarkup: keyboards.ProcessAnalysisMenu(),
		ParseMode:   "Markdown",
	})
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

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgExtendedAnalysisIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
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

	// Проверяем, что состояние установилось
	if r.stateManager.GetState(chatID) != states.StateWaitingBioscanName {
		log.Printf(locales.LogRouterSetStateFail, r.stateManager.GetState(chatID))
	}

	// Сбрасываем данные bioscan
	bioscan.ResetBioscanData(r.stateManager, chatID)
	r.stateManager.SetUserData(chatID, "analysis_type", "")

	log.Printf(locales.LogRouterBioscanLaunch, chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
	return true
}

// isSecureWebAppURL — Telegram Web App (кнопка WebApp) требует HTTPS ИЛИ
// localhost/127.0.0.1 (для тестов на той же машине, например в Telegram
// Desktop). LAN-IP (192.168.x.x и т.п.) и прочие http:// НЕ годятся: такая
// кнопка либо отклоняется API на телефоне (400 Bad Request), либо не
// открывается с другого устройства. Поэтому WebApp-кнопку добавляем для
// https и для localhost; для LAN-IP и прочих http используем обычную
// кнопку-ссылку (URL), которая открывается в браузере / встроенном
// браузере Telegram (при той же сети).
func isSecureWebAppURL(rawURL string) bool {
	if strings.HasPrefix(rawURL, "https") {
		return true
	}
	if strings.HasPrefix(rawURL, "http://localhost") || strings.HasPrefix(rawURL, "http://127.0.0.1") {
		return true
	}
	return false
}

// handleDashboard — открывает веб-дашборд (только для Premium).
func (r *router) handleDashboard(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterDashboard, chatID)

	isPremium := r.paymentService.IsUserPremium(chatID)
	log.Printf(locales.LogDashboardPremiumCheck, chatID, isPremium)
	if !isPremium {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		log.Printf(locales.LogDashboardNotPremiumSent, chatID)
		return true
	}

	linkURL := r.dashboardURL
	if linkURL == "" {
		linkURL = r.webAppURL
	}
	webAppTarget := r.webAppURL
	if webAppTarget == "" {
		webAppTarget = r.dashboardURL
	}
	if linkURL == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ URL дашборда не настроен. Задайте WEBAPP_URL или запустите `make tunnel`.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	// WebApp-кнопка требует HTTPS (кроме localhost в десктопе). Обычная
	// ссылка (linkURL) всегда открывается в браузере/встроенном браузере
	// Telegram — для неё подставляем LAN-IP, чтобы дашборд открывался с
	// телефона в той же Wi-Fi сети.
	secure := isSecureWebAppURL(webAppTarget)
	linkIsLAN := !isSecureWebAppURL(linkURL)

	text := locales.MsgHealthSummaryIntro + "\n\n"
	if secure {
		text += "Нажмите кнопку ниже, чтобы открыть Сводку здоровья прямо в Telegram (Mini App).\n\n"
	}
	if linkIsLAN {
		text += "Ссылка ниже ведёт на этот компьютер по локальной сети — откройте её в " +
			"браузере или встроенном браузере Telegram (телефон должен быть в той же " +
			"Wi-Fi сети, что и этот компьютер). Для полноценного Mini App в телефоне " +
			"запустите make tunnel (HTTPS-туннель) — бот сам подхватит https-URL.\n\n"
	} else if !secure {
		text += "Встроенная кнопка Mini App требует HTTPS. Откройте ссылку ниже в " +
			"браузере. Для Mini App в телефоне запустите make tunnel.\n\n"
	}
	text += "Открыть Сводку здоровья: " + linkURL

	// Клавиатура: WebApp-кнопка (только для защищённых/локальных URL,
	// иначе она «мертвая» на телефоне) + обычная кнопка-ссылка как запасной
	// вариант, который всегда открывается в браузере.
	rows := [][]models.InlineKeyboardButton{}
	if secure {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "💡 Открыть Сводку здоровья (Mini App)", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🌐 Открыть в браузере", URL: linkURL},
	})

	msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: rows},
		ParseMode:   "Markdown",
	})
	if sendErr != nil {
		log.Printf(locales.LogDashboardSendErr, chatID, sendErr)
	} else {
		log.Printf(locales.LogDashboardSent, chatID, msgID, linkURL, len(rows))
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

	linkURL := r.dashboardURL
	if linkURL == "" {
		linkURL = r.webAppURL
	}
	linkURL = monitoringWebAppURL(linkURL)

	webAppTarget := r.webAppURL
	if webAppTarget == "" {
		webAppTarget = r.dashboardURL
	}
	webAppTarget = monitoringWebAppURL(webAppTarget)

	if webAppTarget == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ URL мониторинга не настроен. Задайте WEBAPP_URL или запустите `make mini`.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	secure := isSecureWebAppURL(webAppTarget)
	linkIsLAN := !isSecureWebAppURL(linkURL)

	text := "📊 **Мониторинг**\n\n" +
		"Создавайте проекты и отслеживайте показатели во времени: курсы препаратов, диабет, похудение, здоровье.\n\n"
	if secure {
		text += "Нажмите кнопку ниже, чтобы открыть Мониторинг прямо в Telegram (Mini App).\n\n"
	}
	if linkIsLAN {
		text += "Ссылка ведёт на этот компьютер по локальной сети — откройте её в браузере/встроенном браузере Telegram (телефон в той же Wi-Fi). Для Mini App в телефоне запустите `make mini`.\n\n"
	} else if !secure {
		text += "Встроенная кнопка Mini App требует HTTPS. Откройте ссылку в браузере или запустите `make mini`.\n\n"
	}
	text += "Открыть Мониторинг: " + linkURL

	rows := [][]models.InlineKeyboardButton{}
	if secure {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "📊 Открыть Мониторинг (Mini App)", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "🌐 Открыть в браузере", URL: linkURL},
	})

	msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: rows},
		ParseMode:   "Markdown",
	})
	if sendErr != nil {
		log.Printf("[MONITORING] ошибка отправки chatID=%d: %v", chatID, sendErr)
	} else {
		log.Printf("[MONITORING] сообщение отправлено chatID=%d msgID=%d url=%s кнопок=%d", chatID, msgID, linkURL, len(rows))
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
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgConsultationStart,
		ReplyMarkup: keyboards.FeedbackMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleConsultationMessage — обрабатывает сообщение пользователя в режиме
// StateWaitingConsultation: текстовый вопрос или фото травмы. Отправляет его
// ИИ (GenerateAnalysisSummary / анализ фото) и возвращает консультацию.
// Возвращает true, если сообщение обработано как консультация.
func (r *router) handleConsultationMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[CONSULT] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат из режима консультации.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[CONSULT] отмена chatID=%d", chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationCancelled,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	hasPhoto := update.Message != nil && len(update.Message.Photo) > 0

	// Пустое/неподдерживаемое сообщение (стикер, голос, ничего) — просим
	// прислать вопрос или фото.
	if strings.TrimSpace(text) == "" && !hasPhoto {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationEmpty,
			ReplyMarkup: keyboards.FeedbackMenu(),
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
