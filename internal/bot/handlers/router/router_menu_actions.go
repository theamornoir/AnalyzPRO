package router

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleDiagnostics - показывает меню выбора типа анализа.
func (r *router) handleDiagnostics(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterDiagnostics, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgDiagnosticsIntro,
		ReplyMarkup: keyboards.AnalysisTypeMenu(),
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

	text := "📊 Мой Дашборд\n\n"
	if secure {
		text += "Нажмите кнопку ниже, чтобы открыть дашборд прямо в Telegram (Mini App).\n\n"
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
	text += "Открыть дашборд: " + linkURL

	// Клавиатура: WebApp-кнопка (только для защищённых/локальных URL,
	// иначе она «мертвая» на телефоне) + обычная кнопка-ссылка как запасной
	// вариант, который всегда открывается в браузере.
	rows := [][]models.InlineKeyboardButton{}
	if secure {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "📊 Открыть Дашборд (Mini App)", WebApp: &models.WebAppInfo{URL: webAppTarget}},
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
