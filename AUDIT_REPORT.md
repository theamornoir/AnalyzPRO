# Финальный аудит Prisma перед релизом в production

Дата аудита: 2026-08-19. Аудитор: QA-инженер Prisma.
Объект: ветка рабочего дерева (изменения НЕ закоммичены). Сборка: `go build ./...` — OK,
`go vet ./...` — OK, `go test ./...` — все пакеты OK (в т.ч. dashboard, monitoring,
notifications, router, states). Версия Mini App (Go-константа `WebAppAssetsVersion`) = **v43**.

Легенда: ✅ PASS · ❌ FAIL · ⚠️ NEEDS CHECK · 🔄 NOT APPLICABLE

---

## 1. БЕЗОПАСНОСТЬ И РАЗГРАНИЧЕНИЕ ДОСТУПА

### 1.1 Free/Premium
- Free-пользователь видит в истории ТОЛЬКО 3 последние записи — ✅ PASS.
  `reports_data.go: buildGroup` режет `reports = reports[:freeHistoryLimit]` (freeHistoryLimit=3)
  строго по `isPremium`, БЕЗ учёта любого query-параметра лимита.
- Free не может открыть отчёт старше 3 через прямой API `/api/reports/file` — ✅ PASS.
  `dashboard.go: ReportFile` для не-Premium считает окно `ListHistory(...,1,freeHistoryLimit)`
  и возвращает **403**, если запись вне тройки (`TestReportFileFreeAccessWindow` — зелёный).
- Free не может создать проект/привязать запись/получить график в Мониторинге — ✅ PASS.
  `monitoring/api.go: requirePremium` → 403 на мутирующих эндпоинтах (`TestAPIHandlerFreeGate` — зелёный),
  даже при прямом вызове; GET доступны только для показа заглушки.
- Premium имеет доступ ко ВСЕМУ — ✅ PASS.
- Все проверки на бэкенде (не только фронт) — ✅ PASS (см. выше + `premiumCheck` в APIHandler).
- Прямой `?limit=100` для Free возвращает 3 — ✅ PASS (лимит задаётся `isPremium`, параметр `limit`
  бэкендом не читается).

### 1.2 Пользовательское соглашение
- Текст соглашения обновлён до версии 2.0 — ⚠️ NEEDS CHECK. В коде константа
  `agreement.go: UserAgreementText` помечена как **«версия 1.0»**. По содержанию соглашение
  покрывает анализы, графики, тренды, мониторинг (п.1–6) — ✅, и содержит явное
  «Бот НЕ заменяет врача» (п.2) — ✅. Рекомендую поднять метку версии до 2.0 для соответствия
  формальному требованию.
- Старые пользователи НЕ видят соглашение повторно / новые видят актуальную версию — ✅ PASS.
  `AgreementStorage` хранит факт принятия per-user; версии нет, поэтому старые не
  переспрашиваются (это и нужно), новые видят текущий текст. Кнопка «✅ Принять» работает — ✅.

### 1.3 Данные пользователей
- Поле `premium_expires_at` в таблице users — ✅ PASS (миграция `db.go`, таблица users).
- Поле `notifications_enabled` в preferences (default true) — ✅ PASS (`db.go`, `preferences`).
- Данные не передаются третьим лицам — ⚠️ NEEDS CHECK (см. НАЙДЕННЫЕ ПРОБЛЕМЫ, 🟡 MEDIUM: PostHog).
  В коде есть интеграция PostHog (`analytics.InitPostHog(cfg.PostHogAPIKey)`, вызовы `Track`
  в `router.handle`). При установленном `POSTHOG_API_KEY` в проде события (включая chat ID
  и метки действий) уходят на app.posthog.com — третья сторона. Соглашение утверждает
  «не передаются третьим лицам». Нужно либо поправить формулировку соглашения, либо
  задокументировать аналитику, либо отключить PostHog в проде.

---

## 2. МОНИТОРИНГ И ГРАФИКИ

