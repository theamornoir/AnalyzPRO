package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
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
		log.Printf("[PROFILE] tryProfileConfirm: appStorage==nil, пропускаем для chatID=%d flow=%s", chatID, flow)
		return false
	}
	profile, err := r.appStorage.Users.GetProfile(ctx, chatID)
	if err != nil {
		log.Printf("[PROFILE] не удалось прочитать профиль chatID=%d: %v", chatID, err)
		return false
	}

	// Бот и дашборд «Мой профиль» хранят профиль в ДВУХ независимых местах:
	// user_profiles (читает бот) и анкета type="questionnaire" (читает
	// дашборд). Они легко рассинхронизируются (после /resetme, при молчаливом
	// сбое записи и т.п.), и тогда дашборд видит профиль, а бот - нет и
	// переспрашивает имя/возраст. resolveProfile сводит оба источника воедино
	// (источник истины - анкета Mini App, её правит пользователь) и
	// синхронизирует результат обратно в user_profiles.
	resolved, source := r.resolveProfile(ctx, chatID, profile)
	if resolved == nil {
		log.Printf("[PROFILE] tryProfileConfirm: chatID=%d flow=%s профиль НЕ найден (ни user_profiles, ни анкета) -> запускаем опросник", chatID, flow)
		return false
	}
	if resolved.Age <= 0 || resolved.Gender == "" || resolved.Height <= 0 || resolved.Weight <= 0 {
		log.Printf("[PROFILE] tryProfileConfirm: chatID=%d flow=%s профиль неполный (источник=%s) age=%d gender=%q height=%d weight=%d -> запускаем опросник",
			chatID, flow, source, resolved.Age, resolved.Gender, resolved.Height, resolved.Weight)
		return false
	}

	// Синхронизируем слитый профиль обратно в user_profiles (если ещё не
	// там), чтобы handleProfileUse/saveProfile читали консистентно и бот не
	// переспрашивал уже известные данные.
	if serr := r.appStorage.Users.UpsertProfile(ctx, resolved); serr != nil {
		log.Printf("[PROFILE] tryProfileConfirm: chatID=%d не удалось синхронизировать профиль в user_profiles: %v", chatID, serr)
	}
	log.Printf("[PROFILE] tryProfileConfirm: chatID=%d flow=%s профиль резолвлен из %s age=%d gender=%q height=%d weight=%d name=%q",
		chatID, flow, source, resolved.Age, resolved.Gender, resolved.Height, resolved.Weight, resolved.Name)

	r.stateManager.SetUserData(chatID, profileFlowKey, flow)
	r.stateManager.SetState(chatID, states.StateWaitingProfileConfirm)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf(locales.MsgProfileKnown, resolved.Age, resolved.Gender, resolved.Height, resolved.Weight),
		ReplyMarkup: keyboards.ProfileConfirmMenu(),
		ParseMode:   "Markdown",
	})
	log.Printf("[PROFILE] подтверждение профиля показано chatID=%d flow=%s", chatID, flow)
	return true
}

// resolveProfile сводит профиль из user_profiles и анкеты Mini App
// (type="questionnaire") в единый профиль, устраняя рассинхрон двух
// хранилищ. Источник истины - анкета (её правит пользователь в «Мой
// профиль»), поэтому начинаем с неё и докидываем недостающее из user_profiles.
// Возвращает слитый профиль и метку источника (для логов); (nil, "") - если
// данных нет ни в одном хранилище.
func (r *router) resolveProfile(ctx context.Context, chatID int64, fromUsers *sm.Profile) (*sm.Profile, string) {
	var qp *sm.Profile
	if r.monitorRepo != nil {
		qp = r.loadQuestionnaireProfile(ctx, chatID)
	}
	// Нет данных нигде.
	if fromUsers == nil && qp == nil {
		return nil, ""
	}
	// Только user_profiles.
	if qp == nil {
		return fromUsers, "user_profiles"
	}
	// Только анкета.
	if fromUsers == nil {
		return qp, "questionnaire"
	}
	// Оба есть: анкета приоритетна, докидываем недостающее из user_profiles.
	merged := *qp
	if merged.Name == "" {
		merged.Name = fromUsers.Name
	}
	if merged.Age <= 0 {
		merged.Age = fromUsers.Age
	}
	if merged.Gender == "" {
		merged.Gender = fromUsers.Gender
	}
	if merged.Height <= 0 {
		merged.Height = fromUsers.Height
	}
	if merged.Weight <= 0 {
		merged.Weight = fromUsers.Weight
	}
	if merged.Goal == "" {
		merged.Goal = fromUsers.Goal
	}
	merged.TelegramID = chatID
	return &merged, "questionnaire+user_profiles"
}

