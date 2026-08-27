package bioscan

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// Базовый (бесплатный) Bioscan: фото (шаг 1) + мини-опросник (пол, возраст,
// рост, вес, цель, тренировки) -> текстовый отчёт в чат. Фото - первый шаг:
// пользователь присылает снимок фигуры ИЛИ скриншот показателей умных весов/
// фитнес-приложения. YandexGPT - текстовая модель, поэтому «голое» фото фигуры
// OCR не читает (0 символов); но скриншоты весов/приложений дают текст, который
// модель учитывает. Реальные замеры для отчёта (рост/вес/возраст и т.д.)
// приходят из мини-опросника, идущего сразу за фото. Шаги опросника идут в
// едином состоянии StateWaitingBioscanBasicQ, текущий шаг хранится в user-data
// (bioscan_basic_step). FileID присланного фото и ID его сообщения хранятся в
// user-data (bioscan_basic_photo_fileid / _msgid) для скачивания и удаления.

// Ключи user-data базового Bioscan.
const (
	basicStepKey     = "bioscan_basic_step"
	basicGenderKey   = "bioscan_basic_gender"
	basicAgeKey      = "bioscan_basic_age"
	basicHeightKey   = "bioscan_basic_height"
	basicWeightKey   = "bioscan_basic_weight"
	basicGoalKey     = "bioscan_basic_goal"
	basicActivityKey = "bioscan_basic_activity"
	basicPhotoFileID = "bioscan_basic_photo_fileid"
	basicPhotoMsgID  = "bioscan_basic_photo_msgid"
)

// basicStepStart - первый вопрос опросника (пол, кнопки).
const basicStepStart = 0

// StartBioscanBasicFlow - начало БАЗОВОГО (бесплатного) Bioscan: запускает
// поток с приёма фото. Проверка соглашения выполняется вызывающим
// (router.handleBioscanBasicStart через agreementStorage.IsAgreed).
func StartBioscanBasicFlow(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	ResetBioscanData(sm, chatID)

	sm.SetState(chatID, states.StateWaitingBioscanBasicPhoto)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackQuestionInline(),
	})
	log.Printf("[BIOSCAN] базовый: старт (фото) chatID=%d", chatID)
}

// HandleBioscanBasicPhoto - приём фото пользователя (шаг 1). Сохраняет
// наибольшее фото (FileID) и ID сообщения (для удаления из чата после
// обработки), подтверждает приём и запускает мини-опросник (вопрос о поле).
func HandleBioscanBasicPhoto(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, photos []models.PhotoSize, msgID int) {
	if len(photos) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanPhotoRequired,
			ReplyMarkup: keyboards.BackQuestionInline(),
		})
		return
	}

	// Берём самое крупное из доступных разрешений (последнее в массиве).
	photo := photos[len(photos)-1]
	sm.SetUserData(chatID, basicPhotoFileID, photo.FileID)
	sm.SetUserData(chatID, basicPhotoMsgID, strconv.Itoa(msgID))
	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(basicStepStart))

	log.Printf("[BIOSCAN] базовый: фото получено chatID=%d msgID=%d fileID_len=%d", chatID, msgID, len(photo.FileID))

	// Подтверждаем приём и сразу переходим к опроснику (фото уже сохранено).
	sm.SetState(chatID, states.StateWaitingBioscanBasicQ)
	if sm.GetUserData(chatID, basicStepKey) != "" {
		// Шаг уже выставлен (пользователь выбрал «Использовать» сохранённый
		// профиль и пропустил демографические вопросы) - сразу к этому шагу
		// (обычно цель, step 4), не задавая вопрос про пол заново.
		step, _ := strconv.Atoi(sm.GetUserData(chatID, basicStepKey))
		askBioscanBasicStep(ctx, b, sm, chatID, step)
		return
	}
	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(basicStepStart))
	askBioscanBasicStep(ctx, b, sm, chatID, basicStepStart)
}

// AskBioscanBasicStep - экспортируемая обёртка askBioscanBasicStep (задаёт
// вопрос текущего шага мини-опросника базового Bioscan). Используется
// роутером, когда демографические данные уже известны из профиля.
func AskBioscanBasicStep(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, step int) {
	askBioscanBasicStep(ctx, b, sm, chatID, step)
}