### 2.1 Мониторинг (Free)
- Заглушка показывает заголовок «Мониторинг» — ✅ PASS (`monitoring/webapp_files/app.js: showFreeStub` → `setHeader('Мониторинг', ...)`).
- Заглушка показывает описание из локализации — ✅ PASS (текст в `index.html` free-stub).
- Кнопка «💎 Открыть Мониторинг» ведёт к оплате Premium — ❌ FAIL (функциональный дефект).
  Плашка `freeStubPlaque` — это статичный `<div class="premium-plaque">` с текстом
  «💎 Купите Premium, чтобы открыть Мониторинг...». В `app.js` для `freeStubPlaque`/вида
  `view-free` НЕ навешен ни один обработчик; `__premiumLink` (полученный из
  `/api/monitoring/status`) в ветке заглушки **не используется**. Free-пользователь видит
  заглушку, но не может кликнуть в неё, чтобы открыть Premium. (В отличие от «Моего профиля»,
  где `premiumModalOpen` → `openPremium` корректно проводит к оплате.)
- Демо-график — статичный, с фиктивными данными (не данные пользователя) — ✅ PASS
  (`drawFreeStubChart`: жёстко заданные `[6.8,6.4,...]`, не из БД).
- Демо-график подписан как пример — 🔄 NOT APPLICABLE / ⚠️ NEEDS CHECK. График рисуется с
  референсными линиями, но явной подписи «(пример)» на нём нет. Рекомендую добавить
  подпись «Пример графика» под демо-графиком для однозначности (🟢 LOW).

### 2.2 Мониторинг (Premium)
- Создание/привязка проектов, графики с референсными линиями — ✅ PASS
  (`renderCharts` строит линии `Норма (мин)/(макс)` из `REFERENCE_RANGES`; бэкенд 403 для Free).
- Все эндпоинты доступны (200 OK) — ✅ PASS (покрыто `TestAPIHandler`, `TestServiceFlow`).
- Примечание: отдельного `GET /api/monitoring/projects/{id}/chart` НЕТ — графики с
  референсными линиями строятся на фронте из данных `ProjectDetail`. Гейт Free→заглушка
  реализован корректно. Считаю ✅ (пункт 4.4 про `/chart` помечаю 🔄 NOT APPLICABLE — эндпоинт
  не предусмотрен по дизайну, графики считаются на клиенте).

### 2.3 Графики динамики
- Free — строго по 3 последним записям — ✅ PASS (`buildMetrics`: `trendMeas = trendMeas[:freeHistoryLimit]` для не-Premium).
- Free < 3 записей — график по доступным — ✅ PASS (`renderTrendCard`: при `n<2` линия не рисуется, только индекс).
- Premium — по ВСЕЙ истории — ✅ PASS.
- Подписи под графиками корректны — ✅ PASS.

### 2.4 Тренд-бейджи
- Free — только направление и период (БЕЗ чисел) — ✅ PASS (`renderTrendBadges` собирает
  `b.arrow + " " + b.indicator + ": " + direction + " " + b.period`; значения не выводятся).
- Free — не показываются, если данных < 2 точек — ✅ PASS (`computeTrendBadges` фильтрует
  `len(pts) < 2`; фронт прячет при пустом списке).
- Формат «↓ Глюкоза: снижается за последние 6 месяцев» — ✅ PASS (формируется именно так).
- Бейдж — статический текст (не кнопка) — ✅ PASS.

---

## 3. УВЕДОМЛЕНИЯ

### 3.1 Об окончании подписки
- Тексты за 7/3/1/0 дней в локализации, тон нарастает, «ЗАВТРА» заглавными — ✅ PASS
  (`messages.go`: MsgNotifSub7d/3d/1d/Today; MsgNotifSub1d содержит «ЗАВТРА»).
- Нет дублей — ✅ PASS. `subscription_notifications` с `UNIQUE(telegram_id, days_before)`
  + `hasSubscriptionNotification` (повторный заход не шлёт). Ранее найденный баг
  двойной отправки (`SendSubscriptionTest` + повторный `b.SendMessage`) **исправлен**
  (вызывающие места больше не дублируют; `TestSendSubscriptionTest` зелёный).
