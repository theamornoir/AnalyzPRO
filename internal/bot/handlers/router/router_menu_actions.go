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

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/reminders"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// WebAppAssetsVersion и WithWebAppVersion вынесены в пакет keyboards
// (keyboards.WebAppAssetsVersion / keyboards.WithWebAppVersion), чтобы их
// могли переиспользовать и другие пакеты (например, кнопка «Открыть Мой
// профиль» после выдачи отчёта). Здесь оставлен только комментарий-якорь.

// hubMessageKey - ключ в user-data, в котором хранится message_id текущего
// «блока-хаба» (раздел Анализы/Здоровье/Сервис). Блок редактируется на месте
// (editMessage) при переключении разделов, чтобы не плодить сообщения: один
// блок перерисовывается вкладками, а результаты под-действий приходят
// отдельными сообщениями.
const hubMessageKey = "hub_message_id"

// hubAnchorKey - ключ в user-data для message_id «якорного» сообщения раздела.
// Оно несёт внизу единую Reply-клавиатуру [Назад] (висит на всём протяжении
// раздела). Telegram не позволяет совместить inline-кнопки действий и эту
// Reply-клавиатуру в одном сообщении, поэтому «якорь» и блок - два сообщения.
const hubAnchorKey = "hub_anchor_id"

// lastMsgKey - ключ в user-data для message_id последнего «шагового»
// сообщения раздела/флоу. Используется обработчиком «Назад», чтобы удалить
// именно текущее сообщение раздела перед возвратом в главное меню.
const lastMsgKey = "last_msg_id"

// mainMenuMsgKey - ключ в user-data для message_id «закреплённого» сообщения
// главного меню. Когда пользователь возвращается в главное меню (кнопка
// «Назад» / «Отмена» / выход из Premium), бот оставляет ПЕРСИСТЕНТНОЕ
// сообщение меню, а не самоудаляющееся уведомление. Иначе после удаления
// кнопок по глобальному правилу «кнопка/выбор удаляется после ответа» внизу
// чата образуется «пустое дно». Старое закреплённое сообщение удаляется
// перед показом нового, чтобы не плодить дубли.
//
// Ключ вынесен в пакет helpers (helpers.MainMenuMsgKey), чтобы его мог
// переиспользовать и пакет upload (router импортирует upload, прямой импорт
// наоборот дал бы цикл).
const mainMenuMsgKey = helpers.MainMenuMsgKey

// hubSection описывает содержимое одного раздела-хаба.
type hubSection struct {
	text    string
	actions models.InlineKeyboardMarkup
}

// hubSections возвращает содержимое каждого раздела-хаба по его коду.
func hubSections() map[string]hubSection {
	return map[string]hubSection{
		"analysis": {text: locales.MsgAnalysisHubIntro, actions: keyboards.AnalysisHubMenu()},
		"health":   {text: locales.MsgHealthHubIntro, actions: keyboards.HealthHubMenu()},
		"service":  {text: locales.MsgServiceHubIntro, actions: keyboards.ServiceHubMenu()},
	}
}

