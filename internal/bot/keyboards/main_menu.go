package keyboards

import (
	"github.com/go-telegram/bot/models"
)

func MainMenu() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "📤 Загрузить анализ"},
				{Text: "📊 История"},
			},
			{
				{Text: "💎 Premium"},
				{Text: "ℹ️ О сервисе"},
			},
		},
		ResizeKeyboard: true,
		OneTimeKeyboard: false,
	}
}
