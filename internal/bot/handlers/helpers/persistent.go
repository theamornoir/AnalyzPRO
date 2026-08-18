package helpers

import (
	"context"
	"strconv"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// MainMenuMsgKey - ключ в user-data для message_id единственного
// «закреплённого» навигационного сообщения бота (главное меню / экран отмены
// загрузки и т.п.). Ровно одно такое сообщение висит внизу чата; перед
// показом нового старое удаляется, чтобы не плодить дубли. Используется
// хелперами ShowPersistentMessage / DeletePersistentMessage и совместно
// пакетами router и upload (router импортирует upload, поэтому общий ключ и
// логика трекинга вынесены сюда, чтобы избежать цикла импортов).
const MainMenuMsgKey = "main_menu_msg_id"

// ShowPersistentMessage - отправляет params и закрепляет message_id в
// user-data под key, предварительно удалив ранее закреплённое сообщение.
// Так в чате держится ровно одно видимое навигационное сообщение: оно не
// самоудаляется (иначе после удаления кнопок по глобальному правилу «кнопка/
// выбор удаляется после ответа» внизу чата образовывалось бы «пустое дно»),
// но и не копится при повторных возвратах в один и тот же экран.
// Возвращает message_id отправленного сообщения (0 при ошибке).
func ShowPersistentMessage(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, key string, params tgbot.SendMessageParams) int {
	if id := trackedMsgID(sm, chatID, key); id > 0 {
		DeleteMessage(ctx, b, chatID, id)
	}
	msg, err := b.SendMessage(ctx, &params)
	if err != nil || msg == nil {
		return 0
	}
	if sm != nil && key != "" {
		sm.SetUserData(chatID, key, strconv.Itoa(msg.ID))
	}
	return msg.ID
}

// DeletePersistentMessage - удаляет закреплённое сообщение под key (если есть)
// и сбрасывает его id. Безопасно при отсутствии.
func DeletePersistentMessage(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, key string) {
	if id := trackedMsgID(sm, chatID, key); id > 0 {
		DeleteMessage(ctx, b, chatID, id)
		if sm != nil {
			sm.SetUserData(chatID, key, "0")
		}
	}
}

// trackedMsgID - читает закреплённый message_id из user-data (0, если нет).
func trackedMsgID(sm states.StateManager, chatID int64, key string) int {
	if sm == nil || key == "" {
		return 0
	}
	n, err := strconv.Atoi(sm.GetUserData(chatID, key))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
