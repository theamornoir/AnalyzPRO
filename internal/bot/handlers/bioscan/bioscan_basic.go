package bioscan

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/service"
)

// StartBioscanBasicFlow - начало БАЗОВОГО (бесплатного) Bioscan: 1 фото ->
// текстовый результат в чат. Без вопросника и без PDF.
func StartBioscanBasicFlow(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	// Проверка принятия соглашения выполняется вызывающим
	// (router.handleBioscanBasicStart через agreementStorage.IsAgreed).
	// Согласие хранится в отдельном AgreementStorage, а не в user-data под
	// ключом "agreement_accepted", поэтому внутренняя проверка здесь
	// не нужна (иначе базовый Bioscan был бы недоступен никому).

	// Сбрасываем предыдущие данные Bioscan
	ResetBioscanData(sm, chatID)
	sm.SetUserData(chatID, "bioscan_basic_photo", "")

	// Ожидаем одно фото
	sm.SetState(chatID, states.StateWaitingBioscanBasicPhoto)

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении Basic Bioscan.
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanBasicPhoto - обработка единственного фото в базовом режиме.
// Анализирует фото и выдаёт plain-text рекомендации прямо в чат (без
// markdown-форматирования).
func HandleBioscanBasicPhoto(
	ctx context.Context,
	b *tgbot.Bot,
	sm states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
	chatID int64,
	photos []models.PhotoSize,
	saver monitoring.Repository,
	webAppURL string,
) {
	if len(photos) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgBioscanBasicPhotoPrompt,
		})
		return
	}

	photo := photos[len(photos)-1]

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID, nil)

	data, _, dlErr := helpers.DownloadFileByID(ctx, b, photo.FileID, uploadDir)
	if dlErr != nil {
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		sm.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanDownloadError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	result, err := analysisService.HandleBioscanText(ctx, [][]byte{data}, "image/jpeg", "")
	if err != nil {
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		log.Printf(locales.LogBioscanError, err)
		sm.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanProcessingError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)

	// Результат - ОБЫЧНОЕ сообщение БЕЗ форматирования (без ParseMode).
	// Может превышать лимит Telegram (4096) - дробим на куски <= 4000
	// символов, иначе сообщение не дойдёт, хотя бот пошлёт «Готово».
	if strings.TrimSpace(result) != "" {
		helpers.SendLongMessagePlain(ctx, b, chatID, result)
	}

	sm.SetState(chatID, states.StateIdle)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicDone,
		ReplyMarkup: keyboards.MainMenu(),
	})

	// Сохраняем БАЗОВЫЙ биоскан в «Мой профиль», чтобы он был доступен
	// там вместе с прочими результатами (обычный/расширенный анализ,
	// Bioscan PRO). Только если ИИ вернул непустой результат.
	title := locales.MsgBioscanBasicTitle
	if saver != nil && strings.TrimSpace(result) != "" {
		entry := &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "bioscan",
			Title:      title,
			Date:       time.Now(),
			JsonData:   fmt.Sprintf(`{"title":%q,"note":%q}`, title, result),
			ReportHTML: helpers.PlainResultHTML(title, result),
		}
		if err := saver.SaveResult(ctx, entry); err != nil {
			log.Printf("[BIOSCAN] не удалось сохранить базовый биоскан chatID=%d: %v", chatID, err)
		} else {
			log.Printf("[BIOSCAN] базовый биоскан сохранён chatID=%d", chatID)
		}
	}

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}