// renderHub - показывает/переключает «блок-хаб» раздела ДВУМЯ сообщениями:
//  1. «якорь» - описание раздела + единая Reply-клавиатура [Назад] внизу;
//  2. «блок» - инлайн-кнопки под-действий раздела + подсказка
//     «👇 Выберите действие:». Telegram не позволяет совместить инлайн-кнопки
//     действий и Reply-клавиатуру [Назад] в одном сообщении, поэтому хаб -
//     два сообщения.
//
// Если у пользователя уже открыты оба сообщения хаба (сохранены hub_anchor_id
// и hub_message_id), раздел перерисовывается прямо в них (editMessage на
// месте), иначе отправляются новые. Результаты под-действий (анализ,
// консультация и т.п.) приходят отдельными сообщениями.
func (r *router) renderHub(ctx context.Context, b *tgbot.Bot, chatID int64, section string) bool {
	sections := hubSections()
	sec, ok := sections[section]
	if !ok {
		section = "analysis"
		sec = sections[section]
	}

	// Запоминаем текущий раздел для иерархического «Назад» (подшаг -> хаб).
	r.setCurrentSection(chatID, section)

	// Уходим из главного меню в раздел-хаб - убираем закреплённое
	// сообщение главного меню (в т.ч. приветствие /start), чтобы оно
	// не висело над хабом. Безопасно, если такого сообщения нет.
	r.deleteMainMenuMessage(ctx, b, chatID)

	anchorID := r.hubAnchorID(chatID)
	msgID := r.hubMessageID(chatID)

	// Оба сообщения на месте - перерисовываем на месте (edit), чтобы не
	// плодить новые сообщения при переключении разделов.
	if anchorID > 0 && msgID > 0 && r.editHubPair(ctx, b, chatID, anchorID, msgID, sec) {
		return true
	}

	// Не удалось отредактировать (сообщения удалены пользователем и т.п.) -
	// чистим остатки и отправляем хаб заново.
	r.deleteHubBlock(ctx, b, chatID)

	// 1) Якорь: описание раздела + единая Reply-клавиатура [Назад].
	newAnchorID, anchorErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        sec.text,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	// 2) Блок: под-действия раздела + подсказка «👇 Выберите действие:».
	newMsgID, blockErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Выберите действие:",
		ReplyMarkup: sec.actions,
	})
	if anchorErr == nil && newAnchorID > 0 {
		r.setHubAnchorID(chatID, newAnchorID)
	}
	if blockErr == nil && newMsgID > 0 {
		r.setHubMessageID(chatID, newMsgID)
		r.setLastMsg(chatID, newMsgID)
	}
	return true
}

// editHubPair пытается перерисовать существующий хаб на месте: якорь
// (описание раздела) и блок (подсказка + инлайн-кнопки действий). Возвращает
// true, если редактирование удалось (в т.ч. при «message is not modified»).
func (r *router) editHubPair(ctx context.Context, b *tgbot.Bot, chatID int64, anchorID, msgID int, sec hubSection) bool {
	// Якорь: описание раздела.
	_, aErr := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: anchorID,
		Text:      sec.text,
		ParseMode: "Markdown",
	})
	if aErr != nil && !strings.Contains(aErr.Error(), "message is not modified") {
		log.Printf("[HUB] не удалось отредактировать якорь msgID=%d chatID=%d: %v", anchorID, chatID, aErr)
		return false
	}
	// Блок: подсказка + инлайн-кнопки действий.
	_, bErr := b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   msgID,
		Text:        "Выберите действие:",
		ReplyMarkup: sec.actions,
	})
	if bErr != nil && !strings.Contains(bErr.Error(), "message is not modified") {
		log.Printf("[HUB] не удалось отредактировать блок msgID=%d chatID=%d: %v", msgID, chatID, bErr)
		return false
	}
	log.Printf("[HUB] блок переключён на вкладку (anchor=%d block=%d chatID=%d)", anchorID, msgID, chatID)
	return true
}

// deleteHubBlock - удаляет текущий «блок-хаб» (раздел Анализы/Здоровье/
// Сервис) из чата вместе с «якорем» (Reply-клавиатура [Назад]) и сбрасывает
// сохранённые id. Используется, когда пользователь выбирает под-действие
// (анализ/консультация и т.п.) или нажимает «Назад» из раздела: в чате не
// должно висеть устаревшее меню раздела (иначе - «куча непонятно чего»).
// Безопасно, если блока/якоря нет.
func (r *router) deleteHubBlock(ctx context.Context, b *tgbot.Bot, chatID int64) {
	msgID := r.hubMessageID(chatID)
	anchorID := r.hubAnchorID(chatID)
	if msgID > 0 {
		helpers.DeleteMessage(ctx, b, chatID, msgID)
	}
	if anchorID > 0 {
		helpers.DeleteMessage(ctx, b, chatID, anchorID)
	}
	if msgID > 0 || anchorID > 0 {
		log.Printf("[HUB] блок удалён (msgID=%d anchorID=%d chatID=%d)", msgID, anchorID, chatID)
	}
	r.setHubAnchorID(chatID, 0)
	r.setHubMessageID(chatID, 0)
	// Также убираем «висячее» закреплённое сообщение главного меню: оно
	// могло остаться, если пользователь ушёл из главного меню в раздел/флоу
	// (например, нажал «Анализы»). Безопасно при отсутствии.
	r.deleteMainMenuMessage(ctx, b, chatID)
}

