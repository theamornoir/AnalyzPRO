# Prisma (бывш. AnalyzPRO) — устойчивые факты

## Общение
- С этим пользователем общаться на русском языке.

## Проект (Prisma — Telegram-бот для анализа мед. анализов, Go; ранее AnalyzPRO)
- **РЕБРЕНДИНГ:** публичное название — **Prisma** (старое `AnalyzPRO` убрано из пользовательских текстов). Убраны упоминания ИИ/AI/нейросети — бот = «аналитический помощник Prisma». ⚠️ Меняй ТОЛЬКО пользовательские строки; Go-импорты/пути (`github.com/theamornoir/analyzpro`, `bin/analyzpro`, `/tmp/analyzpro.lock`, `./data/analyzpro.db`) НЕ трогай. Версия Mini App = `WebAppAssetsVersion` (сейчас **"v20"**).
- `make mini` поднимает бота + HTTPS-туннель (`WEBAPP_URL=<туннель>/dashboard`); `make mini-stop` убивает по lock-PID `/tmp/analyzpro-mini.lock/pid`. Menu Button НЕ открывает дашборд (сброс в `bot.go`); Сводка открывается ТОЛЬКО из клавиатуры «📊 Здоровье» → хаб → «Открыть». Туннель по умолчанию ngrok (лимит 1 агент → `err_ngrok_3200`); `TUNNEL=cloudflared make mini` — без лимита.
- Экран Telegram «you are about to visit <domain>» — НЕ ошибка, штатное подтверждение; нажимать «Открыть».

## Онбординг (слайдер) + команда сброса (актуально)
- 8 шагов онбординга (подробный слайдер: база/анализы/Bioscan basic → Premium/Bioscan PRO/Сводка/Мониторинг/Консультация/Сервис; inline-кнопки «➡️ Дальше», на 8-м — «📝 Соглашение») → текст соглашения (v1.0, `internal/locales/agreement.go`) → inline «✅ Принять».
- Статус: `models.User.OnboardingCompleted bool` (JSON `onboarding_completed`); интерфейс `UserRepository.UpdateUserOnboardingStatus`; реализован в mock/file/sql репозиториях; хелперы `storage.SetOnboardingCompleted` / `IsOnboardingCompleted`.
- Поток: `menu.StartHandler` — новый (OnboardingCompleted=false) → `onboarding.SendStep(ctx,b,chatID,1)`; существующий уже-согласный (agreed && !onboarded) → помечается пройденным и сразу в меню (миграция, чтобы не гонять старых юзеров); пройденный → главное меню.
- Шаги/соглашение = отдельные сообщения; `router.handleOnboarding` при переходе делает `helpers.DeleteMessage` предыдущего (чтобы не плодить историю). Callback'и: `onboarding_step_2..N` (N = len(Steps) в `onboarding.go`), `onboarding_agreement`, `onboarding_accept` (финал ставит `agreementStorage.SetAgreed` + `SetOnboardingCompleted(true)` + главное меню). Файлы: `internal/bot/handlers/onboarding/onboarding.go`, `internal/locales/onboarding.go`.
- Команда сброса (только ADMIN_CHAT_ID): `/resetme` и `/reset_premium` → `menu.ResetHandler`: `payment.ResetPremium`, `SetOnboardingCompleted(false)`, `agreementStorage.Reset` (сброс соглашения, чтобы онбординг проходился заново), `stateManager.Reset`. Текст: `locales.MsgResetDone`. Зарегистрировано в `bot.go`.
- ТЕСТИРОВАНИЕ ОНБОРДИНГА: разработчик — уже согласившийся пользователь, поэтому `/start` по логике миграции сразу даёт главное меню (онбординг не показывается). Чтобы увидеть слайдер: набрать `/resetme` (ответ «Статус Premium и онбординг сброшены…») и затем `/start`. Без `/resetme` онбординг не появится.

