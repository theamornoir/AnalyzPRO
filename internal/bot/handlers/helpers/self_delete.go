package helpers

import (
	"context"
	"time"

	tgbot "github.com/go-telegram/bot"
)

// deleteAfterReplyDelay - задержка перед удалением исходного сообщения
// пользователя (reply-кнопка) или сообщения с inline-клавиатурой после того,
// как бот уже ответил. Даём пользователю увидеть собственный выбор, затем
// убираем «мусорную» запись из истории чата (правило «кнопка/выбор удаляется
// после ответа»).
const deleteAfterReplyDelay = 500 * time.Millisecond

// DeleteAfterReply - удаляет сообщение (chatID, messageID) спустя
// deleteAfterReplyDelay после того, как бот уже отправил ответ на действие
// пользователя. Применяется для глобального правила «кнопка/выбор удаляется
// после ответа»: сообщение пользователя с reply-кнопкой либо сообщение с
// inline-клавиатурой убираются из чата, не засоряя историю.
//
// Удаление выполняется в отдельной горутине с фоновым контекстом, поскольку
// контекст апдейта отменяется сразу после ответа, а удалить нужно спустя
// задержку. Ошибки удаления (сообщение уже удалено и т.п.) игнорируются.
func DeleteAfterReply(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	go func() {
		t := time.NewTimer(deleteAfterReplyDelay)
		defer t.Stop()
		<-t.C
		DeleteMessage(context.Background(), b, chatID, messageID)
	}()
}
