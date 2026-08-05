package keyboards

import (
	"github.com/go-telegram/bot/models"
)

// StartMenu - меню после /start (до принятия соглашения)
func StartMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "📝 Пользовательское соглашение"}, // <-- ИЗМЕНЕНО
			},
			{
				{Text: "ℹ️ О сервисе"},
			},
		},
		ResizeKeyboard: true,
	}
}

// MainMenu - основное меню (после принятия соглашения)
func MainMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "🏥 Обычный анализ"},
				{Text: "🏋️ Анализ для спортсмена"},
			},
			{
				{Text: "📸 Bioscan"},
				{Text: "📝 Отзывы и предложения"},
			},
			{
				{Text: "💎 Premium"},
				{Text: "ℹ️ О сервисе"},
			},
		},
		ResizeKeyboard: true,
	}
}

// BackMenu - меню с кнопкой "Назад"
func BackMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "⬅️ Назад"},
			},
		},
		ResizeKeyboard: true,
	}
}

// AgreementMenu - меню с кнопкой "Принять" для соглашения
func AgreementMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "✅ Принять соглашение"},
			},
		},
		ResizeKeyboard: true,
	}
}

// UserAgreementText - текст пользовательского соглашения
func UserAgreementText() string {
	return `📝 **ПОЛЬЗОВАТЕЛЬСКОЕ СОГЛАШЕНИЕ**

Я, AnalyzPRO, предоставляю услуги по интерпретации медицинских анализов с использованием искусственного интеллекта.

⚠️ **ВАЖНО:**
1. Бот НЕ ставит диагнозы и НЕ заменяет врача.
2. Результаты носят информационный характер.
3. Всегда консультируйтесь с квалифицированным врачом.
4. Ответственность за использование результатов лежит на пользователе.
5. Ваши данные используются только для анализа и не передаются третьим лицам.

📅 Версия соглашения: 1.0 от 05.08.2026

Нажмите кнопку ниже, чтобы принять соглашение.`
}
