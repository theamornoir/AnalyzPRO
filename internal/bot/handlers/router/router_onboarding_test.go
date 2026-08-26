package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// TestShortOnboardingFlow проверяет, что онбординг сокращён до ДВУХ
// сообщений: (1) единое описание функционала (MsgOnboardingIntro) и (2)
// согласие на обработку данных со ссылкой на полный HTML-текст. Раньше был
// слайдер из 8 шагов + соглашение. После принятия согласия пользователь
// попадает в ГЛАВНОЕ меню ТОЛЬКО инлайн (без reply-клавиатуры).
//
// ВНИМАНИЕ: в проде /start обрабатывается отдельным командным хендлером
// menu.StartHandler (зарегистрирован в bot.go), а НЕ роутером handle - поэтому
// здесь для /start зовём StartHandler напрямую, как это делает tgbot при
// совпадении команды. Дальше (callback-ы онбординга) идут через роутер.
func TestShortOnboardingFlow(t *testing.T) {
	chatID := int64(555005)

	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	pay := payment.NewPaymentService(nil, payment.YooKassaConfig{})

	mt := newMockTelegram()
	srv := httptest.NewServer(mt.handler())
	defer srv.Close()

	b, err := tgbot.New("TESTTOKEN", tgbot.WithServerURL(srv.URL), tgbot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("не удалось создать бота: %v", err)
	}

	handle := MessageRouter(
		sm, nil, nil, nil, "", "", int64(0), agr, pay, nil, nil, nil,
		"https://app.example/dashboard", "https://app.example/dashboard", "development",
	)
	// В проде /start - отдельный командный хендлер menu.StartHandler.
	startH := menu.StartHandler(sm, agr, nil)

	sleep := func() { time.Sleep(700 * time.Millisecond) }
	pressStart := func() {
		startH(context.Background(), b, &models.Update{
			Message: &models.Message{Chat: models.Chat{ID: chatID}, Text: "/start", ID: 1},
		})
		sleep()
	}
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

	// /start для нового (не согласного) пользователя запускает онбординг.
	pressStart()

	// 1-е сообщение - интро со всем функционалом, кнопка «Согласие»
	// (onboarding_agreement), а НЕ шаги 2..8 старого слайдера.
	mt.mu.Lock()
	introText := mt.sent[0]
	introMarkup := mt.lastSendMarkup
	mt.mu.Unlock()
	if introText != locales.MsgOnboardingIntro {
		t.Fatalf("1-е сообщение онбординга не интро (got=%q)", introText)
	}
	if !strings.Contains(introMarkup, "onboarding_agreement") {
		t.Fatalf("1-е сообщение онбординга не содержит кнопку onboarding_agreement (markup=%s)", introMarkup)
	}

	// Переход ко 2-му сообщению (согласие).
	pressCallback("onboarding_agreement")

	// 2-е сообщение - согласие + ссылка на полный HTML-текст.
	mt.mu.Lock()
	agrText := ""
	for _, s := range mt.sent {
		if strings.Contains(s, "Согласие на обработку персональных данных") {
			agrText = s
		}
	}
	mt.mu.Unlock()
	if agrText == "" {
		t.Fatalf("2-е сообщение онбординга (согласие) не отправлено")
	}
	if !strings.Contains(agrText, "https://clck.ru/3VSSPo") {
		t.Fatalf("2-е сообщение онбординга не содержит ссылку на полный текст согласия (got=%q)", agrText)
	}

	// Принятие согласия - фиксируем и попадаем в Главное меню (инлайн).
	pressCallback("onboarding_accept")
	if !agr.IsAgreed(chatID) {
		t.Fatalf("согласие не зафиксировано после onboarding_accept")
	}
	mt.mu.Lock()
	acceptMarkup := mt.lastSendMarkup
	reachedMain := false
	for _, s := range mt.sent {
		if s == locales.MsgOnboardingDone {
			reachedMain = true
		}
	}
	mt.mu.Unlock()
	if !reachedMain {
		t.Errorf("после onboarding_accept не показано Главное меню (MsgOnboardingDone)")
	}
	// Главное меню - ТОЛЬКО inline: в markup НЕ должно быть reply-
	// клавиатуры (поле "keyboard" - это ReplyKeyboardMarkup; inline -
	// "inline_keyboard").
	if strings.Contains(acceptMarkup, `"keyboard"`) {
		t.Errorf("БАГ: Главное меню после онбординга содержит reply-клавиатуру (markup=%s)", acceptMarkup)
	}

	// Количество сообщений онбординга: ровно 2 (интро + согласие) плюс
	// Главное меню = 3. Отсутствие старого слайдера из 8 шагов.
	mt.mu.Lock()
	total := len(mt.sent)
	mt.mu.Unlock()
	if total != 3 {
		t.Fatalf("онбординг отправил не 3 сообщения (интро+согласие+главное меню), а %d: %v", total, mt.sent)
	}
}