// bioscanBasicGenderKeyboard - inline-кнопки выбора пола.
func bioscanBasicGenderKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnBioscanBasicMale, CallbackData: "bioscan_basic_gender_m"}},
			{{Text: locales.BtnBioscanBasicFemale, CallbackData: "bioscan_basic_gender_f"}},
		},
	}
}

// bioscanBasicGoalKeyboard - inline-кнопки выбора цели.
func bioscanBasicGoalKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnBioscanBasicGoalMass, CallbackData: "bioscan_basic_goal_mass"}},
			{{Text: locales.BtnBioscanBasicGoalCut, CallbackData: "bioscan_basic_goal_cut"}},
			{{Text: locales.BtnBioscanBasicGoalKeep, CallbackData: "bioscan_basic_goal_keep"}},
			{{Text: locales.BtnBioscanBasicGoalEndure, CallbackData: "bioscan_basic_goal_endure"}},
			{{Text: locales.BtnBioscanBasicGoalFlex, CallbackData: "bioscan_basic_goal_flex"}},
		},
	}
}

// askBioscanBasicStep - задаёт вопрос текущего шага опросника.
func askBioscanBasicStep(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, step int) {
	var text string
	var kb models.ReplyMarkup = keyboards.BackQuestionInline()
	switch step {
	case 0:
		text = locales.MsgBioscanBasicQGender
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: bioscanBasicGenderKeyboard(),
		})
		return
	case 1:
		text = locales.MsgBioscanBasicQAge
	case 2:
		text = locales.MsgBioscanBasicQHeight
	case 3:
		text = locales.MsgBioscanBasicQWeight
	case 4:
		text = locales.MsgBioscanBasicQGoal
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: bioscanBasicGoalKeyboard(),
		})
		return
	case 5:
		text = locales.MsgBioscanBasicQActivity
	default:
		text = locales.MsgBioscanBasicQActivity
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: kb,
	})
}

// HandleBioscanBasicGender - выбор пола (inline-кнопка). Сохраняет ответ и
// переходит к следующему шагу (возраст).
func HandleBioscanBasicGender(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, gender string) {
	gender = normalizeGender(gender)
	sm.SetUserData(chatID, basicGenderKey, gender)
	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(1))
	log.Printf("[BIOSCAN] базовый: пол=%s chatID=%d", gender, chatID)
	askBioscanBasicStep(ctx, b, sm, chatID, 1)
}

// normalizeGender приводит текст кнопки к чистому значению.
func normalizeGender(g string) string {
	switch strings.TrimSpace(g) {
	case "Мужской", "мужской", "М", "м", "👨 Мужской":
		return "Мужской"
	case "Женский", "женский", "Ж", "ж", "👩 Женский":
		return "Женский"
	default:
		return strings.TrimSpace(g)
	}
}

// HandleBioscanBasicGoal - выбор цели (inline-кнопка). Сохраняет ответ и
// переходит к последнему шагу (тренировки).
func HandleBioscanBasicGoal(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, goal string) {
	goal = cleanGoalLabel(goal)
	sm.SetUserData(chatID, basicGoalKey, goal)
	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(5))
	log.Printf("[BIOSCAN] базовый: цель=%s chatID=%d", goal, chatID)
	askBioscanBasicStep(ctx, b, sm, chatID, 5)
}

// cleanGoalLabel приводит текст кнопки цели к чистому значению.
func cleanGoalLabel(btn string) string {
	switch strings.TrimSpace(btn) {
	case locales.BtnBioscanBasicGoalMass:
		return "набор мышечной массы"
	case locales.BtnBioscanBasicGoalCut:
		return "снижение веса"
	case locales.BtnBioscanBasicGoalKeep:
		return "поддержание формы"
	case locales.BtnBioscanBasicGoalEndure:
		return "развитие выносливости"
	case locales.BtnBioscanBasicGoalFlex:
		return "развитие гибкости"
	default:
		return strings.TrimSpace(btn)
	}
}