- Нет кнопок продления — ✅ PASS (`subscriptionText` возвращает только текст, без клавиатуры).
- Scheduler ежедневно в 10:00 — ✅ PASS (`service.go: dailyLoop`/`nextAt10`).
- Поле `premium_expires_at` используется для расчёта дней — ✅ PASS (`daysUntil(expires, now)`).

### 3.2 По анализам (выход за норму)
- Только Premium — ✅ PASS (`premiumStatus` gate перед проверкой).
- Раз в 3 дня (каждый 3-й прогон) — ✅ PASS (`day%3==0`).
- Гибкий парсинг 3 форматов JSON — ✅ PASS (`parseFormatCategories/Indicators/Results`).
- Референсный интервал по normal/ref_range/reference/ref_interval/norm/range — ✅ PASS (`refFields`).
- Учитывается status (warning/critical) — ✅ PASS (`isOutOfRange`).
- Текст: `⚠️ {показатель}: {значение} {единица} при норме {норма} {единица}. Рекомендуем обновить анализ.`
  — ⚠️ NEEDS CHECK (минорное отклонение). Реальный `MsgNotifAnalyticsDeviation` =
  «... при норме %s. Рекомендуем **обратиться к врачу и** обновить анализ.» — единица
  подставляется корректно (🟢), но добавлено «обратиться к врачу», чего нет в буквальной
  спецификации. Считаю улучшением, но отмечаю расхождение с документом.
- Единицы из unit/units — ✅ PASS (`unitFields`).
- Подавление 14 дней по показателю — ✅ PASS (`suppress(ctx, ..., now.Add(14*24*time.Hour))`).
- `notifications_enabled` проверяется — ✅ PASS (`notificationsEnabledUser`).

### 3.3 Инфраструктура уведомлений
- Все тексты в локализационном файле — ✅ PASS (`messages.go`), кроме выявленного дефекта ниже.
- Таблица `subscription_notifications` создана — ✅ PASS (`db.go`, `ensureNotificationSchema`).
- Таблица `notification_suppressions` создана — ✅ PASS.
- Индексы: `(telegram_id, days_before)` — ✅; `(telegram_id, indicator)` — ✅;
  `(premium_expires_at)` — ⚠️ NEEDS CHECK (🟢 LOW: индекс отсутствует; проверка подписки
  грузит ВСЕХ пользователей через `GetAllUsers`, поэтому индекс по `premium_expires_at`
  не использовался бы — влияния на перформанс нет, но формальное требование не выполнено).

---

## 4. БЭКЕНД

### 4.1 Сборка и тесты
- `go build ./...` — ✅ PASS (без ошибок).
- `go vet ./...` — ✅ PASS (без предупреждений).
- `go test ./...` — ✅ PASS (все пакеты ok).
- Покрытие критических путей — ✅ PASS (см. 4.2).

### 4.2 Ключевые тесты
Названия из чек-листа (`TestReportFileFreeAccessWindow`, `TestAPIHandlerFreeGate`,
`TestDeleteEntry`, `TestMetricsOK`) — ✅ PASS (присутствуют и зелёные).
`TestSubscriptionNotifications` / `TestAnalyticsNotifications` / `TestNotificationSuppression`
как ПРЯМЫХ имён — 🔄 NOT APPLICABLE: покрытие тех же сценариев реализовано под другими
именами (`TestRunSubscriptionChecks*`, `TestSendSubscriptionTest`, `TestAnalyticsMockPreview`,
`TestFormatDeviationWithUnits`) — все зелёные. Функционально раздел покрыт.

### 4.3 Модели и БД
- `premium_expires_at` в User — ✅ PASS.
- `notifications_enabled` в User (default true) — ✅ PASS (в `preferences`).
- Таблицы `subscription_notifications`, `notification_suppressions`, `used_promocodes` — ✅ PASS.
- Миграции применены (идемпотентно) — ✅ PASS (`db.go: Migrate`, `ensureNotificationSchema`).

