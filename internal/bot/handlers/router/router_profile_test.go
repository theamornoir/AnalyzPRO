package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// TestProfileConfirmShownAndUsed - проверяет, что при запуске опросника с
// уже известным профилем бот показывает экран «Данные уже известны?», а по
// кнопке «Использовать» подставляет сохранённые данные (пол/возраст/рост/вес)
// и пропускает демографические вопросы, переходя сразу к приёму фото (базовый
// Bioscan не требует Premium - удобный путь для проверки без гейта).
func TestProfileConfirmShownAndUsed(t *testing.T) {
	chatID := int64(556001)

	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})
	st := storage.NewMockStorage()
	_ = st.Users.UpsertProfile(context.Background(), &sm.Profile{
		TelegramID: chatID,
		Name:       "Влад",
		Age:        27,
		Gender:     "Мужской",
		Height:     180,
		Weight:     70,
		Goal:       "поддержание формы",
	})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()
	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		smgr, nil, nil, nil, "", "", int64(0), agr, pay, st, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)
	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	// Запуск базового Bioscan (не требует Premium).
	pressCallback("section_bioscan_basic")

	// Профиль известен -> показан экран подтверждения.
	if got := smgr.GetState(chatID); got != states.StateWaitingProfileConfirm {
		t.Fatalf("ожидалось состояние ProfileConfirm, получили %q", got)
	}
	mt.mu.Lock()
	last := ""
	if len(mt.sent) > 0 {
		last = mt.sent[len(mt.sent)-1]
	}
	markup := mt.lastSendMarkup
	mt.mu.Unlock()
	if !strings.Contains(last, "Данные уже известны") {
		t.Fatalf("экран подтверждения не содержит «Данные уже известны»: %q", last)
	}
	if !strings.Contains(markup, "profile_use") || !strings.Contains(markup, "profile_change") {
		t.Fatalf("клавиатура подтверждения не содержит profile_use/profile_change: %q", markup)
	}

	// «Использовать» -> пропуск демографии, переход к приёму фото.
	pressCallback("profile_use")
	if got := smgr.GetState(chatID); got != states.StateWaitingBioscanBasicPhoto {
		t.Fatalf("после profile_use ожидалось состояние приёма фото, получили %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_gender"); got != "Мужской" {
		t.Errorf("profile_use не подставил пол: %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_age"); got != "27" {
		t.Errorf("profile_use не подставил возраст: %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_height"); got != "180" {
		t.Errorf("profile_use не подставил рост: %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_weight"); got != "70" {
		t.Errorf("profile_use не подставил вес: %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_step"); got != "4" {
		t.Errorf("profile_use не пропустил демографию (step=%q, ожидался 4 - цель)", got)
	}
}

// TestProfileChangeRestarts - проверяет, что по кнопке «Изменить» опросник
// запускается заново (данные профиля очищаются, состояние - приём фото).
func TestProfileChangeRestarts(t *testing.T) {
	chatID := int64(556002)

	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})
	st := storage.NewMockStorage()
	_ = st.Users.UpsertProfile(context.Background(), &sm.Profile{
		TelegramID: chatID, Age: 27, Gender: "Мужской", Height: 180, Weight: 70,
	})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()
	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		smgr, nil, nil, nil, "", "", int64(0), agr, pay, st, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)
	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	pressCallback("section_bioscan_basic")
	pressCallback("profile_change")

	if got := smgr.GetState(chatID); got != states.StateWaitingBioscanBasicPhoto {
		t.Fatalf("после profile_change ожидалось состояние приёма фото, получили %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_gender"); got != "" {
		t.Errorf("profile_change не очистил подставленный пол: %q", got)
	}
	if got := smgr.GetUserData(chatID, "bioscan_basic_age"); got != "" {
		t.Errorf("profile_change не очистил подставленный возраст: %q", got)
	}
}

// TestSaveProfileUpserts - проверяет, что saveProfile сохраняет (и
// обновляет) постоянный профиль, но НЕ перезаписывает хороший профиль
// неполными данными.
func TestSaveProfileUpserts(t *testing.T) {
	chatID := int64(556009)

	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	st := storage.NewMockStorage()
	r := &router{stateManager: smgr, appStorage: st}

	smgr.SetUserData(chatID, "name", "Влад")
	smgr.SetUserData(chatID, "age", "27")
	smgr.SetUserData(chatID, "gender", "Мужской")
	smgr.SetUserData(chatID, "height", "180")
	smgr.SetUserData(chatID, "weight", "70")
	smgr.SetUserData(chatID, "goal", "поддержание формы")

	r.saveProfile(context.Background(), chatID, "")

	p, err := st.Users.GetProfile(context.Background(), chatID)
	if err != nil || p == nil {
		t.Fatalf("профиль не сохранён: %v", err)
	}
	if p.Age != 27 || p.Gender != "Мужской" || p.Height != 180 || p.Weight != 70 || p.Name != "Влад" {
		t.Errorf("сохранён неверный профиль: %+v", p)
	}

	// Неполные данные (нет возраста) - не должны перезаписать хороший профиль.
	smgr.SetUserData(chatID, "age", "")
	r.saveProfile(context.Background(), chatID, "")
	p2, _ := st.Users.GetProfile(context.Background(), chatID)
	if p2.Age != 27 {
		t.Errorf("saveProfile перезаписал хороший профиль пустым: %+v", p2)
	}
}

// TestSaveProfileWritesQuestionnaireHistory - проверяет, что bot.saveProfile
// дублирует анкету и в историю (тип "questionnaire"), откуда Mini App «Мой
// профиль» читает карточку профиля. Иначе пользователь, заполнивший опросник
// в боте, не увидел бы свой профиль в Mini App (дашборд считает
// ProfileMissing=true при отсутствии записи-анкеты).
func TestSaveProfileWritesQuestionnaireHistory(t *testing.T) {
	chatID := int64(556010)

	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	st := storage.NewMockStorage()
	monRepo := monitoring.NewMockRepository()
	r := &router{stateManager: smgr, appStorage: st, monitorRepo: monRepo}

	smgr.SetUserData(chatID, "name", "Влад")
	smgr.SetUserData(chatID, "age", "27")
	smgr.SetUserData(chatID, "gender", "Мужской")
	smgr.SetUserData(chatID, "height", "180")
	smgr.SetUserData(chatID, "weight", "70")

	r.saveProfile(context.Background(), chatID, "")

	// 1) user_profiles (для подстановки в боте при следующем запуске).
	if p, err := st.Users.GetProfile(context.Background(), chatID); err != nil || p == nil || p.Age != 27 {
		t.Fatalf("профиль в user_profiles не сохранён: %v %+v", err, p)
	}
	// 2) questionnaire-запись (для карточки профиля в Mini App).
	entries, _, err := monRepo.ListHistory(context.Background(), chatID, "questionnaire", 1, 10)
	if err != nil {
		t.Fatalf("ошибка чтения истории: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ожидалась ровно 1 questionnaire-запись, получили %d", len(entries))
	}
	if entries[0].Type != "questionnaire" {
		t.Fatalf("неверный тип записи: %q", entries[0].Type)
	}
	// JSON должен содержать вложенный profile с полями (формат, понимаемый
	// buildMetrics/parseReport).
	var doc struct {
		Profile struct {
			Name   string `json:"name"`
			Age    int    `json:"age"`
			Gender string `json:"gender"`
			Height int    `json:"height"`
			Weight int    `json:"weight"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(entries[0].JsonData), &doc); err != nil {
		t.Fatalf("JSON записи не парсится: %v", err)
	}
	if doc.Profile.Name != "Влад" || doc.Profile.Age != 27 || doc.Profile.Gender != "Мужской" ||
		doc.Profile.Height != 180 || doc.Profile.Weight != 70 {
		t.Errorf("в questionnaire-записи неверный profile: %+v", doc.Profile)
	}
}

// TestSaveProfileQuestionnaireNoMonitorRepo - если monitorRepo не задан
// (например, в упрощённых тестах), saveProfile не падает и сохраняет хотя бы
// постоянный профиль (user_profiles).
func TestSaveProfileQuestionnaireNoMonitorRepo(t *testing.T) {
	chatID := int64(556011)
	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	st := storage.NewMockStorage()
	r := &router{stateManager: smgr, appStorage: st} // monitorRepo == nil

	smgr.SetUserData(chatID, "name", "Влад")
	smgr.SetUserData(chatID, "age", "27")
	smgr.SetUserData(chatID, "gender", "Мужской")
	smgr.SetUserData(chatID, "height", "180")
	smgr.SetUserData(chatID, "weight", "70")

	r.saveProfile(context.Background(), chatID, "") // не должно паниковать

	if p, err := st.Users.GetProfile(context.Background(), chatID); err != nil || p == nil {
		t.Fatalf("профиль не сохранён при nil monitorRepo: %v %+v", err, p)
	}
}

// TestProfileConfirmFromQuestionnaire - регрессионный: бот должен находить
// профиль не только в user_profiles, но и в анкете Mini App (questionnaire).
// Ранее дашборд видел профиль (анкета есть), а бот - нет (user_profiles пуст),
// и переспрашивал имя/возраст. Теперь tryProfileConfirm резолвит профиль из
// обоих источников (анкета приоритетна) и синхронизирует в user_profiles.
func TestProfileConfirmFromQuestionnaire(t *testing.T) {
	chatID := int64(556020)

	smgr := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})
	st := storage.NewMockStorage() // user_profiles НАМЕРЕННО ПУСТ
	monRepo := monitoring.NewMockRepository()

	// Только анкета Mini App (questionnaire), как если бы профиль заполнили
	// в «Мой профиль», а user_profiles оказался пуст (после /resetme или
	// молчаливого сбоя записи). Именно этот рассинхрон давал баг.
	qPayload, _ := json.Marshal(map[string]interface{}{
		"profile": map[string]interface{}{
			"name":   "Влад",
			"age":    27,
			"gender": "Мужской",
			"height": 180,
			"weight": 70,
		},
		"recommendations": []string{"рекомендация"},
	})
	if err := monRepo.SaveResult(context.Background(), &monitoring.HistoryEntry{
		TelegramID: chatID,
		Type:       "questionnaire",
		Title:      "Профиль пользователя",
		Date:       time.Now(),
		JsonData:   string(qPayload),
	}); err != nil {
		t.Fatalf("не удалось посеять анкету: %v", err)
	}

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()
	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		smgr, nil, nil, nil, "", "", int64(0), agr, pay, st, monRepo, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)
	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressCallback := func(data string) {
		handle(context.Background(), b, &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:      "cb_" + data,
				From:    models.User{ID: chatID},
				Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: int(mt.lastID()), Chat: models.Chat{ID: chatID}}},
				Data:    data,
			},
		})
		sleep()
	}

	// Запуск базового Bioscan: профиль должен быть найден ИЗ АНКЕТЫ.
	pressCallback("section_bioscan_basic")

	if got := smgr.GetState(chatID); got != states.StateWaitingProfileConfirm {
		t.Fatalf("ожидалось состояние ProfileConfirm (профиль из анкеты), получили %q", got)
	}

	// И профиль должен синхронизироваться обратно в user_profiles.
	if p, err := st.Users.GetProfile(context.Background(), chatID); err != nil || p == nil || p.Age != 27 {
		t.Fatalf("профиль не синхронизирован в user_profiles из анкеты: %v %+v", err, p)
	}
}
