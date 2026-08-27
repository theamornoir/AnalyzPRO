package router

import (
	"context"
	"fmt"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// profileFlowKey - ключ user-data для запоминания, какой поток вызвал экран
// подтверждения профиля (значения: "health" / "bioscan_pro" /
// "bioscan_basic"). Нужен обработчикам profile_use / profile_change, чтобы
// знать, в какой опросник возвращаться и какие ключи подставить.
const profileFlowKey = "profile_flow"

// tryProfileConfirm - если у пользователя уже есть сохранённый профиль с
// заполненными ключевыми полями (возраст, пол, рост, вес), показывает экран
// «Данные уже известны?» с inline-кнопками «Использовать» / «Изменить» и
// переводит в StateWaitingProfileConfirm. Возвращает true, если экран
// показан (вызывающий обработчик старта опросника должен прервать запуск).
// Если профиля нет или он неполный - возвращает false (вызывающий запускает
// опросник заново). Это реализует требование «не переспрашивать известные
// данные»: бот не задаёт вопросы имя/возраст/пол/рост/вес повторно.
func (r *router) tryProfileConfirm(ctx context.Context, b *tgbot.Bot, chatID int64, flow string) bool {
	if r.appStorage == nil {
		return false
	}
	profile, err := r.appStorage.Users.GetProfile(ctx, chatID)
	if err != nil {
		log.Printf("[PROFILE] не удалось прочитать профиль chatID=%d: %v", chatID, err)
		return false
	}
	// Профиль считается «известным», только если заполнены ключевые
	// демографические поля (именно они выводятся в сообщении и пропускаются).
	if profile == nil || profile.Age <= 0 || profile.Gender == "" || profile.Height <= 0 || profile.Weight <= 0 {
		return false
	}

	r.stateManager.SetUserData(chatID, profileFlowKey, flow)
	r.stateManager.SetState(chatID, states.StateWaitingProfileConfirm)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf(locales.MsgProfileKnown, profile.Age, profile.Gender, profile.Height, profile.Weight),
		ReplyMarkup: keyboards.ProfileConfirmMenu(),
		ParseMode:   "Markdown",
	})
	log.Printf("[PROFILE] подтверждение профиля показано chatID=%d flow=%s", chatID, flow)
	return true
}

// handleProfileUse - пользователь выбрал «Использовать» известные данные:
// подставляем их в user-data (пропускаем вопросы имя/возраст/пол/рост/вес) и
// переводим в состояние первого уникального вопроса потока.
func (r *router) handleProfileUse(ctx context.Context, b *tgbot.Bot, chatID int64) {
	flow := r.stateManager.GetUserData(chatID, profileFlowKey)
	profile, err := r.appStorage.Users.GetProfile(ctx, chatID)
	if err != nil || profile == nil {
		// На всякий случай: если профиль вдруг исчез - запускаем заново.
		r.startQuestionnaireByFlow(ctx, b, chatID, flow)
		return
	}

	switch flow {
	case "health":
		// Подставляем известные данные (без префикса).
		r.stateManager.SetUserData(chatID, "name", profile.Name)
		r.stateManager.SetUserData(chatID, "age", strconv.Itoa(profile.Age))
		r.stateManager.SetUserData(chatID, "gender", profile.Gender)
		r.stateManager.SetUserData(chatID, "height", strconv.Itoa(profile.Height))
		r.stateManager.SetUserData(chatID, "weight", strconv.Itoa(profile.Weight))
		r.stateManager.SetUserData(chatID, "analysis_type", "extended")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")
		r.stateManager.SetState(chatID, states.StateWaitingGoal)
		userdata.NewUserDataCollector(r.stateManager).SendGoalQuestion(ctx, b, chatID)
	case "bioscan_pro":
		r.stateManager.SetUserData(chatID, "bioscan_name", profile.Name)
		r.stateManager.SetUserData(chatID, "bioscan_age", strconv.Itoa(profile.Age))
		r.stateManager.SetUserData(chatID, "bioscan_gender", profile.Gender)
		r.stateManager.SetUserData(chatID, "bioscan_height", strconv.Itoa(profile.Height))
		r.stateManager.SetUserData(chatID, "bioscan_weight", strconv.Itoa(profile.Weight))
		r.stateManager.SetState(chatID, states.StateWaitingBioscanGoal)
		bioscan.AskBioscanProGoal(ctx, b, chatID)
	case "bioscan_basic":
		// Демографические данные подставляем; цель тоже (пользователь её
		// подтвердит вопросом ниже). Шаг прыгает сразу к цели (4) - пол/
		// возраст/рост/вес пропущены. Фото пересобирается заново (оно не
		// хранится ботом), поэтому состояние - приём фото.
		r.stateManager.SetUserData(chatID, "bioscan_basic_gender", profile.Gender)
		r.stateManager.SetUserData(chatID, "bioscan_basic_age", strconv.Itoa(profile.Age))
		r.stateManager.SetUserData(chatID, "bioscan_basic_height", strconv.Itoa(profile.Height))
		r.stateManager.SetUserData(chatID, "bioscan_basic_weight", strconv.Itoa(profile.Weight))
		r.stateManager.SetUserData(chatID, "bioscan_basic_goal", profile.Goal)
		r.stateManager.SetUserData(chatID, "bioscan_basic_step", "4")
		r.stateManager.SetState(chatID, states.StateWaitingBioscanBasicPhoto)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanBasicIntro,
			ReplyMarkup: keyboards.BackQuestionInline(),
		})
	}

	r.stateManager.SetUserData(chatID, profileFlowKey, "")
	log.Printf("[PROFILE] использованы сохранённые данные chatID=%d flow=%s", chatID, flow)
}

