package router

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	apmodels "github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
)

// handleHealthAssessment - завершающий шаг «Общей оценки здоровья»
// (бывший расширенный анализ). Вызывается из handle(), когда последний
// вопрос опросника перевёл состояние в StateWaitingHealthAssessment. Строит
// отчёт ИСКЛЮЧИТЕЛЬНО по тексту опросника (без загрузки файлов), отправляет
// его в чат как PDF-файл (как Bioscan PRO) и сохраняет красивый HTML-дашборд
// в «Мой профиль» для просмотра в мини-приложении.
func (r *router) handleHealthAssessment(ctx context.Context, b *tgbot.Bot, chatID int64) {
	collector := userdata.NewUserDataCollector(r.stateManager)
	collected := collector.FormatCollected(chatID)

	loadingMsg, textMsg := apmodels.SendLoadingMessages(ctx, b, chatID, r.stickerID, locales.HealthAssessmentLoadingSteps)

	jsonReport, err := r.analysisService.HandleHealthAssessment(ctx, collected)
	if err != nil || strings.TrimSpace(jsonReport) == "" {
		log.Printf("[HEALTH] ошибка генерации общей оценки здоровья chatID=%d: %v", chatID, err)
		apmodels.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		r.stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgTextProcessingError,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}

	ha, perr := report.ParseHealthAssessmentJSON(jsonReport)
	if perr != nil {
		log.Printf("[HEALTH] не удалось разобрать JSON общей оценки здоровья chatID=%d: %v", chatID, perr)
		apmodels.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		r.stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgTextProcessingError,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}

	// Подставляем имя пользователя (для шапки дашборда) и вырезаем его из
	// текстовых полей отчёта, если ИИ всё же добавил («Влад спит 8 часов»
	// -> «спит 8 часов»). Шапка показывает имя, сами разборы - обезличенно.
	name := r.stateManager.GetUserData(chatID, "name")
	ha.Name = strings.TrimSpace(name)
	report.SanitizeHealthAssessment(&ha, name)

	// Проверяем содержательность отчёта: если ИИ вернул «пустой»/мусорный
	// результат (нулевой индекс, нет развёрнутых разборов сфер), не шлём
	// пользователю пугающий отчёт с карточками «Критично · 0». Вместо
	// этого - дружелюбное сообщение с предложением пройти опросник ещё раз.
	if verr := report.ValidateHealthAssessment(ha); verr != nil {
		log.Printf("[HEALTH] отчёт оценки здоровья не прошёл валидацию качества chatID=%d: %v", chatID, verr)
		apmodels.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		r.stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgHealthAssessmentLowQuality,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}

	// Отправляем отчёт «Общая оценка здоровья» в чат как PDF-файл (как
	// Bioscan PRO): тот же красивый HTML-дашборд (фиолетовый неон с
	// кольцевыми диаграммами) конвертируется в PDF через внешний сервис
	// html2pdf.app (r.pdfConverter), чтобы итоговый файл выглядел точно как
	// дизайн, присланный пользователем. Сам HTML-дашборд параллельно
	// сохраняется в «Мой профиль» (ReportHTML) для просмотра в мини-приложении.
	// При сбое конвертации (нет ключа html2pdf.app / сервис недоступен) -
	// откат к отправке HTML-файла, чтобы результат не потерялся.
	htmlResult := report.RenderHealthAssessmentHTML(ha, time.Now(), true)
	pdfBytes, convErr := r.pdfConverter.ConvertHTML(ctx, htmlResult)

	// Отправляем отчёт в чат. Индикатор загрузки (стикер + текст) гасим
	// СТРОГО ПОСЛЕ того, как b.SendDocument завершил загрузку и доставку
	// файла: так анимация видна до самого появления PDF/HTML в чате и
	// исчезает вместе с ним, без «немой паузы» между исчезновением загрузки
	// и приходом файла (конвертация HTML->PDF и сама отправка файла в
	// Telegram занимают несколько секунд).
	var sendErr error
	if convErr == nil && len(pdfBytes) > 0 {
		_, sendErr = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &tgmodels.InputFileUpload{
				Filename: "Prisma_health_assessment.pdf",
				Data:     bytes.NewReader(pdfBytes),
			},
			Caption: locales.MsgHealthAssessmentPdfCaption,
		})
	} else {
		if convErr != nil {
			log.Printf("[HEALTH] не удалось конвертировать дашборд оценки здоровья в PDF chatID=%d: %v - отправляю HTML", chatID, convErr)
		}
		_, sendErr = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &tgmodels.InputFileUpload{
				Filename: "Prisma_health_assessment.html",
				Data:     bytes.NewReader([]byte(htmlResult)),
			},
			Caption: locales.MsgHealthAssessmentHtmlCaption,
		})
	}

	// Гасим индикатор загрузки ТОЛЬКО после фактической отправки отчёта:
	// до этого момента анимация держится даже во время загрузки файла в
	// Telegram, поэтому «пустой паузы» между исчезновением загрузки и
	// приходом отчёта не возникает.
	apmodels.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)

	if sendErr != nil {
		log.Printf("[HEALTH] не удалось отправить отчёт оценки здоровья chatID=%d: %v", chatID, sendErr)
	}

	// Сохраняем результат в «Мой профиль» (история пользователя), чтобы он
	// был доступен там вместе с прочими результатами (вкладка «Оценка
	// здоровья» дашборда).
	saveHealthAssessmentResult(ctx, r.monitorRepo, chatID, ha)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём кнопку для
	// мгновенного открытия.
	apmodels.SendSavedToSummary(ctx, b, chatID, r.webAppURL)

	r.stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgHealthAssessmentDone,
		ReplyMarkup: reportFeedbackKeyboard(),
		ParseMode:   "Markdown",
	})
}

// reportFeedbackKeyboard - inline-клавиатура обратной связи под результатами
// отчётов (Общая оценка здоровья, базовый Bioscan и т.п.). Позволяет
// пользователю быстро оценить результат или оставить комментарий.
func reportFeedbackKeyboard() tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
			{
				{Text: locales.BtnFeedbackLike, CallbackData: "report_feedback_like"},
				{Text: locales.BtnFeedbackDislike, CallbackData: "report_feedback_dislike"},
			},
		},
	}
}

// saveHealthAssessmentResult - сохраняет отчёт «Общая оценка здоровья» в
// историю пользователя как запись типа "health_assessment", чтобы он
// появился в «Мой профиль» (вкладка «Оценка здоровья»). Формирует аккуратный
// HTML-документ (ReportHTML), чтобы кнопка просмотра отчёта открывала именно
// этот результат без ошибки рендера.
func saveHealthAssessmentResult(ctx context.Context, saver monitoring.Repository, chatID int64, ha models.HealthAssessment) {
	if saver == nil {
		return
	}
	jsonData, err := json.Marshal(ha)
	if err != nil {
		return
	}
	entry := &monitoring.HistoryEntry{
		TelegramID: chatID,
		Type:       "health_assessment",
		Title:      locales.MsgHealthAssessmentTitle,
		Date:       time.Now(),
		JsonData:   string(jsonData),
		// Сохраняем тот же красивый HTML-дашборд, что и в чате (фиолетовый
		// неон), чтобы кнопка «Открыть отчёт» в «Моём профиле» показывала
		// именно его (а не «голый» текст).
		ReportHTML: report.RenderHealthAssessmentHTML(ha, time.Now(), false),
	}
	if err := saver.SaveResult(ctx, entry); err != nil {
		log.Printf("[HEALTH] не удалось сохранить оценку здоровья chatID=%d: %v", chatID, err)
	}
}