// hubMessageID / setHubMessageID - чтение/запись message_id текущего блока-хаба.
func (r *router) hubMessageID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, hubMessageKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setHubMessageID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, hubMessageKey, strconv.Itoa(msgID))
}

// hubAnchorID / setHubAnchorID - чтение/запись message_id «якоря» раздела
// (сообщение с Reply-клавиатурой [Назад]).
func (r *router) hubAnchorID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, hubAnchorKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setHubAnchorID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, hubAnchorKey, strconv.Itoa(msgID))
}

// lastMsgID / setLastMsg - чтение/запись message_id последнего «шагового»
// сообщения раздела/флоу (для удаления при нажатии «Назад»).
func (r *router) lastMsgID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, lastMsgKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setLastMsg(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, lastMsgKey, strconv.Itoa(msgID))
}

// mainMenuMsgID / setMainMenuMsgID - чтение/запись message_id закреплённого
// сообщения главного меню.
func (r *router) mainMenuMsgID(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, mainMenuMsgKey))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (r *router) setMainMenuMsgID(chatID int64, msgID int) {
	r.stateManager.SetUserData(chatID, mainMenuMsgKey, strconv.Itoa(msgID))
}

// deleteMainMenuMessage - удаляет закреплённое сообщение главного меню (если
// есть) и сбрасывает его id. Безопасно при отсутствии.
func (r *router) deleteMainMenuMessage(ctx context.Context, b *tgbot.Bot, chatID int64) {
	if id := r.mainMenuMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setMainMenuMsgID(chatID, 0)
	}
}

// showMainMenuMessage - показывает ПЕРСИСТЕНТНОЕ сообщение главного меню
// (text + Reply-клавиатура MainMenu) и закрепляет его id в user-data,
// предварительно удалив предыдущее закреплённое сообщение. Используется
// вместо helpers.SendAndDelete(MsgBackToMainMenu): по глобальному правилу
// «кнопка/выбор удаляется после ответа» исходные сообщения с кнопками
// (включая «Назад») исчезают, поэтому возврат в главное меню должен оставить
// видимое меню, а не самоудаляющуюся запись - иначе внизу чата образуется
// «пустое дно».
func (r *router) showMainMenuMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	r.deleteMainMenuMessage(ctx, b, chatID)
	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: keyboards.MainMenu(),
	})
	if err == nil && msg != nil {
		r.setMainMenuMsgID(chatID, msg.ID)
	}
}