## Сводка здоровья (Mini App, /dashboard)
- Дашборд `/dashboard` (`internal/bot/handlers/dashboard`) читает `monitoring_history` (данные переживают рестарт). Файлы: `internal/bot/handlers/dashboard/webapp_files/{index.html,app.js,style.css}` (go:embed). `Cache-Control: no-store` + отрезание `?v=`. WebApp-URL версионируется `?v=<WebAppAssetsVersion>`. При правке фронта ОБЯЗАТЕЛЬНО увеличить `router.WebAppAssetsVersion`; `<script>` только с `defer`; редиректы `/dashboard`/`/monitoring` — только `307`.
- Премиум-гейт: `GET /api/metrics` всегда 200 + `premiumRequired`; богатые метрики скрыты для не-Premium, `noData` виден всем. Онбординг всем: профиль `POST /api/profile` (Type `questionnaire`) снимает `noData`. ⚠️ `MockPaymentService` НЕ синхронизирует Premium из Telegram — только запись в `./data/premium_users.json`.
- `getInitData()` (v7) читает initData из `window.Telegram.WebApp.initData` → `tgWebAppData` query → `tgWebAppData` hash, перечитывает на каждом вызове. `chart.js`/`app.js` грузятся с `defer`; `#registerCard` видим по умолчанию; `submitProfile` навешивается один раз при загрузке скрипта.

- **ДЕМО-РЕЖИМ (актуально, v16):** «🧪 Демо-Сводка»/«🧪 Демо-Мониторинг» открывают `/dashboard`/`/monitoring` с `?demo=1` (минуя Premium, без реальных данных). Сводка: `GET /api/metrics?demo=1` → `buildDemoMetrics()`; демо-отчёты в «Истории отчётов» имеют `ID` (analysis 1/2/3, bioscan 10/11/12) и разные даты.
  - **ОТКРЫТИЕ ДЕМО-ОТЧЁТА:** клик по «📄 PDF»/строке архива шлёт `GET /api/reports/file?demo=1&type=<kind>&id=<r.id>&view=inline` (без initData). Бэкенд в демо-ветке `ReportFile` **учитывает `id`**: `buildDemoReportHTML(kind, id)` → `demoReportBlock(kind, id)` ищет блок в `buildDemoReports()` по `ID` (дата/баллы/показатели), а не «последний». `view=inline` → бэкенд отдаёт HTML (iframe внутри Mini App, без окна «посетить сайт»). Тест `TestReportFileDemoRespectsID` фиксирует разные даты по разным id.
  - ДЕМО-МОКИ анализов/Bioscan: хаб «Анализы» сдвоен с `🧪 Демо: …` (callback `section_diag_regular_demo` и т.д. → `router_demo.go`), шлют синтетику БЕЗ ИИ/Premium. Шаблоны `internal/report` используют `{{.Title}}` → в `models.Report` есть поле `Title string` (JSON `title`).

- **Отчёт Bioscan PRO = «Body Intelligence» (HTML, не PDF):** `report.Renderer.RenderBodyScan` + `templates/body_scan_report.html`. Поле `TrainingProgram []BodyScanTrainingPhase` (JSON `training_program`) в `models.BodyScanReport`. Хелперы рендера (`renderer.go`): `donutDash`, `bodyStatusColor/Class/Text`, `postureRadar`, `zoneDonuts`, `nl2p`. Весь SVG/текст inline, A4 `@page size`.

## Мониторинг (Mini App, /monitoring)
- Файлы: `internal/monitoring/webapp_files/{index.html,app.js,style.css}` (go:embed, `ServeWebApp`). Отдаётся с `Cache-Control: no-store` — поэтому внутренний `?v=` в тегах активов НЕ обязателен для сброса кэша (но версию `v9→v10` всё же подняли для надёжности; менять синхронно в `style.css`/`app.js` тегах index.html).
- ⚠️ ПАТТЕРН ЭМОДЗИ: тип проекта рендерится как `TYPE_ICON[p.type] + " " + TYPE_LABELS[p.type]`. Эмодзи НЕ должно быть в `TYPE_LABELS` (оно уже есть в `TYPE_ICON`) — иначе дублируется («💉 💉 Курс препаратов»). Демо-проекты (`DEMO_PROJECTS`) тоже НЕ должны нести эмодзи в `name` (иконка типа показывается отдельно).
- API под защитой initData: `NewAPIHandler(monitorSvc, botToken)`, префикс `/api/monitoring/`. Проекты/записи/графики. `PROJECT_TYPES` = course/diabetes/weight/health/other.

