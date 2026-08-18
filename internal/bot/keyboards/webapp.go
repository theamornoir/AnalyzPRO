package keyboards

import (
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// WebAppAssetsVersion - версия статических активов Mini App (Мой профиль
// и Мониторинг). Увеличивайте при изменении index.html/app.js/style.css,
// чтобы Telegram WebView перезапросил свежие файлы (иначе отдаёт
// закэшированную старую версию - отсюда «пустой/старый» дашборд после правок).
//
// ВАЖНО: версия «вшивается» в САМ ПУТЬ к активам (app.<ver>.js /
// style.<ver>.css) в webapp_files/index.html, а НЕ только в query-параметр
// ?v=. Telegram WebView кэширует JS/CSS по пути файла и часто игнорирует
// query при кэшировании, поэтому версия в пути - единственный надёжный
// способ сбросить кэш: при смене версии меняется сам URL, и старый
// закэшированный файл отдать невозможно. ServeWebApp (dashboard.go)
// резолвит любую версию в актуальный встроенный файл.
// Должна совпадать с версией в ссылках на активы в webapp_files/index.html.
const WebAppAssetsVersion = "v39"

// WithWebAppVersion добавляет ?v=<version> к URL Mini App, сбрасывая кэш
// Telegram WebView при обновлении активов. Пустой URL не трогает.
func WithWebAppVersion(u string) string {
	if u == "" {
		return u
	}
	if strings.Contains(u, "?") {
		return u + "&v=" + WebAppAssetsVersion
	}
	return u + "?v=" + WebAppAssetsVersion
}

// OpenHealthSummaryButton возвращает inline-кнопку «Открыть Мой профиль»,
// которая запускает Mini App «Мой профиль» прямо из чата. URL версионируется
// (?v=), чтобы Telegram WebView не отдавал закэшированную старую версию дашборда.
// Используется после выдачи отчёта, чтобы пользователь мог сразу открыть
// сохранённый результат в «Моём профиле».
func OpenHealthSummaryButton(webAppURL string) models.InlineKeyboardMarkup {
	url := WithWebAppVersion(webAppURL)
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnOpenHealthSummary, WebApp: &models.WebAppInfo{URL: url}},
			},
		},
	}
}
