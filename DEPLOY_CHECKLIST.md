# Чек-лист деплоя Prisma в production (DevOps)

Дата: 2026-08-19. Объект: ветка рабочего дерева (изменения НЕ закоммичены).
Сборка `go build ./...`, `go vet ./...`, `go test ./...` — зелёные. Версия Mini App
(Go-константа `WebAppAssetsVersion`) = **v43**.

Конкретный ответ на вопрос «что сделать, чтобы бот заработал в production»:
бот НЕ готов к реальному запуску, потому что **в коде нет настоящей оплаты** —
платёжный слой `internal/payment/mock_yookassa.go` это мок, и Premium выдаётся
бесплатно по кнопке «✅ Оплатил (симуляция)». Ниже — полный список действий.

Легенда: ✅ есть · ❌ нет/сломано · ⚠️ надо проверить вручную в проде.

---

## 🔴 КРИТИЧЕСКИ ВАЖНО (без этого бот не работает в проде)

### П1. Подключить РЕАЛЬНЫЕ платежи (YooKassa ИЛИ Telegram Stars) — этот код ОТСУТСТВУЕТ
Файл `internal/payment/mock_yookassa.go` — это мок, НЕ боевая интеграция:
- `CreatePayment` возвращает `URL: "https://pay.test/checkout/<id>"` (мёртвая ссылка).
- `HandleWebhook` (обработчик успеха оплаты YooKassa) НИГДЕ не зарегистрирован как
  HTTP-роут (в `internal/bot/bot.go` `Start` мукс содержит только `/healthz`,
  `/dashboard*`, `/api/metrics`, `/api/profile`, `/api/reports*`, `/monitoring*`,
  `/api/monitoring*` — вебхука платежа нет).
- В `internal/bot/handlers/menu/premium_callback.go` кнопка
  «✅ Оплатил (симуляция)» (`premium_confirm_<tariff>`) сразу вызывает
  `ActivatePremiumManually` — то есть **любой пользователь получает Premium
  бесплатно**, просто нажав эту кнопку.
- `docs/DEPLOY.md` это подтверждает: «YooKassa подключена отдельно (в этой сборке
  платёж — мок, как договорено)».

Что сделать (один из вариантов, ДО релиза):
1. **YooKassa (реальная):** заменить `CreatePayment` на вызов Client YooKassa
   (создание платежа, возврат реальной `confirmation_url`); поднять HTTP-роут
   `/api/payment/webhook` → `paymentService.HandleWebhook`; убрать кнопку
   «Оплатил (симуляция)»; задать `WEBAPP_PREMIUM_LINK` (используется в
   `/api/monitoring/status` и фронте).
2. **Telegram Stars (нативно):** реализовать инвойс-флоу (`createInvoiceLink` /
   `sendInvoice`) и активацию по `successful_payment` в апдейте — вебхук не нужен,
   но код всё равно надо написать (сейчас нет).
3. **Или запуск как бесплатный/бета:** если монетизация не нужна на старте —
   удалить кнопку «Оплатил (симуляция)» и раздавать Premium только промокодами,
   иначе Premium будет бесплатен для всех. Даже в этом случае «Оплатить» с
   мёртвой ссылкой `pay.test` нужно убрать.

### П2. Выставить `APP_ENV=production` (иначе в проде активны dev-инструменты)
`internal/config/config.go`: `AppEnv: getEnv("APP_ENV", "development")` — дефолт
**"development"**. От этого зависят:
- видимость меню «🧪 Тест уведомлений» (`keyboards.SetDevMode(appEnv=="development")` в `bot.go`);
- регистрация команд `/test_sub_*` и `/test_analytics_*`
  (`bot.go: setupCommands`, `registerHandlers` — `if b.appEnv == "development"`);
