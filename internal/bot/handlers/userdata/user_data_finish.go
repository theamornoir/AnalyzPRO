package userdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// finishCollection - завершает сбор данных.
func (c *UserDataCollector) finishCollection(ctx context.Context, b *tgbot.Bot, chatID int64) {
	// Показываем собранные данные
	userData := c.stateManager.GetAllUserData(chatID)
	summary := formatUserData(userData)

	name := userData["name"]
	if name == "" {
		name = locales.MsgUserDefaultName
	}

	// «Общая оценка здоровья» (бывший расширенный анализ): самодостаточный
	// блок БЕЗ загрузки файлов. Все ответы уже собраны - переводим в
	// состояние подтверждения и показываем экран с собранными данными и
	// кнопкой «Подтвердить и отправить». ИИ НЕ вызывается, пока
	// пользователь не нажмёт кнопку (callback health_assessment_confirm) -
	// см. роутер. Шаг загрузки PDF/фото здесь не нужен.
	if c.stateManager.GetUserData(chatID, "analysis_subtype") == "extended" {
		c.stateManager.SetState(chatID, states.StateWaitingHealthAssessmentConfirm)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf(locales.MsgHealthAssessmentConfirm, name, summary),
			ReplyMarkup: keyboards.HealthAssessmentConfirmMenu(),
			ParseMode:   "Markdown",
		})
		return
	}

	// legacy-путь (загрузка файлов) - не используется, оставлен для
	// обратной совместимости состояний.
	c.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf(locales.MsgUserDataSaved, name, summary),
		ReplyMarkup: keyboards.BackInline(),
		ParseMode:   "Markdown",
	})
}

// formatUserData - форматирует данные пользователя для показа.
func formatUserData(data map[string]string) string {
	var parts []string
	parts = append(parts, locales.MsgUserDataSummaryHeader)
	parts = append(parts, "")

	if name := data["name"]; name != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryName, name))
	}
	if gender := data["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryGender, gender))
	}
	if age := data["age"]; age != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryAge, age))
	}
	if height := data["height"]; height != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryHeight, height))
	}
	if weight := data["weight"]; weight != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryWeight, weight))

		if height := data["height"]; height != "" {
			if weight := data["weight"]; weight != "" {
				h, _ := strconv.ParseFloat(height, 64)
				w, _ := strconv.ParseFloat(weight, 64)
				if h > 0 && w > 0 {
					bmi := w / ((h / 100) * (h / 100))
					parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryBMI, bmi))
				}
			}
		}
	}
	if goal := data["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryGoal, goal))
	}
	if lifestyle := data["lifestyle"]; lifestyle != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryLifestyle, lifestyle))
	}
	if habits := data["habits"]; habits != "" && strings.ToLower(habits) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryHabits, habits))
	}

	return strings.Join(parts, "\n")
}

// FormatCollected - возвращает отформатированный текст всех собранных ответов
// опросника для передачи ИИ (без прогресс-бара и служебных префиксов).
// Используется «Общей оценкой здоровья»: отчёт строится ИСКЛЮЧИТЕЛЬНО по
// тексту опросника (без загрузки медицинских файлов).
func (c *UserDataCollector) FormatCollected(chatID int64) string {
	return formatUserData(c.stateManager.GetAllUserData(chatID))
}