### 4.4 API-эндпоинты
- `GET /api/reports` — лимит 3 для Free — ✅ PASS.
- `GET /api/reports/file` — гейт «окно 3» для Free — ✅ PASS (403 вне окна).
- `POST /api/monitoring/projects` — 403 для Free — ✅ PASS.
- `POST /api/monitoring/projects/{id}/bind` — 403 для Free — ✅ PASS.
- `GET /api/monitoring/projects/{id}/chart` — 🔄 NOT APPLICABLE (эндпоинт не предусмотрен;
  графики строятся на фронте; гейт Free→заглушка работает).

---

## 5. ФРОНТЕНД (MINI APP)

### 5.1 Версии и кеширование
- Актуальная версия `app.v41.js` (или новее) — ✅ PASS (фактически `app.v42.js` в `index.html`).
- Кеширование обновлено (версия в URL) — ⚠️ NEEDS CHECK (🟢 LOW). `index.html` ссылается на
  `app.v42.js`/`style.v42.css`, а Go-константа `WebAppAssetsVersion="v43"`. `ServeWebApp`
  резолвит ЛЮБУЮ версию в актуальный встроенный файл, поэтому функционально кэш сбрасывается
  корректно; но рассинхрон v42(HTML) vs v43(Go) стоит убрать, подняв версию в `index.html` до v43.

### 5.2 История
- `renderFreeHistoryNote` показывает «Показаны 3 из N. Открыть всю историю →» — ✅ PASS
  (`app.js: TEXTS.historyNote`).
- Подсказка кликабельна → модалка Premium — ✅ PASS (`el.onclick = openPremiumModal`).
- При ровно 3 записях подсказка НЕ показывается — ✅ PASS (`hiddenCount > 0`).
- Клик по карточке → полный отчёт без ограничений — ✅ PASS (`openReportPDF`/`openReportInline`,
  гейт окна 3 на бэкенде пропускает свои 3).
- Кнопка «Назад» возвращает в список — ✅ PASS.

### 5.3 Мониторинг
- Заглушка Free отображается корректно — ✅ PASS.
- Демо-график с фиктивными данными — ✅ PASS.
- Кнопка «Открыть Мониторинг» ведёт к оплате — ❌ FAIL (см. 2.1: плашка не кликабельна).

### 5.4 Оплата и возврат
- `openPremium` fallback при пустой ссылке (тост + открытие бота) — ✅ PASS
  (`app.js: openPremium` → `showMessage(TEXTS.openPremiumFallback)`).
- `sessionStorage.setItem("payment_attempt","true")` перед оплатой — ✅ PASS.
- `visibilitychange` → `checkPremiumStatusAfterReturn()` с кнопкой «Проверить статус» — ✅ PASS.
- После нажатия данные обновляются — ✅ PASS (`loadMetrics()`/`reloadMonitoringFrame()`).

### 5.5 Мёртвый код
- `premiumBlocked` полностью удалён — ✅ PASS (в `webapp_files` нет упоминаний `premiumBlocked`).
- Нет ссылок на несуществующее поле `Rich` — ✅ PASS. Флаг `latest.rich` используется
  легитимно (есть в `ReportBlock` на бэкенде), не является мёртвым кодом.

---

## 6. UX И ЛОКАЛИЗАЦИЯ

### 6.1 Тексты в локализации
- Соглашение (короткое, юридическое) — ✅ PASS.
- Уведомления подписки 7/3/1/день окончания — ✅ PASS.
- Уведомление по анализам (1 строка) — ✅ PASS (MsgNotifAnalyticsDeviation/List).
- Все тексты в `internal/locales/messages.go` (не `ru.json` — проект использует Go-константы,
  что корректно; пункт «в ru.json» считаю 🔄 NOT APPLICABLE по фактической архитектуре).

