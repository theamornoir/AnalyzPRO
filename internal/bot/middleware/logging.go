package middleware

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Handler func(context.Context, *tgbot.Bot, *models.Update)

type Middleware func(Handler) Handler

func LoggingMiddleware(next Handler) Handler {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		next(ctx, b, update)
	}
}
