package helpers

import (
	"context"
	"html"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// SendSavedToSummary отправляет пользователю сообщение о том, что результат
// анализа/биоскана сохранён в «Мой профиль», и кнопку для мгновенного
// открытия этого Mini App. Вызывается после выдачи любого отчёта
// (обычный/расширенный анализ, базовый/PRO Bioscan), чтобы пользователь
// сразу видел, где найти сохранённые данные и мог открыть их в один тап.
//
// Безопасно вызывать при пустом webAppURL - тогда сообщение не шлётся (нет
// смысла показывать кнопку, ведущую в никуда).
func SendSavedToSummary(ctx context.Context, b *tgbot.Bot, chatID int64, webAppURL string) {
	if strings.TrimSpace(webAppURL) == "" {
		return
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgResultSavedSummary,
		ReplyMarkup: keyboards.OpenHealthSummaryButton(webAppURL),
	})
}

// SendSavedToSummaryDemo - вариант SendSavedToSummary для ДЕМО-режима
// (предпросмотр результатов «как заполнено», без реального сохранения).
// Сообщает, что в демо данные не сохраняются, но в обычном режиме всё
// попадёт в «Мой профиль», и даёт кнопку открыть ДЕМО-Сводку
// (?demo=1) - чтобы пользователь сразу увидел, как там оформлены отчёты.
// Безопасно при пустом webAppURL - сообщение не шлётся.
func SendSavedToSummaryDemo(ctx context.Context, b *tgbot.Bot, chatID int64, webAppURL string) {
	if strings.TrimSpace(webAppURL) == "" {
		return
	}
	demoURL := webAppURL
	if strings.Contains(demoURL, "?") {
		demoURL += "&demo=1"
	} else {
		demoURL += "?demo=1"
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgDemoResultSavedSummary,
		ReplyMarkup: keyboards.OpenHealthSummaryButton(demoURL),
	})
}

// PlainResultHTML формирует простой, но аккуратный HTML-документ для
// «неструктурированного» результата (обычный анализ текстом или базовый
// Bioscan). Сохраняется как ReportHTML истории, чтобы кнопка «📄 PDF» в
// «Мой профиль» открывала именно этот результат (без ошибки рендера).
// Текст экранируется и сохраняет переносы строк.
func PlainResultHTML(title, note string) string {
	esc := html.EscapeString
	body := strings.ReplaceAll(esc(note), "\n", "<br>")
	return `<!doctype html>
<html lang="ru"><head><meta charset="utf-8">
<title>` + esc(title) + `</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;max-width:720px;margin:32px auto;padding:0 16px;color:#1a2330}
h1{color:#1FA6A8} .note{white-space:pre-wrap;line-height:1.55}
</style></head><body>
<h1>` + esc(title) + `</h1>
<div class="note">` + body + `</div>
</body></html>`
}
