# Prisma (бывш. AnalyzPRO) — устойчивые факты

## Общение
- С этим пользователем общаться на русском языке.

## Проект (Prisma — Telegram-бот, Go; ранее AnalyzPRO)
- **РЕБРЕНДИНГ:** публичное имя — **Prisma** (убрано из пользовательских текстов). Нет упоминаний ИИ/AI/нейросети — «аналитический помощник Prisma». ⚠️ Менять ТОЛЬКО пользовательские строки; Go-импорты/пути (`github.com/theamornoir/analyzpro`, `bin/analyzpro`, `/tmp/analyzpro.lock`, `./data/analyzpro.db`) НЕ трогать.
- **Версия Mini App = `WebAppAssetsVersion`** (в пакете `keyboards`, сейчас **"v37"**) — поднимать при каждой правке фронта `webapp_files/{index.html,app.js,style.css}`. Активы грузятся по ВЕРСИОНИРОВАННОМУ ПУТИ (`style.<ver>.css`/`app.<ver>.js`), `ServeWebApp` резолвит любую версию. `Cache-Control: no-store` + отрезание `?v=`.
- `make mini` — бот + HTTPS-туннель (`WEBAPP_URL=<туннель>/dashboard`); Menu Button НЕ открывает дашборд; Мой профиль — только из «📊 Здоровье» → хаб → «Открыть». Туннель ngrok по умолчанию (лимит 1 агент), `TUNNEL=cloudflared make mini` без лимита. Экран Telegram «you are about to visit <domain>» — штатное подтверждение.

## Онбординг + сброс
- 8 шагов слайдера → соглашение (`internal/locales/agreement.go`) → inline «✅ Принять». Статус `User.OnboardingCompleted`; `SetOnboardingCompleted`/`IsOnboardingCompleted`. Новый (OnboardingCompleted=false) → `onboarding.SendStep`; уже-согласный без онбординга → помечается пройденным и в меню; пройденный → меню.
- Сброс (ADMIN_CHAT_ID): `/resetme`, `/reset_premium` → `ResetHandler`: `payment.ResetPremium`, `SetOnboardingCompleted(false)`, `agreementStorage.Reset`, `stateManager.Reset`. **Чтобы увидеть онбординг:** `/resetme` затем `/start`.

## Мой профиль (Mini App, /dashboard) + Мониторинг (/monitoring)
- `GET /api/metrics` всегда 200 + `premiumRequired`; богатые метрики скрыты для не-Premium, `noData` виден всем. `POST /api/profile` (Type `questionnaire`) снимает `noData`.
- Сохраняются ВСЕ 4 типа результата в `monitoring.Repository`: расширенный/обычный анализ, Bioscan PRO, базовый Bioscan. После каждого — `MsgResultSavedSummary` + кнопка `BtnOpenHealthSummary` (WebApp `/dashboard`, без Premium).
- Удаление: `DELETE /api/reports/delete?id=&initData=` (`dashboard.Handler.DeleteEntry`, владение + initData; запрещён в демо `?demo=1`). Метод `DeleteHistoryEntry` во всех 3 реализациях репозитория.
- Демо: хаб «Анализы» сдвоен с `🧪 Демо` (router_demo.go, синтетика БЕЗ ИИ/Premium, результаты НЕ сохраняются, но шлёт `MsgDemoResultSavedSummary` + демо-Сводка `/dashboard?demo=1`). `GET /api/reports/file?demo=1&type=&id=&view=inline` учитывает `id`.
- Мониторинг API под initData: `NewAPIHandler(monitorSvc, botToken)`, `/api/monitoring/`. `PROJECT_TYPES` = course/diabetes/weight/health/other. ⚠️ Эмодзи НЕ в `TYPE_LABELS` (уже в `TYPE_ICON`).

## Стикер загрузки
- `helpers.SendLoadingMessages(ctx,b,chatID,stickerID,steps)`. `LOADING_STICKER_ID` опционален; если пуст/плейсхолдер `your_sticker_id` — встроенная анимация `SendChatAction(UploadDocument)` + цикличный текст.