// loadQuestionnaireProfile читает последнюю анкету (type="questionnaire") из
// истории мониторинга и извлекает из неё демографические поля профиля.
// Формат JSON анкеты совпадает с тем, что пишут dashboard.SaveProfile и
// bot.saveProfile: {"profile":{"name","age","gender","height","weight",...}}.
// Возвращает nil, если анкеты нет или JSON невалиден.
func (r *router) loadQuestionnaireProfile(ctx context.Context, telegramID int64) *sm.Profile {
	entries, _, err := r.monitorRepo.ListHistory(ctx, telegramID, "questionnaire", 1, 1)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var doc struct {
		Profile struct {
			Name   string `json:"name"`
			Age    int    `json:"age"`
			Gender string `json:"gender"`
			Height int    `json:"height"`
			Weight int    `json:"weight"`
			Goal   string `json:"goal"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(entries[0].JsonData), &doc); err != nil {
		log.Printf("[PROFILE] loadQuestionnaireProfile: не удалось распарсить анкету user=%d: %v", telegramID, err)
		return nil
	}
	return &sm.Profile{
		TelegramID: telegramID,
		Name:       strings.TrimSpace(doc.Profile.Name),
		Age:        doc.Profile.Age,
		Gender:     strings.TrimSpace(doc.Profile.Gender),
		Height:     doc.Profile.Height,
		Weight:     doc.Profile.Weight,
		Goal:       strings.TrimSpace(doc.Profile.Goal),
	}
}

// handleProfileUse - пользователь выбрал «Использовать» известные данные:
// подставляем их в user-data (пропускаем вопросы имя/возраст/пол/рост/вес) и
// переводим в состояние первого уникального вопроса потока.
func (r *router) handleProfileUse(ctx context.Context, b *tgbot.Bot, chatID int64) {
	flow := r.stateManager.GetUserData(chatID, profileFlowKey)
	profile, err := r.appStorage.Users.GetProfile(ctx, chatID)
	if err != nil || profile == nil {
		// На всякий случай: если профиль вдруг исчез - запускаем заново.
		log.Printf("[PROFILE] handleProfileUse: профиль не найден chatID=%d flow=%s (err=%v) -> запуск опросника заново", chatID, flow, err)
		r.startQuestionnaireByFlow(ctx, b, chatID, flow)
		return
	}
	log.Printf("[PROFILE] handleProfileUse: chatID=%d flow=%s подставляем name=%q age=%d gender=%q height=%d weight=%d goal=%q (пропуск вопросов)",
		chatID, flow, profile.Name, profile.Age, profile.Gender, profile.Height, profile.Weight, profile.Goal)

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
		// Демография (имя/пол/возраст/рост+вес) подставлена из профиля ->
		// пропущено 4 ведущих вопроса. Смещение для прогресс-бара: опросник
		// начинается с цели, поэтому первый реальный вопрос = «1 из 3».
		r.stateManager.SetUserData(chatID, userdata.HealthSkipKey, "4")
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
		// Чистый старт (без подстановки профиля): никакие вопросы не
		// пропущены, счётчик «Вопрос N из 7» абсолютный.
		r.stateManager.SetUserData(chatID, userdata.HealthSkipKey, "0")
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
		log.Printf("[PROFILE] saveProfile: пропуск сохранения chatID=%d prefix=%q (неполные поля: age=%d gender=%q height=%d weight=%d)",
			chatID, prefix, age, gender, height, weight)
		return
	}

	log.Printf("[PROFILE] saveProfile: chatID=%d prefix=%q собраны поля name=%q age=%d gender=%q height=%d weight=%d goal=%q",
		chatID, prefix, name, age, gender, height, weight, goal)

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
	log.Printf("[PROFILE] профиль сохранён chatID=%d prefix=%q (user_profiles)", chatID, prefix)

	// Мост в Mini App «Мой профиль»: дублируем анкету в историю (тип
	// "questionnaire"), откуда дашборд читает карточку профиля. buildMetrics
	// берёт name/age/gender/height/weight ИМЕННО из этой записи
	// (ProfileMissing = true, когда записи-анкеты нет). Без этого новый
	// пользователь, заполнивший опросник в боте (Bioscan PRO / базовый
	// Bioscan / Оценка здоровья), НЕ увидел бы свой профиль в Mini App.
	// Если monitorRepo==nil - пропускаем (запись user_profiles уже есть,
	// она нужна боту для подстановки данных в следующий раз).
	if r.monitorRepo != nil {
		payload := map[string]interface{}{
			"profile": map[string]interface{}{
				"name":   name,
				"age":    age,
				"gender": gender,
				"height": height,
				"weight": weight,
			},
			"recommendations": []string{
				"Регулярно загружайте анализы и биосканы, чтобы отслеживать динамику здоровья.",
			},
		}
		payloadBytes, _ := json.Marshal(payload)
		entry := &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "questionnaire",
			Title:      "Профиль пользователя",
			Date:       time.Now(),
			JsonData:   string(payloadBytes),
		}
		if err := r.monitorRepo.SaveResult(ctx, entry); err != nil {
			log.Printf("[PROFILE] не удалось продублировать профиль в историю (questionnaire) chatID=%d: %v", chatID, err)
			return
		}
		log.Printf("[PROFILE] профиль продублирован в историю (questionnaire) chatID=%d prefix=%q (bridge -> дашборд)", chatID, prefix)
	}
}

// safe* — безопасные акцессоры профиля для логирования: profile может быть
// nil (новый пользователь), прямое обращение к полям вызвало бы панику.
func safeName(p *sm.Profile) string {
	if p == nil {
		return ""
	}
	return p.Name
}
func safeAge(p *sm.Profile) int {
	if p == nil {
		return 0
	}
	return p.Age
}
func safeGender(p *sm.Profile) string {
	if p == nil {
		return ""
	}
	return p.Gender
}
func safeHeight(p *sm.Profile) int {
	if p == nil {
		return 0
	}
	return p.Height
}
func safeWeight(p *sm.Profile) int {
	if p == nil {
		return 0
	}
	return p.Weight
}