// handleFeedbackStart - запускает режим ввода отзыва/предложения: описывает
// раздел и переводит пользователя в StateWaitingFeedback, ожидая следующее
// сообщение (текст/фото/документ), которое будет переслано разработчику.
func (r *router) handleFeedbackStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[FEEDBACK] открытие раздела для chatID=%d", chatID)

	// Выбрано под-действие - убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingFeedback)
	r.setCurrentSection(chatID, "service")

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении ввода отзыва.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgFeedbackIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleFeedbackMessage - пересылает сообщение пользователя (текст/фото/
// документ) разработчику (adminChatID) и подтверждает доставку. Срабатывает
// при любом сообщении в режиме StateWaitingFeedback. Возвращает true, если
// сообщение обработано как отзыв.
func (r *router) handleFeedbackMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[FEEDBACK] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат - на уровень выше (хаб Сервис), а не в главное меню.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[FEEDBACK] отмена ввода chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	// Получатель не настроен - отзыв некуда доставлять.
	if r.adminChatID == 0 {
		log.Printf("[FEEDBACK] adminChatID не задан, доставка невозможна chatID=%d", chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgFeedbackUnavailable,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	from := update.Message.From
	fullName := "-"
	username := "-"
	if from != nil {
		name := strings.TrimSpace(from.FirstName + " " + from.LastName)
		if name != "" {
			fullName = name
		}
		if from.Username != "" {
			username = from.Username
		}
	}

	// Служебная «шапка» админу перед пересланным сообщением.
	_, metaErr := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    r.adminChatID,
		Text:      fmt.Sprintf(locales.MsgFeedbackMeta, fullName, chatID, username),
		ParseMode: "Markdown",
	})

	// Пересылаем само сообщение (текст/фото/документ) как есть - так
	// разработчик видит оригинал, включая вложения.
	if update.Message != nil {
		if _, fErr := b.ForwardMessage(ctx, &tgbot.ForwardMessageParams{
			ChatID:     r.adminChatID,
			FromChatID: chatID,
			MessageID:  update.Message.ID,
		}); fErr != nil {
			log.Printf("[FEEDBACK] ошибка пересылки chatID=%d: %v", chatID, fErr)
		}
	}

	if metaErr != nil {
		log.Printf("[FEEDBACK] ошибка отправки админу chatID=%d: %v", chatID, metaErr)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgFeedbackSendError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateIdle)
	log.Printf("[FEEDBACK] отзыв доставлен админу от chatID=%d", chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgFeedbackConfirmed,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleRegularAnalysis - запускает обычный анализ.
func (r *router) handleRegularAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterRegular, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "regular")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "regular")
	r.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	r.setCurrentSection(chatID, "analysis")

	// Выбрано под-действие - убираем блок-хаб, чтобы в чате не висело меню
	// раздела поверх начатого анализа.
	r.deleteHubBlock(ctx, b, chatID)

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении анализа.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgRegularAnalysisIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleExtendedAnalysis - запускает расширенный анализ (с опросником).
func (r *router) handleExtendedAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterExtended, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	// Расширенный анализ - функция Premium: без подписки не запускаем
	// опросник, предлагаем оформить Premium.
	if !r.paymentService.IsUserPremium(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgExtendedAnalysisPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "extended")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")

	r.stateManager.SetState(chatID, states.StateWaitingName)
	r.setCurrentSection(chatID, "analysis")

	// Выбрано под-действие - убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	// Единая Reply-клавиатура [Назад / ❌ Отмена] на всём протяжении
	// опросника; первый вопрос показывается с прогресс-баром.
	userdata.NewUserDataCollector(r.stateManager).SendStep(ctx, b, chatID, states.StateWaitingName, locales.MsgExtendedAnalysisIntro)
	return true
}

// handleBioscanBasicStart - запускает БАЗОВЫЙ (бесплатный) Bioscan: 1 фото ->
// текстовый результат в чат. Доступен всем (проверка соглашения + busy).
func (r *router) handleBioscanBasicStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[BIOSCAN] запуск БАЗОВОГО для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		log.Printf(locales.LogRouterAgreeNotDone, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		log.Printf(locales.LogRouterUserBusy, chatID, currentState)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	r.setCurrentSection(chatID, "analysis")
	r.deleteHubBlock(ctx, b, chatID)
	bioscan.StartBioscanBasicFlow(ctx, b, r.stateManager, chatID)
	return true
}