## Индикатор загрузки (стикер/анимация) при анализе, Bioscan, консультации
- `helpers.SendLoadingMessages(ctx,b,chatID,stickerID,steps)` — единая точка индикатора ожидания. Вызывается из `upload_analyze.go`, `upload_text.go`, `bioscan_process.go`, `bioscan_basic.go`, `router_menu_actions.go` (консультация/сводка).
- `LOADING_STICKER_ID` (env) — **опционален**. Если пуст ИЛИ равен плейсхолдеру `your_sticker_id` (из `_env_example_tmp`) — стикер НЕ шлётся; вместо него `SendLoadingMessages` показывает ВСТРОЕННУЮ анимацию Telegram `SendChatAction(ChatActionUploadDocument)` («отправка документа», цикл ~4с) + циклически меняющий текст (фразы из `steps`, каждые 2с; дефолт `defaultLoadingSteps` из `locales/helpers.go`; для Bioscan — `bioscanSteps` из `locales/keyboards.go`). Гасится через `SafeDeleteLoadingMsgs`/`DeleteMessage(textMsg.ID)` → `CancelLoadingAnimation`.
- Чтобы показывался анимированный СТИКЕР (а не только встроенная анимация) — нужно задать реальный `LOADING_STICKER_ID` (file_id стикера, полученный от BotFather/через getFile).

## AI-прокси / провайдеры (кратко)
- `internal/ai/httpclient/client.go` — `AIHTTPClient` (прокси: `GEMINI_PROXY` → env → системный → direct). `FetchWithRetry` возвращает реальный `HTTPError{StatusCode,Message}` (повтор только 5xx/429).
- **OpenRouter — основной free-провайдер** (`openrouter.go`): text+vision+PDF (PDF → `ledongthuc/pdf` в текст). Дефолт `google/gemma-4-26b-a4b-it:free`; фоллбэк-цепочка `openRouterFallbackModels` в коде; таймаут (40с/модель) тоже триггерит фоллбэк. Бесплатные модели часто 429 (shared pool) — решение BYOK. ⚠️ Правка `openrouter.go` капризна (em-dash/стрелка): проверяй реальный диск `grep`/`sed`, правь Python с `assert old in s`, потом `gofmt -w`.
- Gemini: модели 404 для новых ключей → авто-фоллбэк `candidateModels()` (дефолт `gemini-2.5-flash-latest`). DeepSeek НЕ умеет vision (400 на фото). YandexGPT/Gemini/Claude — падают (folder-id / гео-блок / кредиты).

## Деплой / прод
- Прод: свой домен HTTPS + `WEBAPP_URL=https://<домен>/dashboard`; есть `analyzpro.service` (systemd, flock `/tmp/analyzpro.lock`).

## Бот: ошибки обработки / UX
- `sendAnalysisError` (пакет `upload`) — при провале анализа удаляет loading, `stateManager.Reset`, шлёт `MsgTextProcessingError` + `MainMenu()`.
- Хаб разделов — ДВА сообщения: «якорь» (описание + reply `[Назад]`) и «блок» (inline-кнопки + «👇 Выберите действие:», без «Назад»).

## Готчи при правке (сеанс 2026-08-17)
- ⚠️ `internal/locales/messages.go` может оказаться ПОВРЕЖДЁН в рабочем дереве: обрезанный строковой литерал (напр. `MsgUploadDossierCaption` без закрывающей кавычки) → сборка падает `newline in string` в `messages.go:229`. Это НЕ пользовательские правки — `git checkout -- internal/locales/messages.go`.
- ⚠️ Крупные незакоммиченные правки `dashboard.go` (демо `?demo=1`, `view=inline` в `ReportFile`, `id` в `buildDemoReportHTML`/`demoReportBlock`, `buildDemoReports` с `ID`) живут ТОЛЬКО в рабочем дереве. **НЕ делай `git checkout -- internal/bot/handlers/dashboard/dashboard.go`** — сотрёт эти фичи (HEAD их не содержит). Правь поверх текущего состояния, проверяя реальный диск `grep`/`sed` (`read_file` может отдавать кэш).
- ⚠️ `internal/db/db.go` `Migrate` работает с `SetMaxOpenConns(1)` (SQLite). Любой `conn.Query(...)` в миграции БЕЗ `rows.Close()` «утекает» единственное соединение → СЛЕДУЮЩАЯ операция с БД зависает (deadlock пула, тест `sqlrepo` виснет на CreateUser). Проверено на практике: добавил `ALTER users ADD COLUMN` через `conn.Query("SELECT ...")` без Close → бот/тесты зависали. Правило: в миграциях либо `conn.QueryRow(...).Scan()`, либо обязательно `rows.Close()`.

