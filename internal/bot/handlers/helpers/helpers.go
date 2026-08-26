package helpers

import (
	"context"
	"strings"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// SendError - отправляет сообщение об ошибке вместе с главным меню, чтобы
// после сбоя пользователь никогда не «зависал» без клавиатуры и мог вернуться
// в меню.
func SendError(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgTextProcessingError,
		ReplyMarkup: keyboards.MainMenuInline(),
	})
}

// DeleteMessage - удаляет сообщение. Если удаляемое сообщение - текст
// индикатора загрузки, попутно гасит его анимацию.
func DeleteMessage(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	CancelLoadingAnimation(messageID)
	_, _ = b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
}

// loadingAnimations - карта активных анимаций загрузки: ключ - message_id
// текстового сообщения индикатора, значение - функция отмены горутины.
var loadingAnimations sync.Map

// CancelLoadingAnimation - останавливает анимацию индикатора загрузки,
// связанную с текстовым сообщением messageID (если есть). Безопасно
// вызывать повторно и для несуществующего ID.
func CancelLoadingAnimation(messageID int) {
	if v, ok := loadingAnimations.LoadAndDelete(messageID); ok {
		if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
	}
}

// SafeDeleteLoadingMsgs - безопасно удаляет сообщения о загрузке (стикер и
// текст). Удаление текста также гасит анимацию (см. DeleteMessage).
func SafeDeleteLoadingMsgs(ctx context.Context, b *tgbot.Bot, chatID int64, stickerMsg, textMsg *models.Message) {
	if stickerMsg != nil {
		DeleteMessage(ctx, b, chatID, stickerMsg.ID)
	}
	if textMsg != nil {
		DeleteMessage(ctx, b, chatID, textMsg.ID)
	}
}

// defaultLoadingSteps - фразы по умолчанию для анимированного индикатора
// ожидания (анализ/отчёт). Циклически сменяются в текстовом сообщении.
var defaultLoadingSteps = []string{
	locales.LoadingStepFormReport,
	locales.LoadingStepCountMetrics,
	locales.LoadingStepAnalyze,
	locales.LoadingStepRecommend,
	locales.LoadingStepAlmostDone,
}

// normalizeStickerID - приводит переданный ID стикера к «настоящему».
// Плейсхолдер из .env-примера («your_sticker_id») считаем «стикер не задан»,
// чтобы не пытаться отправить заведомо невалидный стикер (иначе SendSticker
// падает и анимации не видно).
func normalizeStickerID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || id == "your_sticker_id" {
		return ""
	}
	return id
}

// SendLoadingMessages - отправляет индикатор ожидания на время длительной
// операции (анализ / биоскан / консультация).
//
// Если задан валидный stickerID - шлёт анимированный стикер, под ним текст,
// циклически сменяющий фразы из steps (каждые ~2с).
//
// Если стикер недоступен (не задан или плейсхолдер) - вместо стикера
// показываем ВСТРОЕННУЮ анимацию Telegram («отправка документа»,
// SendChatAction) + циклически меняющийся текст. Так индикатор ожидания
// виден ВСЕГДА, без привязки к настроенному файлу стикера.
//
// Удаляйте индикатор через SafeDeleteLoadingMsgs/DeleteMessage (по textMsg.ID)
// - это также гасит анимацию и chat-action.
func SendLoadingMessages(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	stickerID string,
	steps []string,
) (*models.Message, *models.Message) {

	if len(steps) == 0 {
		steps = defaultLoadingSteps
	}
	firstText := steps[0]

	effectiveSticker := normalizeStickerID(stickerID)

	var stickerMsg *models.Message
	if effectiveSticker != "" {
		stickerMsg, _ = b.SendSticker(ctx, &tgbot.SendStickerParams{
			ChatID: chatID,
			Sticker: &models.InputFileString{
				Data: effectiveSticker,
			},
		})
	}

	if stickerMsg == nil {
		// Запасной путь: встроенная анимация Telegram + текстовый индикатор.
		textMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   firstText,
		})
		if textMsg == nil {
			return nil, nil
		}
		// stickerMsg == textMsg: SafeDeleteLoadingMsgs удалит ровно одно
		// сообщение (textMsg), а DeleteMessage(textMsg.ID) погасит анимацию.
		stickerMsg = textMsg
		startLoadingAnimation(ctx, b, chatID, textMsg.ID, steps, true)
		return stickerMsg, textMsg
	}

	// Стикер есть - анимируем только текст под ним.
	textMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   firstText,
	})
	if textMsg == nil {
		return stickerMsg, nil
	}
	startLoadingAnimation(ctx, b, chatID, textMsg.ID, steps, false)
	return stickerMsg, textMsg
}

// startLoadingAnimation запускает анимацию индикатора ожидания для сообщения
// textMsgID: текст циклически меняется (каждые ~2с) фразами из steps. Если
// withChatAction=true - дополнительно периодически шлёт SendChatAction
// («отправка документа»), давая встроенную анимацию Telegram поверх стикера.
// Останавливается при удалении сообщения (DeleteMessage → CancelLoadingAnimation
// по textMsg.ID).
func startLoadingAnimation(ctx context.Context, b *tgbot.Bot, chatID int64, textMsgID int, steps []string, withChatAction bool) {
	animCtx, cancel := context.WithCancel(ctx)
	loadingAnimations.Store(textMsgID, cancel)
	txtID := textMsgID

	sendAction := func() {
		if withChatAction {
			_, _ = b.SendChatAction(animCtx, &tgbot.SendChatActionParams{
				ChatID: chatID,
				Action: models.ChatActionUploadDocument,
			})
		}
	}
	sendAction() // сразу при старте, чтобы анимация появилась до первого тика

	go func() {
		idx := 0
		textTicker := time.NewTicker(2 * time.Second)
		actionTicker := time.NewTicker(4 * time.Second)
		defer textTicker.Stop()
		defer actionTicker.Stop()
		for {
			select {
			case <-animCtx.Done():
				return
			case <-actionTicker.C:
				sendAction()
			case <-textTicker.C:
				if len(steps) <= 1 {
					return
				}
				idx = (idx + 1) % len(steps)
				_, _ = b.EditMessageText(animCtx, &tgbot.EditMessageTextParams{
					ChatID:    chatID,
					MessageID: txtID,
					Text:      steps[idx],
				})
			}
		}
	}()
}