## AI-слой (единый Claude-клиент)
- `internal/ai/claude/client.go`: `Client`, `NewClient()` (ключ `ANTHROPIC_API_KEY`; пустой → вызовы возвращают ошибку, бот стартует). Метод `GenerateWithFiles(ctx, systemPrompt, prompt, files []Attachment, maxTokens)` — промпт + ВСЕ файлы в ОДНО сообщение (image-блок/PDF document-блок). Модель `claude-3-5-sonnet-20241022`, таймаут 120с, прокси `HTTP(S)_PROXY`.
- **AI-СЕМАФОР:** `var sem = semaphore.NewWeighted(8)` (golang.org/x/sync). В начале `GenerateWithFiles`: `sem.Acquire(ctx,1)` + `defer sem.Release(1)`. Лимит 8 одновременных запросов к Claude — защита от 429 и OOM.

## Аналитика (PostHog)
- `github.com/posthog/posthog-go`. Ключ `POSTHOG_API_KEY`; пустой → no-op. Глобальный `InitPostHog`/`Track`/`ClosePostHog` (async Enqueue). Имя события = русская подпись (labels.go), тех. ключ в `action`/`source`. Подключено в `router.handle` (каждый clickstream) + предметные события.

## Деплой / прод
- Прод: свой домен HTTPS + `WEBAPP_URL=https://<домен>/dashboard`; `analyzpro.service` (systemd, flock `/tmp/analyzpro.lock`).

## Промокоды на Premium (команда /promo)
- Одноразовая активация на 365 дней. Коды в env `PROMO_CODES` (через запятую) → `config.PromoCodes []string`. `/promo <code>` (MatchTypePrefix) → `Bot.handlePromo`: нормализация → проверка в списке → `Users.IsPromoCodeUsed` → `paymentService.ActivatePremiumManually(chatID, "premium_yearly")` + `UpdateUserPremiumStatus`(is_premium=1, +1 год) + `MarkPromoCodeUsed`. Таблица `used_promocodes (user_id, code, used_at, UNIQUE(user_id,code))`. ⚠️ Коды ТОЛЬКО в `.env`.

## Бот: ошибки / UX
- `sendAnalysisError` (upload): удаляет loading, `stateManager.Reset`, `MsgTextProcessingError` + `MainMenu()`.
- Хаб разделов — ДВА сообщения: «якорь» (описание + reply `[Назад]`) и «блок» (inline-кнопки, без «Назад»). Кнопка «← Назад» (`back_to_main`) возвращает в главное меню.