// handleProfileChange - пользователь выбрал «Изменить»: запускаем опросник
// заново (сбрасываем profile_flow и стартуем с первого вопроса).
func (r *router) handleProfileChange(ctx context.Context, b *tgbot.Bot, chatID int64) {
	flow := r.stateManager.GetUserData(chatID, profileFlowKey)
	r.stateManager.SetUserData(chatID, profileFlowKey, "")
	r.startQuestionnaireByFlow(ctx, b, chatID, flow)
	log.Printf("[PROFILE] данные будут введены заново chatID=%d flow=%s", chatID, flow)
}

// startQuestionnaireByFlow - перезапускает опросник нужного потока с начала
// (используется при «Изменить» и как запасной путь, если профиль исчез).
func (r *router) startQuestionnaireByFlow(ctx context.Context, b *tgbot.Bot, chatID int64, flow string) {
	switch flow {
	case "health":
		r.stateManager.SetUserData(chatID, "analysis_type", "extended")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")
		r.stateManager.SetState(chatID, states.StateWaitingName)
		r.deleteHubBlock(ctx, b, chatID)
		userdata.NewUserDataCollector(r.stateManager).SendStep(ctx, b, chatID, states.StateWaitingName, locales.MsgExtendedAnalysisIntro)
	case "bioscan_pro":
		r.stateManager.SetState(chatID, states.StateWaitingBioscanName)
		r.editNavMessage(ctx, b, chatID, locales.MsgBioscanIntro, keyboards.BackInline())
	case "bioscan_basic":
		bioscan.StartBioscanBasicFlow(ctx, b, r.stateManager, chatID)
	}
}

// saveProfile - сохраняет постоянный профиль пользователя по собранным
// данным текущего потока (чтобы при следующем запуске не переспрашивать).
// prefix определяет префикс ключей user-data: "" для health, "bioscan_" для
// PRO, "bioscan_basic_" для базового. Сохраняем, только если заполнены
// ключевые поля (возраст/пол/рост/вес) - иначе профиль «битый» и лучше не
// перезаписывать хороший старый.
func (r *router) saveProfile(ctx context.Context, chatID int64, prefix string) {
	if r.appStorage == nil {
		return
	}
	get := func(k string) string { return r.stateManager.GetUserData(chatID, prefix+k) }

	name := get("name")
	age, _ := strconv.Atoi(get("age"))
	gender := get("gender")
	height, _ := strconv.Atoi(get("height"))
	weight, _ := strconv.Atoi(get("weight"))
	goal := get("goal")

	if age <= 0 || gender == "" || height <= 0 || weight <= 0 {
		return
	}

	profile := &sm.Profile{
		TelegramID: chatID,
		Name:       name,
		Age:        age,
		Gender:     gender,
		Height:     height,
		Weight:     weight,
		Goal:       goal,
	}
	if err := r.appStorage.Users.UpsertProfile(ctx, profile); err != nil {
		log.Printf("[PROFILE] не удалось сохранить профиль chatID=%d: %v", chatID, err)
		return
	}
	log.Printf("[PROFILE] профиль сохранён chatID=%d prefix=%q", chatID, prefix)
}
