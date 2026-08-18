package router

import (
	"context"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// TestHandleRecoversFromPanicInHandleText - проверяет, что defer recover() в
// handle ловит панику, возникшую в handleText, и обработка сообщения
// завершается штатно (без падения горутины/процесса).
//
// Сценарий: пользователь (с принятым соглашением) в состоянии Idle шлёт
// произвольный текст. Все промежуточные обработчики возвращают false, и поток
// доходит до handleText, который вызывает метод на nil-интерфейсе
// analysisService -> паника. recover в handle должен её перехватить.
func TestHandleRecoversFromPanicInHandleText(t *testing.T) {
	chatID := int64(987654321)
	sm := states.NewMemoryStateManager(t.TempDir() + "/states.json")
	agr := storage.NewAgreementStorage(t.TempDir() + "/agree.json")
	agr.SetAgreed(chatID)

	// analysisService намеренно оставляем nil: вызов метода на nil-интерфейсе
	// в handleText вызовет панику, которую должен поймать defer recover в handle.
	r := &router{
		stateManager:     sm,
		agreementStorage: agr,
	}

	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{ID: chatID},
			Text: "произвольный текст, который уходит в обработку через ИИ",
		},
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				// Паника дошла до нас - значит, recover внутри handle НЕ сработал.
				t.Errorf("паника в handle НЕ была перехвачена defer recover: %v", rec)
			}
			close(done)
		}()
		r.handle(context.Background(), &tgbot.Bot{}, update)
	}()

	select {
	case <-done:
		// Обработка завершилась: паника перехвачена recover внутри handle.
	case <-time.After(3 * time.Second):
		t.Fatal("handle не завершился за 3с - возможно, паника не перехвачена и горутина зависла")
	}
}