## Навигация Premium (важно при правке!)
- Экран Premium — ДВА сообщения: `premium_anchor_id` («якорь» 💎 Premium + reply `[Назад]`) и `premium_msg_id` (список тарифов / экран оплаты / подтверждение).
- **ТРЕКИНГ id экрана Premium ВЫНЕСЕН в ОТДЕЛЬНЫЙ map `MemoryStateManager.premiumScreen` (методы `Set/Get/ClearPremiumScreenID` в интерфейсе `StateManager`), персистится в `states.json` и пишется СИНХРОННО.** НЕ лежит в `m.data` (user-data), потому что `stateManager.Reset` целиком стирает `m.data`, а запись user-data — асинхронная (`go m.save()`), терялась при kill/перезапуске. Отдельный map переживает `Reset` И перезапуск бота → старое сообщение Premium в чате больше не «висит» навсегда. Это РЕАЛЬНЫЙ корень «висящего Premium».
- Очистка — `clearPremiumScreen` (`internal/bot/handlers/menu/premium.go`): читает id СНАЧАЛА из нового map, затем fallback из legacy user-data (миграция старых висящих экранов), удаляет оба сообщения, затем `ClearPremiumScreenIDs` + сброс legacy-ключей. Вызывается ПЕРЕД каждым показом (вход/PremiumHandler, смена тарифа HandleChangeTariff, confirm) и из `backToParent`/`start.go`/`reset.go`.
- Кнопка `💎 Premium` идёт ТОЛЬКО через роутер (`router_menu.go` case BtnPremium) → `current_section="premium"` + убирает закреплённое меню. НЕЛЬЗЯ регистрировать как отдельный `RegisterHandler` (иначе current_section не выставляется → «Назад» оставляет экран висеть).
- На выходе «Назад»: `backToParent` (`router_back.go`) чистит экран Premium по `premiumScreenTracked()` (факт отслеживания id в новом map ИЛИ legacy user-data) — устойчиво к рассинхронизации. Затем `showMainMenuMessage`.
- `/start` (`StartHandler`) и `/resetme` (`ResetHandler`) вызывают `clearPremiumScreen` СТРОГО ПЕРЕД `stateManager.Reset`. Теперь id в отдельном map, так что даже при нарушении порядка они не теряются, но порядок сохранён для надёжности.
- **ТЕСТЫ навигации:**
  - `router_premium_test.go` — интеграционный (реальный `tgbot.New` + httptest mock Telegram, sendMessage отдаёт инкрементный message_id, deleteMessage запоминает удалённые id): 💎 Premium → premium_<id> → premium_confirm_<id> → 💎 Premium → premium_change → Назад. `go test ./internal/bot/handlers/router/ -run TestPremiumScreenCleanedOnBack -v`.
  - `router_premium_delete_test.go` — юнит `backToParent` удаляет якорь+список (mock парсит multipart-форму для deleteMessage, иначе MessageID=0 и тест ложно падает).
  - `internal/bot/states/manager_test.go` — `TestPremiumScreenSurvivesReset` (id переживают `Reset`) и `TestPremiumScreenPersistedAcrossReload` (id переживают перезапуск/файл).

## Готчи при правке
- ⚠️ `internal/locales/messages.go` может быть ПОВРЕЖДЁН в рабочем дереве (обрезанный литерал) → `git checkout -- internal/locales/messages.go`.
- ⚠️ Демо-правки `dashboard.go` живут ТОЛЬКО в рабочем дереве — **НЕ `git checkout -- internal/bot/handlers/dashboard/dashboard.go`**.
- ⚠️ `internal/db/db.go` `Migrate` использует WAL + `busy_timeout(5000)` + `synchronous(NORMAL)`; пул `SetMaxOpenConns(8)`/`SetMaxIdleConns(8)`. ПРАВИЛО: в миграциях `conn.Query(...)` БЕЗ `rows.Close()` → утечка соединения → deadlock. Либо `QueryRow().Scan()`, либо `rows.Close()`.
- ⚠️ `WebAppAssetsVersion`/`WithWebAppVersion`/`OpenHealthSummaryButton` — в пакете `keyboards` (не в router_menu_actions).

## Стиль пользовательского текста
- Без длинного тире `—` (только дефис -); без `*`/`**` для жирности; текст чистый, не «сгенерированный ИИ».
- Расширенный анализ = Premium (Premium-гейт в `handleExtendedAnalysis`); обычный бесплатен. Демо-путь бесплатный.

## Опросники
- Расширенный анализ (Premium): 20 вопросов `internal/bot/handlers/userdata/`. Bioscan PRO (Premium): 18 вопросов `bioscan_questionnaire.go`. Базовый Bioscan/обычный анализ опросники НЕ используют.

## Premium-плашка в Сводке (Mini App, без кнопок)
- Нет Premium → `#messageCard`/`#messageText` (функция `showPremiumRequired`): «💎 Для просмотра «Мой профиль» нужна Premium-выподска. Оформите её в боте - нажмите кнопку «💎 Premium» в меню.» Без deep-link `/start premium`.

## Напоминания (internal/bot/reminders)
- `RunReminderLoop` (goroutine в app.Run, раз в 6ч) по `users.last_activity_date`: ≥10 дней неактивности → `GetRandomReminder()` из пула `locales.Reminders` (≤200 символов); ставит `last_activity_date=now` (дебаунс). Уважает `Preferences.NotificationsEnabled`. `BroadcastFeature` — одноразовая рассылка.