// handleBioscanExtendedStart - запускает РАСШИРЕННЫЙ (Premium) Bioscan PRO:
// 4 фото -> детальный PDF-отчёт. Premium-гейт: без подписки - сообщение об
// оформлении Premium (анализ не начинается).
func (r *router) handleBioscanExtendedStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[BIOSCAN] запуск РАСШИРЕННОГО (PRO) для chatID=%d", chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		log.Printf(locales.LogRouterAgreeNotDone, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		log.Printf(locales.LogRouterUserBusy, chatID, currentState)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	if !r.paymentService.IsUserPremium(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanExtendedPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	r.setCurrentSection(chatID, "analysis")
	r.deleteHubBlock(ctx, b, chatID)

	// Принудительно устанавливаем состояние
	r.stateManager.SetState(chatID, states.StateWaitingBioscanName)
	log.Printf(locales.LogRouterForceBioscan, chatID)

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении Bioscan.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleDashboard - открывает веб-дашборд. Если demo=true - добавляет
// ?demo=1 к URL, чтобы открыть «полностью заполненную» синтетическую
// сводку без реальных анализов и без Premium (для предпросмотра графиков).
func (r *router) handleDashboard(ctx context.Context, b *tgbot.Bot, chatID int64, demo bool) bool {
	log.Printf(locales.LogRouterDashboard, chatID)

	// Выбрано под-действие - убираем блок-хаб, чтобы в чате не висело меню
	// раздела поверх открытого дашборда.
	r.deleteHubBlock(ctx, b, chatID)
	r.setCurrentSection(chatID, "health")

	isPremium := r.paymentService.IsUserPremium(chatID)
	log.Printf(locales.LogDashboardPremiumCheck, chatID, isPremium)

	// Версия в URL сбрасывает кэш Telegram WebView (см. keyboards.WebAppAssetsVersion).
	// При demo добавляем ?demo=1 - бэкенд отдаст синтетические метрики.
	webAppTarget := keyboards.WithWebAppVersion(r.webAppURL)
	if demo {
		if strings.Contains(webAppTarget, "?") {
			webAppTarget += "&demo=1"
		} else {
			webAppTarget += "?demo=1"
		}
	}
	if webAppTarget == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ URL дашборда не настроен. Задайте WEBAPP_URL или запустите `make mini`.",
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	text := locales.MsgHealthSummaryIntro + "\n\n"
	if !isPremium {
		// Полный доступ к показателям - по Premium, но профиль заполнить
		// можно бесплатно (онбординг доступен всем).
		text += "📝 Профиль можно заполнить бесплатно - после этого Мой профиль оживёт. " +
			"Полный доступ к показателям крови и динамике - по Premium-подписке.\n\n"
	}

	// Только Mini App - без ссылок и «открыть в браузере».
	rows := [][]models.InlineKeyboardButton{
		{
			{Text: "Открыть", WebApp: &models.WebAppInfo{URL: webAppTarget}},
		},
		{
			{Text: locales.BtnBack, CallbackData: "msg_back"},
		},
	}

	msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: rows},
		ParseMode:   "Markdown",
	})
	if sendErr != nil {
		log.Printf(locales.LogDashboardSendErr, chatID, sendErr)
	} else {
		log.Printf(locales.LogDashboardSent, chatID, msgID, webAppTarget, len(rows))
	}
	return true
}

// ============================================================================
// Быстрая консультация (с ИИ)
// ============================================================================

// freeConsultationLimit - сколько бесплатных консультаций доступно
// не-Premium пользователю. Premium - безлимит.
const freeConsultationLimit = 3

// consultUserKey - ключ счётчика использованных бесплатных консультаций
// в user-data состояния.
const consultUserKey = "ai_consult_count"

