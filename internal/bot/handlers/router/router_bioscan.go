package router

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
)

// handleBioscanStates - обработка состояний Bioscan. Возвращает true, если обработано.
func (r *router) handleBioscanStates(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	state := r.stateManager.GetState(chatID)

	// Базовый Bioscan: первый шаг - приём фото пользователя (фигура целиком
	// ИЛИ скриншот показателей умных весов/фитнес-приложения). Обрабатываем
	// «Назад»/«Отмена» и само фото здесь, до общих ранних return для
	// BtnBack/BtnCancel (чтобы они не ушли в handleBack).
	if state == states.StateWaitingBioscanBasicPhoto {
		if text == locales.BtnBack {
			bioscan.BackBioscanBasic(ctx, b, r.stateManager, chatID)
		} else if text == locales.BtnCancel {
			bioscan.CancelBioscanBasic(ctx, b, r.stateManager, chatID)
		} else if update != nil && update.Message != nil && len(update.Message.Photo) > 0 {
			bioscan.HandleBioscanBasicPhoto(ctx, b, r.stateManager, chatID, update.Message.Photo, update.Message.ID)
		} else {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgBioscanBasicPhotoPrompt,
				ReplyMarkup: keyboards.BackQuestionInline(),
			})
		}
		return true
	}

	// Базовый Bioscan: мини-опросник (пол/возраст/рост/вес/цель/тренировки),
	// идущий ПОСЛЕ приёма фото. Обрабатываем «Назад»/«Отмена» и шаги
	// опросника здесь, до общих ранних return для BtnBack/BtnCancel.
	if state == states.StateWaitingBioscanBasicQ {
		if text == locales.BtnBack {
			bioscan.BackBioscanBasic(ctx, b, r.stateManager, chatID)
		} else if text == locales.BtnCancel {
			bioscan.CancelBioscanBasic(ctx, b, r.stateManager, chatID)
		} else if bioscan.HandleBioscanBasicStep(ctx, b, r.stateManager, chatID, text) {
			// Опросник завершён последним ответом - запускаем генерацию
			// отчёта (используем зависимости роутера: сервис ИИ и т.п.).
			r.finalizeBioscanBasicReport(ctx, b, chatID)
		}
		return true
	}

	// Кнопка «⬅️ Назад» должна дойти до handleBack, а не трактоваться как
	// ввод очередного шага (иначе в фото-состояниях бот требует «пришлите
	// фото», а на экране подтверждения - «выберите действие»).
	if text == locales.BtnBack {
		return false
	}
	// Кнопка «❌ Отмена» должна дойти до handleCancel (выход из опросника),
	// а не сохраняться как ответ на вопрос.
	if text == locales.BtnCancel {
		return false
	}

	// Вопросы опросника Bioscan PRO (образ жизни / спорт / здоровье), которые
	// идут после цели и до загрузки 4 фотографий. Возвращает true, если
	// текущее состояние - один из вопросов и ввод обработан.
	if bioscan.HandleBioscanQuestionnaireState(ctx, b, r.stateManager, chatID, text) {
		return true
	}

	switch state {
	case states.StateWaitingBioscanName:
		log.Printf(locales.LogProcessingBioscanName, chatID)
		bioscan.HandleBioscanName(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanAge:
		log.Printf(locales.LogProcessingBioscanAge, chatID)
		bioscan.HandleBioscanAge(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanHeight:
		log.Printf(locales.LogProcessingBioscanHeight, chatID)
		bioscan.HandleBioscanHeight(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanWeight:
		log.Printf(locales.LogProcessingBioscanWeight, chatID)
		bioscan.HandleBioscanWeight(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanGoal:
		log.Printf(locales.LogProcessingBioscanGoal, chatID)
		bioscan.HandleBioscanGoal(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanPhoto1,
		states.StateWaitingBioscanPhoto2,
		states.StateWaitingBioscanPhoto3,
		states.StateWaitingBioscanPhoto4:

		log.Printf(locales.LogRouterBioscanPhoto, state, chatID)
		if len(update.Message.Photo) > 0 {
			bioscan.HandleBioscanPhoto(ctx, b, r.stateManager, chatID, update.Message.Photo, update.Message.ID)
		} else {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgBioscanPhotoRequired,
			})
		}
		return true

	case states.StateWaitingBioscanConfirm:
		log.Printf(locales.LogRouterBioscanConfirm, chatID, text)
		switch text {
		case locales.BtnBioscanConfirm:
			bioscan.ProcessBioscanWithPhotos(ctx, b, r.stateManager, r.analysisService, r.pdfConverter, r.uploadDir, r.stickerID, chatID, r.appStorage, r.monitorRepo, r.webAppURL)
		case locales.BtnBioscanRestart:
			bioscan.StartBioscanFlow(ctx, b, r.stateManager, chatID)
		default:
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgBioscanConfirmAction,
			})
		}
		return true
	}

	return false
}