### 6.2 Кнопки и сообщения
- Кнопки/сообщения используют локализацию — ✅ PASS (`locales.*` везде).
- Нет захардкоженных пользовательских текстов — ✅ PASS (тексты вынесены в `TEXTS`/`locales`).

---

## 7. ТЕСТОВЫЕ КОМАНДЫ И КНОПКИ

### 7.1 Доступность
- Тестовые кнопки «⚙️ Сервис» доступны ТОЛЬКО в development — ✅ PASS
  (`main_menu.go: ServiceHubMenu` добавляет `BtnTestNotify` только при `isDev`;
  `keyboards.SetDevMode(appEnv=="development")`).
- Тестовые команды `/test_*` доступны ТОЛЬКО в development — ✅ PASS
  (`bot.go: setupCommands` и `registerHandlers` регистрируют `/test_*` при `appEnv=="development"`).
- В production кнопки СКРЫТЫ, команды НЕ РАБОТАЮТ — ⚠️ NEEDS CHECK (🟡 HIGH).
  Гейт корректен, НО `config.go` задаёт **`AppEnv: getEnv("APP_ENV", "development")`** —
  значение ПО УМОЛЧАНИЮ **"development"**. Если в проде `APP_ENV` не выставлен явно в
  `production`, бот стартует в dev-режиме и test-меню/команды будут АКТИВНЫ в проде.
  Рекомендую сменить дефолт на `"production"` (fail-safe) + обязательно прописать
  `APP_ENV=production` в прод-окружении.

### 7.2 Функциональность
- `/test_sub_7d/3d/1d/today` — имитация через 10 сек — ✅ PASS
  (`router.go` → `runTestNotification` через `time.After(10s)`; команды → `SendSubscriptionTest`).
- `/test_analytics_check` — проверка БЕЗ отправки (логи/предпросмотр) — ✅ PASS
  (`RunAnalyticsDryRun`, `DryRunMessage`).
- `/test_analytics_send` — проверка + реальная отправка — ✅ PASS (`SendAnalyticsTest`).
- Моки для анализов (когда у пользователя нет разборных анализов) — ✅ PASS
  (`mockAnalysisJSON`, `previewIndicators`, `TestAnalyticsMockPreview`; работают только в dev).

### 7.3 Интерфейс
- Кнопка «🧪 Тест уведомлений» в Сервисе (dev only) — ✅ PASS.
- Подменю с кнопками для каждой имитации — ✅ PASS (`TestNotifyMenu`).
- «🔎 Проверить» — предпросмотр отклонений — ✅ PASS (`test_analytics_check`).
- «📨 Отправить» — реальная отправка — ✅ PASS (`test_analytics_send`).

---

## 8. КРАЕВЫЕ СЛУЧАИ

### 8.1 История
- Free ровно 3 → подсказка НЕ показывается — ✅ PASS.
- Free 0 записей → пустая история, график не строится — ✅ PASS.
- Free 1–2 записи → график по доступным, тренд-бейдж НЕ показывается — ✅ PASS.
- Free → Premium → доступ разблокируется сразу (без перезагрузки) — ✅ PASS
  (`/api/metrics`/`/api/reports` пересчитывают по `isPremium` на каждый запрос;
  `visibilitychange` перезагружает данные).

### 8.2 Анализы
- Нет анализов → проверка пропускается (не падает) — ✅ PASS (`ErrNoAnalysisData`,
  `latestAnalysisIndicators` возвращает false при пустой истории).
- Анализ без референсного интервала → пропускается — ✅ PASS (`isOutOfRange` → false при
  нераспознанном интервале).
- Неизвестный формат → логируется, пропускается — ✅ PASS (`parseIndicators` логирует
  «неизвестный формат» и возвращает nil).
- Показатель в норме → не шлётся — ✅ PASS.
- Показатель вне нормы → шлётся (если нет подавления) — ✅ PASS.
- После отправки → подавление 14 дней — ✅ PASS.