// consultCount - сколько бесплатных консультаций уже использовано.
func (r *router) consultCount(chatID int64) int {
	n, err := strconv.Atoi(r.stateManager.GetUserData(chatID, consultUserKey))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// consultSetCount - сохраняет счётчик использованных бесплатных консультаций.
func (r *router) consultSetCount(chatID int64, n int) {
	r.stateManager.SetUserData(chatID, consultUserKey, strconv.Itoa(n))
}

// consultationTextPrompt - формирует запрос к ИИ для текстовой консультации:
// задаёт роль консультанта и просит дать рекомендации + дисклеймер.
func consultationTextPrompt(question string) string {
	return "Вопрос пользователя о здоровье: " + strings.TrimSpace(question) +
		"\n\nТвоя задача - дать медицинскую консультацию: ответь на вопрос, " +
		"объясни возможные причины, дай практические рекомендации по облегчению " +
		"состояния. В конце обязательно напомни, что это информационная " +
		"консультация и не заменяет очный визит к врачу."
}

// consultationImageContext - формирует контекст к ИИ для консультации по
// фотографии (травма/проблемная зона). При наличии добавляет текстовый
// вопрос пользователя к фото.
func consultationImageContext(question string) string {
	base := "Это фото травмы или проблемной зоны пользователя. Пожалуйста, дай " +
		"медицинскую консультацию по фото: опиши, что видишь, возможные причины, " +
		"рекомендации по облегчению состояния и напомни, что это не заменяет " +
		"очный визит к врачу."
	question = strings.TrimSpace(question)
	if question != "" {
		base += "\n\nВопрос пользователя к фото: " + question
	}
	return base
}

// handleConsultationStart - запускает режим консультации: проверяет
// соглашение, Premium и оставшуюся бесплатную квоту. Если квота исчерпана и
// Premium нет - предлагает оформить подписку. Иначе переводит пользователя
// в StateWaitingConsultation, ожидая вопрос (текст) или фото.
func (r *router) handleConsultationStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[CONSULT] запуск для chatID=%d", chatID)

	// Выбрано под-действие - убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationPremiumRequired,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	r.stateManager.SetState(chatID, states.StateWaitingConsultation)
	r.setCurrentSection(chatID, "health")

	// Единая Reply-клавиатура [Назад] внизу на всём протяжении консультации.
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgConsultationStart,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleConsultationMessage - обрабатывает сообщение пользователя в режиме
// StateWaitingConsultation: текстовый вопрос или фото травмы. Отправляет его
// ИИ (GenerateAnalysisSummary / анализ фото) и возвращает консультацию.
// Возвращает true, если сообщение обработано как консультация.
func (r *router) handleConsultationMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	log.Printf("[CONSULT] ввод сообщения от chatID=%d", chatID)

	// Отмена / возврат - на уровень выше (хаб Здоровье), а не в главное меню.
	if text == locales.BtnCancel || text == locales.BtnBack {
		log.Printf("[CONSULT] отмена chatID=%d", chatID)
		r.backToParent(ctx, b, chatID)
		return true
	}

	hasPhoto := update.Message != nil && len(update.Message.Photo) > 0

	// Пустое/неподдерживаемое сообщение (стикер, голос, ничего) - просим
	// прислать вопрос или фото.
	if strings.TrimSpace(text) == "" && !hasPhoto {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationEmpty,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return true
	}

	isPremium := r.paymentService.IsUserPremium(chatID)

	// Повторная проверка квоты на случай «залипшего» состояния.
	if !isPremium && r.consultCount(chatID) >= freeConsultationLimit {
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationPremiumRequired,
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	// Индикатор ожидания: стикер + анимированный текст (формирую отчёт /
	// считаю показатели и т.п.). Гасится при получении результата или ошибке.
	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, r.stickerID, nil)

	var (
		result string
		err    error
	)

	if hasPhoto {
		photos := update.Message.Photo
		largest := photos[len(photos)-1]
		data, mimeType, dlErr := helpers.DownloadFileByID(ctx, b, largest.FileID, r.uploadDir)
		if dlErr != nil {
			log.Printf("[CONSULT] ошибка загрузки фото chatID=%d: %v", chatID, dlErr)
			r.stateManager.SetState(chatID, states.StateIdle)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgConsultationError,
				ReplyMarkup: keyboards.MainMenu(),
			})
			helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
			return true
		}
		result, err = r.analysisService.HandleAnalysisFromFileWithContext(ctx, data, mimeType, consultationImageContext(text))
	} else {
		result, err = r.analysisService.HandleAnalysis(ctx, consultationTextPrompt(text))
	}

	if err != nil || strings.TrimSpace(result) == "" {
		log.Printf("[CONSULT] ошибка генерации chatID=%d: %v", chatID, err)
		r.stateManager.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgConsultationError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		return true
	}

	// Успех: считаем бесплатную квоту только для не-Premium пользователей.
	if !isPremium {
		r.consultSetCount(chatID, r.consultCount(chatID)+1)
	}
	r.stateManager.SetState(chatID, states.StateIdle)

	// Собираем итоговый текст (без Markdown - результат ИИ неконтролируем,
	// чтобы не сломать разметку). Клавиатуру (главное меню) крепим к
	// последнему куску через sendLongMessage.
	full := locales.MsgConsultationResultIntro + result
	if !isPremium {
		freeLeft := freeConsultationLimit - r.consultCount(chatID)
		if freeLeft < 0 {
			freeLeft = 0
		}
		if freeLeft > 0 {
			full += fmt.Sprintf(locales.MsgConsultationQuotaLeft, freeLeft)
		}
	}

	helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
	sendLongMessage(ctx, b, chatID, full, keyboards.MainMenu())
	return true
}

