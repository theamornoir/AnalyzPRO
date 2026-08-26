package router

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

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
// отчёт ИСКЛЮЧИТЕЛЬНО по тексту опросника (без загрузки файлов), выводит
// его текстом в чат и сохраняет в «Мой профиль».
func (r *router) handleHealthAssessment(ctx context.Context, b *tgbot.Bot, chatID int64) {
	collector := userdata.NewUserDataCollector(r.stateManager)
	collected := collector.FormatCollected(chatID)

	loadingMsg, textMsg := apmodels.SendLoadingMessages(ctx, b, chatID, r.stickerID, locales.HealthAssessmentLoadingSteps)

	jsonReport, err := r.analysisService.HandleHealthAssessment(ctx, collected)
	apmodels.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
	if err != nil || strings.TrimSpace(jsonReport) == "" {
		log.Printf("[HEALTH] ошибка генерации общей оценки здоровья chatID=%d: %v", chatID, err)
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
		r.stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgTextProcessingError,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}

	result := report.RenderHealthAssessmentText(ha)
	apmodels.SendLongMessagePlain(ctx, b, chatID, result)

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
		ReplyMarkup: keyboards.MainMenuInline(),
		ParseMode:   "Markdown",
	})
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
		ReportHTML: apmodels.PlainResultHTML(locales.MsgHealthAssessmentTitle, report.RenderHealthAssessmentText(ha)),
	}
	if err := saver.SaveResult(ctx, entry); err != nil {
		log.Printf("[HEALTH] не удалось сохранить оценку здоровья chatID=%d: %v", chatID, err)
	}
}
