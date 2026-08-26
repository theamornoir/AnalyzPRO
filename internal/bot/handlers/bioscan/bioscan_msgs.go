package bioscan

import (
	"context"
	"encoding/json"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// bioscanMsgIDsKey - ключ состояния со списком ID исходных фото-сообщений
// пользователя (Bioscan PRO - 4 фото, базовый Bioscan - 1 фото), присланных
// для анализа. После успешной обработки ИИ фото удаляются из чата.
const bioscanMsgIDsKey = "bioscan_msg_ids"

// appendBioscanMsgID добавляет ID исходного фото-сообщения пользователя в
// список для последующего удаления из чата после обработки Bioscan.
func appendBioscanMsgID(sm states.StateManager, chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	ids := getBioscanMsgIDs(sm, chatID)
	ids = append(ids, msgID)
	data, _ := json.Marshal(ids)
	sm.SetUserData(chatID, bioscanMsgIDsKey, string(data))
}

// getBioscanMsgIDs возвращает список ID исходных фото-сообщений пользователя.
func getBioscanMsgIDs(sm states.StateManager, chatID int64) []int {
	raw := sm.GetUserData(chatID, bioscanMsgIDsKey)
	if raw == "" {
		return nil
	}
	var ids []int
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// clearBioscanMsgIDs очищает список ID исходных фото-сообщений.
func clearBioscanMsgIDs(sm states.StateManager, chatID int64) {
	sm.SetUserData(chatID, bioscanMsgIDsKey, "")
}

// deleteBioscanSubmittedMessages удаляет из чата исходные фото-сообщения
// пользователя (Bioscan), присланные для анализа. Вызывается ПОСЛЕ
// успешной обработки ИИ, чтобы исходные фото не оставались в истории чата и
// не хранились ботом. Удаление наилучшего усилия: ошибки игнорируются.
//
// В приватном чате (1-на-1) бот может удалять сообщения пользователя без
// ограничений по возрасту сообщения.
func deleteBioscanSubmittedMessages(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	for _, id := range getBioscanMsgIDs(sm, chatID) {
		helpers.DeleteMessage(ctx, b, chatID, id)
	}
	clearBioscanMsgIDs(sm, chatID)
}