// HandleBioscanBasicStep - обработка свободного текста (возраст/рост/вес/
// тренировки) на текущем шаге опросника. Проверяет формат, сохраняет ответ,
// продвигает шаг. Возвращает true, если это был последний шаг (тренировки) и
// опросник завершён - тогда роутер запускает генерацию отчёта (использует
// сохранённое фото + ответы опросника).
func HandleBioscanBasicStep(
	ctx context.Context,
	b *tgbot.Bot,
	sm states.StateManager,
	chatID int64,
	text string,
) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	step, _ := strconv.Atoi(sm.GetUserData(chatID, basicStepKey))
	log.Printf("[BIOSCAN] базовый: шаг %d chatID=%d", step, chatID)

	switch step {
	case 1: // возраст
		v, ok := parseNumber(text, 5, 120)
		if !ok {
			sendBasicInvalid(ctx, b, chatID, locales.MsgBioscanBasicQAge)
			return false
		}
		sm.SetUserData(chatID, basicAgeKey, strconv.Itoa(int(v)))
		step = 2
	case 2: // рост
		v, ok := parseNumber(text, 50, 260)
		if !ok {
			sendBasicInvalid(ctx, b, chatID, locales.MsgBioscanBasicQHeight)
			return false
		}
		sm.SetUserData(chatID, basicHeightKey, formatNumber(v))
		step = 3
	case 3: // вес
		v, ok := parseNumber(text, 20, 400)
		if !ok {
			sendBasicInvalid(ctx, b, chatID, locales.MsgBioscanBasicQWeight)
			return false
		}
		sm.SetUserData(chatID, basicWeightKey, formatNumber(v))
		step = 4
	case 5: // тренировки - последний шаг -> опросник завершён
		sm.SetUserData(chatID, basicActivityKey, strings.TrimSpace(text))
		sm.SetState(chatID, states.StateIdle)
		sm.SetUserData(chatID, basicStepKey, "")
		log.Printf("[BIOSCAN] базовый: опросник завершён chatID=%d", chatID)
		return true
	default:
		askBioscanBasicStep(ctx, b, sm, chatID, step)
		return false
	}

	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(step))
	askBioscanBasicStep(ctx, b, sm, chatID, step)
	return false
}

// sendBasicInvalid - подсказка о некорректном вводе + повтор вопроса.
func sendBasicInvalid(ctx context.Context, b *tgbot.Bot, chatID int64, prompt string) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicInvalid + "\n\n" + prompt,
		ReplyMarkup: keyboards.BackQuestionInline(),
	})
}

// parseNumber парсит число (целое или десятичное, с точкой/запятой) из текста
// и проверяет диапазон.
func parseNumber(text string, min, max float64) (float64, bool) {
	t := strings.TrimSpace(text)
	t = strings.ReplaceAll(t, ",", ".")
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v < min || v > max {
		return 0, false
	}
	return v, true
}

// formatNumber - компактное представление числа (без лишних нулей).
func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// BackBioscanBasic - шаг "Назад" внутри базового Bioscan (от фото к выходу из
// потока, либо внутри опросника - к предыдущему вопросу). Если вопрос первый
// (пол) - выход из опросника в главное меню. Если текущее состояние - шаг с
// фото - сразу выход из потока.
func BackBioscanBasic(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	if sm.GetState(chatID) == states.StateWaitingBioscanBasicPhoto {
		ResetBioscanData(sm, chatID)
		sm.SetState(chatID, states.StateIdle)
		sm.SetUserData(chatID, "analysis_type", "")
		sm.SetUserData(chatID, "analysis_subtype", "")
		log.Printf("[BIOSCAN] базовый: выход (фото) chatID=%d", chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToAnalysisType,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}

	step, _ := strconv.Atoi(sm.GetUserData(chatID, basicStepKey))
	step--
	if step < basicStepStart {
		ResetBioscanData(sm, chatID)
		sm.SetState(chatID, states.StateIdle)
		sm.SetUserData(chatID, "analysis_type", "")
		sm.SetUserData(chatID, "analysis_subtype", "")
		log.Printf("[BIOSCAN] базовый: выход из опросника chatID=%d", chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToAnalysisType,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}
	sm.SetUserData(chatID, basicStepKey, strconv.Itoa(step))
	log.Printf("[BIOSCAN] базовый: назад к шагу %d chatID=%d", step, chatID)
	askBioscanBasicStep(ctx, b, sm, chatID, step)
}

// CancelBioscanBasic - отмена базового Bioscan.
func CancelBioscanBasic(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	ResetBioscanData(sm, chatID)
	sm.SetState(chatID, states.StateIdle)
	sm.SetUserData(chatID, "analysis_type", "")
	sm.SetUserData(chatID, "analysis_subtype", "")
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanBasicCanceled,
		ReplyMarkup: keyboards.MainMenuInline(),
	})
}