- флаг `isDev` в `notifications.NewService` (dev-only моки анализов).
Действие: в `.env` прод-сервера жёстко прописать `APP_ENV=production` И сменить
дефолт в `config.go` на `"production"` (fail-safe, чтобы забытый env не открыл
dev-меню пользователям). Проверить: после старта в логах `LogAppEnvironment=production`
и отсутствие `/test_*` в `setMyCommands`.

### П3. Настоящий HTTPS-домен + Mini App-домен в @BotFather
Telegram Mini App требует HTTPS и зарегистрированный домен Mini App.
- Бот САМ TLS не терминирует: `bot.go: Start` поднимает HTTP на `HTTP_ADDR` (`:8080`).
  Нужен reverse-proxy (Caddy/nginx + Let's Encrypt) на 443 → :8080.
- Задать `WEBAPP_URL=https://<домен>/dashboard` и
  `DASHBOARD_URL=https://<домен>/dashboard` (иначе `config.go` подставит LAN-IP/
  туннель — работает только в dev).
- В @BotFather → Bot Settings → Mini App → указать `<домен>` (без /dashboard),
  иначе WebApp-кнопки «Открыть» не откроют дашборд.
- Проверить: открыть Mini App from phone over mobile network (не Wi-Fi dev) —
  должен грузиться дашборд, не «Cloudflare error» и не localhost.

### П4. Production BOT_TOKEN (не dev/тестовый) + ровно ОДИН инстанс
- `BOT_TOKEN` в `.env` должен быть токеном боевого бота. initData в
  `/api/monitoring/*` валидируется именно этим токеном (`monitoring/api.go: auth`)
  — несовпадение токена → 401 на всех запросах Мониторинга.
- Бот стартует `go client.Start(ctx)` (long-polling) И HTTP-сервер в одном
  процессе. Защита от двойного запуска — `flock /tmp/analyzpro.lock`
  (`app.go: acquireInstanceLock`). НЕ поднимать 2 реплики/контейнера
  (предыдущий инцидент «дубль уведомлений» был от двух инстансов с одним
  токеном). В systemd `Restart=always` — ок; в Docker — 1 контейнер, не scale>1.

### П5. Персистентное хранилище (иначе все данные теряются при рестарте)
Бот пишет в RELATIVE-пути, резолвящиеся относительно `WorkingDirectory`:
- `DB_PATH` (дефолт `./data/analyzpro.db`) — SQLite, реальная БД (профили,
  диагнозы, курсы, преференции, история мониторинга); миграции накатываются
  автоматически в `db.Migrate` при старте.
- `./data/premium_users.json` (статус Premium, `app.go:103`).
- `./data/states.json` (`states.NewMemoryStateManager`).
- `./data/agreements.json` (`NewAgreementStorage`).
- `./data/analytics.jsonl` (`analytics.Init`, `AnalyticsPath`).
- `UPLOAD_DIR` (дефолт `./uploads`).
Действие:
- Docker: монтировать VOLUME `/app/data` и `/app/uploads` (Dockerfile уже
  объявляет VOLUME; WORKDIR=/app, не-root user `analyzpro` владеет /app/data).
- systemd: `WorkingDirectory=/app`, убедиться что `/app/data` и `/app/uploads`
  существуют и принадлежат `User=vladislav` (иначе SQLite/WAL упадёт на запись).
- Задать `DB_PATH` явно на персистентный том. Проверить после рестарта:
  профили/Premium/согласия на месте.

---

## 🟡 ВАЖНО (без этого юзеры не смогут купить Premium / UX-потери)

### В1. `WEBAPP_PREMIUM_LINK` — задать и сделать рабочим (см. П1)
Читается только в `internal/monitoring/api.go:37` и отдаётся фронту через
`/api/monitoring/status` как `premiumLink`. В коде он НЕ в `config.go` — берётся
прямо из env. Без реальной оплаты (П1) эта ссылка бесполезна. В проде задать
реальную ссылку на оплату (YooKassa checkout / Stars-инвойс / вход в бот).

### В2. Заглушка Мониторинга для Free НЕ кликабельна (теряется конверсия)
`internal/monitoring/webapp_files/index.html` (плашка `freeStubPlaque`) — статичный
`<div>`, в `app.js: showFreeStub` на неё не навешан обработчик; `__premiumLink`
(из `/api/monitoring/status`) в ветке заглушки не используется. Free-юзер видит
«💎 Купите Premium, чтобы открыть Мониторинг...», но кликнуть нельзя — попасть
к оплате можно только из меню бота 💎 Premium. Исправить до того, как Мониторинг
станет точкой продаж: сделать плашку кликабельной (аналог `openPremium` из
«Моего профиля») или добавить кнопку «💎 Открыть Мониторинг» →
`tg.openTelegramLink(__premiumLink)`.

### В3. `ANTHROPIC_API_KEY` — обязателен для анализов
`internal/ai/claude/client.go`: пустой ключ → вызовы возвращают ошибку, но бот
стартует. Без него НЕ работают: расширенный анализ (Premium), Bioscan PRO,
обычный/базовый анализ (AI-клиент общий). Задать в `.env` прод-ключ. Проверить
на тестовом файле: отчёт приходит, а не `MsgTextProcessingError`.

### В4. `HTML2PDF_API_KEY` — для PDF-отчётов (опционально, но желательно)
Без ключа отчёты расширенного анализа/Bioscan PRO уходят как HTML (функционально
работает, но менее презентабельно). Если нужен PDF — задать ключ html2pdf.app.

### В5. `LOG_LEVEL` — выставить в проде
Дефолт `INFO` (`config.go`). Логи идут в stdout (перенаправлены в slog через
`logging.SetupLogging`). Для прод-тишины можно `WARN`, но INFO полезен при
разборе инцидентов. Оставить `INFO` на старте.

---

## 🟢 МОЖНО СДЕЛАТЬ ПОТОМ (не блокирует запуск)

- **M2 PostHog vs соглашение.** `analytics.InitPostHog(cfg.PostHogAPIKey)` шлёт
  события (вкл. chat ID) в app.posthog.com при заданном `POSTHOG_API_KEY`. В
  соглашении (`agreement.go`) написано «данные не передаются третьим лицам».
  Решение: либо не задавать `POSTHOG_API_KEY` в проде (no-op), либо поправить
  формулировку соглашения про аналитику. Не влияет на работоспособность.
- **L1 Опечатка** «Рекомендуемобратиться» в `MsgNotifAnalyticsDeviationList`
  (`internal/locales/messages.go`) — пропущен пробел. Косметика.
- **L2 Метка версии соглашения** «1.0» → «2.0» (`agreement.go`), если требуется
  по документации.
- **L3 Версии активов** `index.html` ссылается на `app.v42.js`, Go-константа
  `v43`. `ServeWebApp` резолвит любую версию, кэш сбрасывается корректно; поднять
  до v43 для единообразия.
- **L4 Индекс `(premium_expires_at)`** не создан — формально не выполнено, но
  проверка подписки грузит всех юзеров (`GetAllUsers`), индекс не использовался бы.
- **L5 Демо-график заглушки Мониторинга** не подписан «(пример)» — добавить
  подпись для ясности.

---

## ЧТО ПРОВЕРИТЬ

### Переменные окружения (минимум для прод-запуска)
В `.env` (systemd `EnvironmentFile=/app/.env`; Docker — env/secret):
- `BOT_TOKEN` — ❗ боевой токен (П4).
- `APP_ENV=production` — ❗ (П2).
- `WEBAPP_URL=https://<домен>/dashboard` — ❗ (П3).
- `DASHBOARD_URL=https://<домен>/dashboard` — опц. (авто=WEBAPP_URL).
- `DB_PATH=/персистентный/путь/analyzpro.db` — ❗ (П5).
- `HTTP_ADDR=:8080` — дефолт ок.
- `UPLOAD_DIR=/app/uploads` — (П5, volume).
- `ANTHROPIC_API_KEY` — ❗ для анализов (В3).
- `HTML2PDF_API_KEY` — опц. PDF (В4).
- `WEBAPP_PREMIUM_LINK` — ❗ только после П1 (В1).
- `PROMO_CODES` / `PROMO_CODES_MONTHLY` — опц. (промокоды на Premium).
- `POSTHOG_API_KEY` — опц.; лучше не задавать (M2).
- `LOG_LEVEL=INFO` — (В5).
- `ADMIN_CHAT_ID` — опц. (админ-команды сброса/анонсов).
- `LOADINGER_STICKER_ID` — опц. (анимация загрузки).
- `USE_MOCK` — НЕ задавать / `false` в проде (иначе мок-хранилище без БД).

### База данных (миграции, индексы)
- ✅ Миграции применяются автоматически (`db.Migrate`, `ensureNotificationSchema`)
  при старте — таблицы `users` (с `premium_expires_at`, `notifications_enabled`
  через preferences), `subscription_notifications`, `notification_suppressions`,
  `used_promocodes` создаются идемпотентно.
- ⚠️ Проверить, что `DB_PATH` указывает на ПЕРСИСТЕНТНЫЙ том (П5) и файл `+`-writable
  для пользователя процесса.
- ⚠️ SQLite + WAL: пул 8 соединений (`db.go`), `busy_timeout(5000)`. Убедиться, что
  диск быстрый и не пересекается с сетевым (NFS) — иначе lock-конфликты.
- ⚠️ Индекс `(premium_expires_at)` отсутствует (L4) — не критично (см. выше).

### Платежи (ЮKassa или Telegram Stars)
- ❌ **Реальной интеграции НЕТ** — только мок (`mock_yookassa.go`). Это блокер П1.
- Текущий in-bot флоу: выбор тарифа → `CreatePayment` → кнопка «Оплатить» ведёт
  на мёртвый `pay.test` → кнопка «✅ Оплатил (симуляция)» даёт Premium бесплатно.
- Действие: реализовать YooKassa (реальный `CreatePayment` + роут вебхука +
  `WEBAPP_PREMIUM_LINK`) ИЛИ Telegram Stars (инвойс + `successful_payment`) ИЛИ
  убрать симуляцию и раздавать Premium промокодами. Без этого — в проде Premium
  либо бесплатен для всех, либо недоступен.

### Тестовые команды (чтобы не светились в проде)
- ✅ Гейт корректен: `setupCommands` и `registerHandlers` добавляют `/test_*`
  только при `appEnv=="development"`; меню «🧪 Тест уведомлений» — только при
  `SetDevMode(true)`.
- ⚠️ Проверить в проде (после П2): `APP_ENV=production` → `getMyCommands` НЕ
  содержит `/test_sub_7d` и т.д., и в меню «Сервис» нет кнопки теста. Если
  `APP_ENV` забыт — dev-меню активно (H1 из аудита).

### Сборка и деплой
- ✅ `make build` → `bin/analyzpro` (`./cmd/bot`); `go build ./...` зелёный.
  Для статического бинаря (как в Dockerfile) нужен `CGO_ENABLED=0`
  (Makefile `build` его НЕ ставит — для systemd на Linux обычный build ок, но для
  alpine-образа лучше собрать с CGO_ENABLED=0).
- ✅ `Dockerfile`: multi-stage, статичный бинарь, non-root `analyzpro`, VOLUME
  `/app/data`+`/app/uploads`, `EXPOSE 8080`, `HEALTHCHECK` на `/healthz`
  (роут есть в `bot.go`). Собрать: `docker build -t prisma .`
- ✅ `analyzpro.service` (systemd): `Type=simple`, `Restart=always`,
  `WorkingDirectory=/app`, `EnvironmentFile=/app/.env`. Один инстанс (flock).
- ⚠️ Нужен reverse-proxy TLS (Caddy/nginx) 443→:8080 (П3). Сам бот TLS не делает.
- ⚠️ Перед деплоем: `git checkout -- internal/locales/messages.go` (риск повреждения
  литерала в рабочем дереве, по памяти проекта) и НЕ трогать
  `internal/bot/handlers/dashboard/dashboard.go` (демо-правки только в дереве).

### Логи и мониторинг
- ✅ `logging.SetupLogging(LOG_LEVEL)` → stdout; `healthz` на `/healthz` для
  оркестратора.
- ⚠️ Настроить агрегацию логов: `journalctl -u analyzpro` (systemd) или
  `docker logs` + форвард в Loki/CloudWatch. Логи содержат `chatID` (замаскирован
  частично через `MaskID`) — не светить в публичный доступ.
- ⚠️ Алерты: падение инстанса (restart loop), рост `SEND OK` дублей (признак
  второго инстанса), ошибки миграции БД при старте.

### Безопасность (секреты, доступы)
- ✅ Секреты в `.env` (не в репо). Убедиться, что `.env` права `600`, не
  world-readable, не в образе Docker (монтировать, а не COPY).
- ✅ initData Мониторинга валидируется бот-токеном (`monitoring/api.go: auth`) —
  совпадение `BOT_TOKEN` обязательно (П4).
- ✅ Один инстанс через flock — защита от конкурентного long-polling (П4).
- ❌ Кнопка «✅ Оплатил (симуляция)» — бизнес-дыра: бесплатный Premium для любого.
  Убрать в рамках П1.
- ⚠️ PostHog шлёт данные третьей стороне (M2) — либо отключить, либо поправить
  соглашение.

---

## ДОПОЛНИТЕЛЬНО (чего НЕТ в коде — надо добавить)

1. **Реальная интеграция платежей — ЭТОГО НЕТ.** Ни YooKassa (боевой вызов +
   вебхук-роут), ни Telegram Stars (инвойс + обработка `successful_payment`).
   `CreatePayment`/`HandleWebhook` существуют, но: первый возвращает фейк-URL,
   второй не смонтирован, а активация идёт по кнопке-симуляции. Без этого пункта
   бот в проде либо не продаёт Premium, либо раздаёт его бесплатно. **Блокер.**

2. **Фоллбэк-оплата из заглушки Мониторинга — ЭТОГО НЕТ.** Плашка Free-заглушки
   не кликабельна (В2). Нужен обработчик клика по `__premiumLink`.

3. **Healthcheck/docker-compose — частично.** `Dockerfile` готов, ноcompose-файла
   в репо нет (проверить, если используется оркестрация). Reverse-proxy (Caddy/nginx)
   конфигурации в репо тоже нет — его надо добавить/настроить на сервере (П3).

4. **`.env.example` заблокирован для чтения агентом** (доступ к credential-файлам
   закрыт) — убедиться, что в репо есть корректный шаблон `.env.example` с ВСЕМИ
   переменными из раздела «Переменные окружения» выше, чтобы деплой не гадал.

---

## ФИНАЛЬНЫЙ ВЕРДИКТ (DevOps)

❌ **FAIL — ДЕПЛОЙ ОТКЛАДЫВАЕТСЯ** до реализации реальных платежей.

Критический блокер один, но фатальный: в коде нет боевой оплаты (`internal/payment`
= мок), а кнопка «✅ Оплатил (симуляция)» выдаёт Premium бесплатно любому. Без П1
бот в проде либо не монетизируется, либо раздаёт Premium задаром, либо (если
убрать симуляцию, не добавив платёж) не даёт купить Premium вообще.

Обязательно ДО релиза: П1 (платежи) + П2 (`APP_ENV=production`) + П3 (HTTPS/домен) +
П4 (боевой токен, 1 инстанс) + П5 (персистентный том). Желательно В1–В5.

После закрытия П1–П5 бот технически готов к запуску; В2 (кликабельность заглушки)
и M2 (PostHog/соглашение) докрутить в первом спринте после релиза.