// sendLongMessage - отправляет текст, разбивая его на куски ≤ 4000 символов
// по границам строк, чтобы не упереться в лимит Telegram (4096). Клавиатура
// (keyboard) крепится только к последнему куску.
func sendLongMessage(ctx context.Context, b *tgbot.Bot, chatID int64, text string, keyboard models.ReplyKeyboardMarkup) {
	const maxChunk = 4000
	runes := []rune(text)
	n := len(runes)
	if n <= maxChunk {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: keyboard,
		})
		return
	}

	chunks := []string{}
	for start := 0; start < n; {
		end := start + maxChunk
		if end > n {
			end = n
		}
		chunk := string(runes[start:end])
		if end < n {
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				end = start + idx + 1
				chunk = string(runes[start:end])
			}
		}
		chunks = append(chunks, chunk)
		start = end
	}

	for i, chunk := range chunks {
		kb := models.ReplyKeyboardMarkup{}
		if i == len(chunks)-1 {
			kb = keyboard
		}
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        chunk,
			ReplyMarkup: kb,
		})
	}
}

// ============================================================================
// Тест уведомлений (раздел «Сервис» → 🧪 Тест уведомлений)
// ============================================================================

// testNotifyDelay - через сколько секунд приходит ТЕСТОВОЕ уведомление после
// нажатия кнопки в под-меню «Сервис → 🧪 Тест уведомлений». Сделано малым,
// чтобы разработчик мог быстро проверить систему уведомлений «вживую».
const testNotifyDelay = 30 * time.Second

// handleTestNotifyMenu - открывает под-меню проверки уведомлений (раздел
// «Сервис»): поясняет, что нажатие кнопки пришлёт реальный образец
// уведомления через 30 секунд.
func (r *router) handleTestNotifyMenu(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf("[TEST-NOTIFY] открытие меню для chatID=%d", chatID)

	// Выбрано под-действие - убираем блок-хаб.
	r.deleteHubBlock(ctx, b, chatID)
	r.setCurrentSection(chatID, "service")

	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgTestNotifyIntro,
		ReplyMarkup: keyboards.TestNotifyMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}

// handleTestNotifyAction - планирует отправку ТЕСТОВОГО уведомления указанного
// типа (reminder/motivation/feature) через 30 секунд и подтверждает это
// пользователю, оставляя открытым тестовое меню (можно проверить иные типы).
func (r *router) handleTestNotifyAction(ctx context.Context, b *tgbot.Bot, chatID int64, kind string) bool {
	log.Printf("[TEST-NOTIFY] планирование уведомления kind=%s для chatID=%d", kind, chatID)

	// Используем фоновый контекст: контекст апдейта отменяется сразу после
	// ответа, а уведомление должно прийти спустя 30 секунд.
	go func() {
		select {
		case <-time.After(testNotifyDelay):
			reminders.SendTestNotification(context.Background(), b, chatID, kind)
		}
	}()

	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgTestNotifyScheduled,
		ReplyMarkup: keyboards.TestNotifyMenu(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		r.setLastMsg(chatID, msg.ID)
	}
	return true
}