### 8.3 Подписка
- Premium не активен → уведомления НЕ шлются — ✅ PASS (`premiumStatus` → false → `continue`).
- Истекает через 7/3/1/0 дней → 1 раз каждое — ✅ PASS (`TestRunSubscriptionChecks*`).
- Истекла и была пропущена (бот лежал) → досылается ровно 1 раз (kind=0) — ✅ PASS
  (`TestRunSubscriptionChecksExpired`, логика `rawPremium` + catch-up).
- Дубль НЕ отправляется (проверка таблицы) — ✅ PASS (`UNIQUE` + `hasSubscriptionNotification`).

---

## 9. ПРОИЗВОДИТЕЛЬНОСТЬ

- Запрос истории для Free не грузит все записи — ✅ PASS (лимит 3 на бэкенде).
- Проверка уведомлений не нагружает БД (индексы) — ✅ PASS (индексы на
  `(telegram_id, days_before)` и `(telegram_id, indicator)`; WAL + пул 8 соединений в `db.go`).
- Scheduler не дублирует проверки — ✅ PASS (один инстанс через flock `/tmp/analyzpro.lock` +
  `UNIQUE` в таблицах).

---

## 10. ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ

- `WEBAPP_PREMIUM_LINK` задан в production — ⚠️ NEEDS CHECK. Переменная читается только в
  `monitoring/api.go` (`os.Getenv("WEBAPP_PREMIUM_LINK")`) и отдаётся фронту через
  `/api/monitoring/status`. В `config.go` её НЕТ — значит она берётся напрямую из окружения.
  Нужно убедиться, что в проде `WEBAPP_PREMIUM_LINK` задан (иначе кнопка оплаты в Мониторинге
  не сработает). В «Моём профиле» fallback на подсказку открыть бота есть; в Мониторинге —
  см. дефект 2.1.
- `APP_ENV=production` в проде (отключает тестовые команды) — ⚠️ NEEDS CHECK (🟡 HIGH, см. 7.1:
  дефолт "development").
- Все переменные окружения настроены корректно — ⚠️ NEEDS CHECK (требует проверки прод-окружения:
  `BOT_TOKEN`, `APP_ENV`, `WEBAPP_PREMIUM_LINK`, `PROMO_CODES`, `PROMO_CODES_MONTHLY`,
  `ANTHROPIC_API_KEY`, `POSTHOG_API_KEY` (по необходимости), `DB_PATH`).

---

## НАЙДЕННЫЕ ПРОБЛЕМЫ

### 🟡 HIGH — серьёзный баг, желательно исправить до релиза

**H1. Дефолт `AppEnv="development"` открывает dev-инструменты в проде.**
`internal/config/config.go`: `AppEnv: getEnv("APP_ENV", "development")`. Гейт тестовых кнопок
и команд (`/test_sub_*`, `/test_analytics_*`, меню «🧪 Тест уведомлений») завязан на
`appEnv=="development"`. Если в прод-окружении `APP_ENV` не выставлен явно, бот стартует в
dev-режиме и пользователи видят отладочное меню и могут слать себе тестовые рассылки.
Рекомендация: сменить дефолт на `"production"` (fail-safe) и обязательно прописать
`APP_ENV=production` в прод-env.

### 🟠 MEDIUM — некритично, но исправить в ближайшем спринте

**M1. Заглушка Мониторинга для Free не ведёт к оплате Premium (не кликабельна).**
`internal/monitoring/webapp_files/index.html` (плашка `freeStubPlaque`) + `app.js`
(`showFreeStub` только рисует заголовок и демо-график). Поле `__premiumLink` из
`/api/monitoring/status` получено, но в ветке заглушки не используется, обработчик клика по
плашке отсутствует. Free-пользователь видит «💎 Купите Premium, чтобы открыть Мониторинг...»,
но кликнуть нельзя — конверсия Free→Premium на вкладке Мониторинга теряется (открыть Premium
можно только из меню бота кнопкой 💎 Premium).
Рекомендация: сделать `freeStubPlaque` кликабельным (по аналогии с `openPremium` из
«Моего профиля») или добавить кнопку «💎 Открыть Мониторинг», которая при наличии
`__premiumLink` вызывает `tg.openTelegramLink(__premiumLink)`, иначе — тост с подсказкой
открыть бота.