// finalizeBioscanBasicReport - генерация и отправка отчёта базового Bioscan
// по собранным ответам мини-опросника (без фото: текстовая модель строит
// отчёт по замерам). Использует зависимости роутера (сервис ИИ и т.п.).
func (r *router) finalizeBioscanBasicReport(ctx context.Context, b *tgbot.Bot, chatID int64) {
	sm := r.stateManager
	gender := sm.GetUserData(chatID, "bioscan_basic_gender")
	age := sm.GetUserData(chatID, "bioscan_basic_age")
	height := sm.GetUserData(chatID, "bioscan_basic_height")
	weight := sm.GetUserData(chatID, "bioscan_basic_weight")
	goal := sm.GetUserData(chatID, "bioscan_basic_goal")
	activity := sm.GetUserData(chatID, "bioscan_basic_activity")

	contextInfo := fmt.Sprintf(
		"Пол: %s\nВозраст: %s\nРост: %s см\nВес: %s кг\nЦель: %s\nТренировки: %s",
		gender, age, height, weight, goal, activity,
	)
	log.Printf("[BIOSCAN] базовый: генерация отчёта chatID=%d", chatID)

	// Скачиваем присланное фото (если есть) - оно поступает в ИИ вместе с
	// данными опросника. OCR достанет текст со скриншотов умных весов/
	// фитнес-приложений, а замеры из опросника дадут модели реальные данные
	// для построения отчёта. Если фото не скачалось - отчёт строится по
	// опроснику (фото текстовая модель «не видит», но замеры есть).
	var photosData [][]byte
	photoID := sm.GetUserData(chatID, "bioscan_basic_photo_fileid")
	photoMsgID := sm.GetUserData(chatID, "bioscan_basic_photo_msgid")
	if photoID != "" {
		data, _, derr := helpers.DownloadFileByID(ctx, b, photoID)
		if derr != nil {
			log.Printf("[BIOSCAN] не удалось загрузить фото базового биоскана chatID=%d: %v", chatID, derr)
		} else {
			photosData = append(photosData, data)
			log.Printf("[BIOSCAN] фото загружено chatID=%d bytes=%d", chatID, len(data))
		}
	}

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, r.stickerID, nil)

	// Базовый Bioscan: фото + данные опросника уходят в ИИ вместе. Photos
	// могут быть пустыми (если фото не скачалось) - тогда отчёт строится
	// только по опроснику, что всё равно даёт реальный результат.
	result, err := r.analysisService.HandleBioscanText(ctx, photosData, "image/jpeg", contextInfo)
	if err != nil {
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		log.Printf(locales.LogBioscanError, err)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanProcessingError,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}
	log.Printf("[BIOSCAN] ИИ вернул результат chatID=%d len=%d", chatID, len(result))

	helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)

	// Удаляем исходное фото из чата (приватность: исходный материал не
	// остаётся в истории и не хранится ботом). FileID сохранён для
	// возможного повторного скачивания, но само сообщение убираем.
	if photoMsgID != "" {
		if id, cerr := strconv.Atoi(photoMsgID); cerr == nil && id != 0 {
			helpers.DeleteMessage(ctx, b, chatID, id)
		}
	}

	if strings.TrimSpace(result) != "" {
		log.Printf("[BIOSCAN] отправка результата в чат chatID=%d len=%d", chatID, len(result))
		helpers.SendLongMessagePlain(ctx, b, chatID, result)
	} else {
		log.Printf("[BIOSCAN] ИИ вернул ПУСТОЙ результат chatID=%d", chatID)
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicDone + "\n\n" + locales.MsgMaterialsDeleted,
		ReplyMarkup: keyboards.MainMenuInline(),
	})

	log.Printf("[BIOSCAN] обработка завершена chatID=%d", chatID)

	title := locales.MsgBioscanBasicTitle
	if r.monitorRepo != nil && strings.TrimSpace(result) != "" {
		entry := &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "bioscan",
			Title:      title,
			Date:       time.Now(),
			JsonData:   fmt.Sprintf(`{"title":%q,"note":%q}`, title, result),
			ReportHTML: helpers.PlainResultHTML(title, result),
		}
		if err := r.monitorRepo.SaveResult(ctx, entry); err != nil {
			log.Printf("[BIOSCAN] не удалось сохранить базовый биоскан chatID=%d: %v", chatID, err)
		} else {
			log.Printf("[BIOSCAN] базовый биоскан сохранён chatID=%d", chatID)
		}
	}

	helpers.SendSavedToSummary(ctx, b, chatID, r.webAppURL)
}