## Пользовательский текст: стиль (актуально 2026-08-17)
- В пользовательских строках (локали `internal/locales/*`, отчёты `internal/report/*`, HTML/JS/CSS Mini App) НЕ использовать длинное тире `—` (U+2014) - только короткое дефис `-`. Просьба пользователя: текст не должен выглядеть сгенерированным ИИ. Звёздочки `*`/`**` для жирности не нужны (Telegram Markdown их не рендерит корректно) - текст чистый.
- **Расширенный анализ = Premium (актуально 2026-08-17):** `router.handleExtendedAnalysis` имеет Premium-гейт (как `handleBioscanExtendedStart`): без подписки посылает `locales.MsgExtendedAnalysisPremiumRequired` и НЕ запускает опросник. Упоминание «Расширенный анализ (Premium)» - в онбординге (шаг 1), `MsgAnalysisHubIntro`, `MsgDiagnosticsIntro`. Обычный анализ бесплатен. Демо-путь `section_diag_extended_demo` бесплатный (синтетика).

## Опросники (расширенный анализ и Bioscan PRO)
- **Расширенный анализ (Premium):** опросник из 20 вопросов в пакете `internal/bot/handlers/userdata/` (цепочка: Name→Gender→Age→Height→Weight→Sleep→Stress→NutritionVeg→NutritionProcessed→Water→Activity→ChronicDiseases→Allergies→Medications→Smoking→Alcohol→FamilyHistory→Digestion→SportType→Goal). Раньше ветки с болезнями (ChronicDiseases/Allergies/Medications/FamilyHistory/Digestion) были мёртвым кодом - переподключены в сеансе 2026-08-17 (Activity→ChronicDiseases, Alcohol→FamilyHistory, убрана мёртвая ветка on_course в `user_data_health.go`). Собранное попадает в промпт через `helpers.BuildAnalysisText`. Обычный анализ опросник НЕ использует (только файл).
- **Bioscan PRO (Premium):** отдельный опросник 18 вопросов про образ жизни/спорт/травмы/здоровье. Движок в `internal/bot/handlers/bioscan/bioscan_questionnaire.go` (слайс `bioscanQuestionnaire` + `HandleBioscanQuestionnaireState`), состояния `StateWaitingBioscan*` в `internal/bot/states/manager.go`, подключён в `router_bioscan.go`. Идёт ПОСЛЕ цели и ДО 4 фото. Ответы собираются в ключи `bioscan_*` и попадают в промпт PRO через `BuildBioscanText` (`bioscan_context.go`), который используется в `bioscan_process.go`. Промпт `PromptForBodyScanJSON` (правило 13) обязывает ИИ использовать опросник в программе тренировок/осанке/рекомендациях. Базовый Bioscan (1 фото) опросник НЕ использует.
- Тест целостности цепочки: `internal/bot/handlers/bioscan/bioscan_questionnaire_test.go`.

## Premium-плашка в Сводке (Mini App, без кнопки/переходов)
- При `premiumRequired` (нет Premium) Мини-апп «Сводка здоровья» показывает ТОЛЬКО плашку-сообщение: `#messageCard`/`#messageText`, функция `showPremiumRequired` в `app.js` выводит «💎 Для просмотра «Сводки здоровья» нужна Premium-подписка. Оформите её в боте - нажмите кнопку «💎 Premium» в меню.». НИКАКИХ кнопок и переходов - пользователь сам идёт в бота.
- Если Premium активна - эта ветка НЕ вызывается (`data.premiumRequired`=false), плашка не показывается.
- Кнопка «Открыть премиум-подписку» и весь deep-link `/start premium` УДАЛЕНЫ (пользователь счёл тот флоу «неправильным»). Вместе с ними убраны: `botUsername`/`client.GetMe` в `bot.go`, эндпоинт `/api/config` и метод `Config` в `dashboard.go`, параметр `paymentService` у `menu.StartHandler`, регистрация `/start` возвращена в `tgbot.MatchTypeExact`. Плашка рендерится из `index.html` (тег `<a id="messageAction">` удалён). Версия активов поднята `v19 -> v20`.
