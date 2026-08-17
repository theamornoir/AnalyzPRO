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
	c.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	// Показываем собранные данные
	userData := c.stateManager.GetAllUserData(chatID)
	summary := formatUserData(userData)

	name := userData["name"]
	if name == "" {
		name = locales.MsgUserDefaultName
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf(locales.MsgUserDataSaved, name, summary),
		ReplyMarkup: keyboards.BackMenu(),
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
	if chronic := data["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryChronicDiseases, chronic))
	}
	if allergies := data["allergies"]; allergies != "" && strings.ToLower(allergies) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryAllergies, allergies))
	}
	if medications := data["medications"]; medications != "" && strings.ToLower(medications) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryMedications, medications))
	}
	if smoking := data["smoking"]; smoking != "" && strings.ToLower(smoking) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummarySmoking, smoking))
	}
	if alcohol := data["alcohol"]; alcohol != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryAlcohol, alcohol))
	}
	if sport := data["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummarySportType, sport))
	}
	if exp := data["training_experience"]; exp != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryTrainingExperience, exp))
	}
	if goal := data["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryGoal, goal))
	}
	if sleep := data["sleep"]; sleep != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummarySleep, sleep))
	}
	if stress := data["stress"]; stress != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryStress, stress))
	}
	if veg := data["nutrition_veg"]; veg != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryNutritionVeg, veg))
	}
	if proc := data["nutrition_processed"]; proc != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryNutritionProcessed, proc))
	}
	if water := data["water"]; water != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryWater, water))
	}
	if activity := data["activity"]; activity != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryActivity, activity))
	}
	if family := data["family_history"]; family != "" && strings.ToLower(family) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryFamilyHistory, family))
	}
	if digestion := data["digestion"]; digestion != "" && strings.ToLower(digestion) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgUserSummaryDigestion, digestion))
	}

	return strings.Join(parts, "\n")
}