**M2. Передача данных в PostHog противоречит соглашению «не передаются третьим лицам».**
`internal/analytics` + вызовы `analytics.Track(...)` в `router.handle` шлют события
(включая chat ID и метки действий) в PostHog при установленном `POSTHOG_API_KEY`. Соглашение
(`agreement.go`, п.5) утверждает обратное. Рекомендация: либо скорректировать формулировку
соглашения (добавить пункт об аналитике/обезличенных метриках), либо отключить PostHog в
проде, либо хранить события локально.

### 🟢 LOW — косметика

**L1. Опечатка в пользовательском тексте уведомления.**
`internal/locales/messages.go:304` `MsgNotifAnalyticsDeviationList` =
`"...Рекомендуем обратиться к врачу и обновить анализ."` — пропущен пробел
(«Рекомендуем обратиться»). Видно пользователю в сводном уведомлении по нескольким
отклонениям. Исправить на «Рекомендуем обратиться». (Одиночный вариант `MsgNotifAnalyticsDeviation`
написан верно — «Рекомендуем обратиться».)

**L2. Метка версии соглашения.**
`agreement.go` помечен «версия 1.0»; требование — 2.0. Содержательно покрывает все функции и
предупреждение о враче. Поднять метку версии.

**L3. Рассинхрон версий активов Mini App.**
`index.html` ссылается на `app.v42.js`/`style.v42.css`, Go-константа `WebAppAssetsVersion="v43"`.
`ServeWebApp` резолвит любую версию, поэтому кэш сбрасывается корректно, но для единообразия
поднять версию в `index.html` до v43.

**L4. Индекс `(premium_expires_at)` не создан.**
Формальное требование 3.3 не выполнено, но проверка подписки грузит всех пользователей
(`GetAllUsers`), поэтому индекс не использовался бы — влияния на перформанс нет. Можно
добавить для полноты схемы.

**L5. Демо-график заглушки Мониторинга не подписан «(пример)».**
График статичный и фиктивный (✅), но явной подписи «пример» нет. Добавить подпись для ясности.

**L6. Формулировка уведомления по анализам.**
`MsgNotifAnalyticsDeviation` содержит «Рекомендуем обратиться к врачу и обновить анализ.», тогда
как спецификация 3.2 — «Рекомендуем обновить анализ.» (без «обратиться к врачу»). Считаю
улучшением, но отмечаю расхождение с документом.

---

## ФИНАЛЬНЫЙ ВЕРДИКТ

⚠️ **PASS WITH CONDITIONS — РЕЛИЗ РАЗРЕШЁН С ОГОВОРКАМИ**

Основной функционал работает: Free/Premium-гейты на бэкенде (история 3 записи, файлы,
Мониторинг 403 для Free, тренд-бейджи без чисел), уведомления подписки (без дублей, catch-up
пропущенного 0-го дня), уведомления по анализам (Premium-only, раз в 3 дня, гибкий парсинг,
подавление 14 дней), сборка/вет/тесты зелёные, фронтенд «Мой профиль» корректен (гейт,
подсказка 3 из N, оплата с fallback, возврат по visibilitychange).

Условия до/сразу после релиза (обязательно проверить/исправить):
1. **H1** — выставить `APP_ENV=production` в проде И сменить дефолт `config.go` на
   `"production"` (иначе dev-команды/меню активны в проде).
2. **M1** — сделать заглушку Мониторинга кликабельной к оплате Premium (Free-конверсия).
3. **M2** — устранить противоречие соглашения и PostHog (правка формулировки/отключение).
4. **L1** — исправить опечатку «Рекомендуемобратиться».

Мелкие (L2–L6) можно закрыть в ближайшем спринте.
