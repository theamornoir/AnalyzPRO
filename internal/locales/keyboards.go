package locales

// Keyboard button texts
const (
	BtnProcessAnalysis = "✅ Обработать анализы"
	BtnBack            = "⬅️ Назад"
	BtnCancel          = "❌ Отмена"
	BtnAgreement       = "📝 Пользовательское соглашение"
	BtnAbout           = "ℹ️ О сервисе"
	// BtnAnalysisHub - верхняя кнопка-хаб «Анализы»: объединяет всё, что
	// связано с расшифровкой (лаб. анализы + фотографический Bioscan).
	BtnAnalysisHub = "📋 Анализы"
	BtnDiagnostics = "🏥 Диагностика анализов"
	BtnBioscan     = "📸 Bioscan"
	// Базовый (бесплатный) Bioscan: 1 фото -> текстовый результат в чат.
	BtnBioscanBasic = "🌐 Bioscan"
	// Расширенный (Premium) Bioscan PRO: 4 фото -> детальный PDF-отчёт.
	BtnBioscanExtended = "✨ Bioscan PRO"
	// BtnHealthHub - верхняя кнопка-хаб «Здоровье»: отслеживание состояния
	// (Мой профиль / Мониторинг) + консультация с ИИ.
	BtnHealthHub = "📊 Здоровье"
	// BtnServiceHub - верхняя кнопка-хаб «Сервис»: связь с разработчиком
	// (Отзывы) и информация о боте (О сервисе).
	BtnServiceHub = "⚙️ Сервис"
	// BtnHealthSummary - переименованный «Дашборд»: слово «дашборд»
	// пользователям непонятно, поэтому теперь это «Мой профиль» -
	// снимок состояния здоровья прямо сейчас.
	BtnHealthSummary = "📊 Мой профиль"
	// BtnHealthDynamics - новый раздел-хаб, объединяющий «Мой профиль»
	// и «Мониторинг» (отслеживание показателей в динамике).
	BtnHealthDynamics = "📈 Здоровье в динамике"
	BtnFeedback       = "📝 Отзывы и предложения"
	// BtnDeleteAccount - кнопка удаления аккаунта (раздел «Сервис»).
	// Полностью стирает все данные пользователя (профиль, анализы,
	// биосканы, мониторинг, уведомления) - необратимо.
	BtnDeleteAccount = "🗑 Удалить аккаунт"
	// BtnTestNotify - кнопка входа в тестовое меню проверки уведомлений
	// (раздел «Сервис»). Позволяет разработчику отправить себе образец
	// реального уведомления (напоминание/мотивация/фича) спустя 30 секунд.
	BtnTestNotify         = "🧪 Тест уведомлений"
	BtnTestSub7d          = "⏳ Подписка: за 7 дней"
	BtnTestSub3d          = "⚠️ Подписка: за 3 дня"
	BtnTestSub1d          = "🔴 Подписка: за 1 день (ЗАВТРА)"
	BtnTestSubToday       = "❌ Подписка: в день окончания"
	BtnTestAnalyticsCheck = "🔎 Анализы: проверить (предпросмотр)"
	BtnTestAnalyticsSend  = "📨 Анализы: отправить уведомления"
	// BtnConsultation - новый раздел «Быстрая консультация (с ИИ)».
	// Пользователю непонятно «консультация с ИИ», поэтому названо как
	// «Быстрая консультация»: можно прислать фото травмы или задать вопрос,
	// и ИИ даст консультацию с рекомендациями. Premium-функция (3 бесплатно).
	BtnConsultation = "💬 Быстрая консультация"
	// BtnConsultAgain - reply-кнопка «Задать ещё вопрос» в финиш-состоянии
	// консультации: позволяет продолжить диалог с ИИ (вернуться в
	// StateWaitingConsultation, не выходя в главное меню).
	BtnConsultAgain = "💬 Задать ещё вопрос"
	// BtnConsultFinish - reply-кнопка «Закончить консультацию» в
	// финиш-состоянии: единственный способ выйти из флоу и вернуться в
	// обычное главное меню. Находится в нижней reply-клавиатуре (не inline).
	BtnConsultFinish    = "✅ Закончить консультацию"
	BtnPremium          = "💎 Premium"
	BtnPremiumChange    = "🔄 Сменить тариф"
	BtnRegularAnalysis  = "📊 Обычный анализ"
	BtnExtendedAnalysis = "🩺 Общая оценка здоровья"
	// 🧪 Демо-кнопки для предпросмотра результатов «как заполнено»,
	// без реальных файлов и без вызовов ИИ. Работают всем (и без Premium).
	BtnRegularAnalysisDemo  = "🧪 Демо: Обычный"
	BtnExtendedAnalysisDemo = "🧪 Демо: Оценка здоровья"
	BtnBioscanBasicDemo     = "🧪 Демо: Bioscan"
	BtnBioscanExtendedDemo  = "🧪 Демо: Bioscan PRO"
	BtnAcceptAgreement      = "✅ Принять соглашение"
	BtnBioscanConfirm       = "✅ Подтвердить и проанализировать"
	BtnBioscanRestart       = "🔄 Начать заново"
	// BtnOpenHealthSummary - inline-кнопка, которая открывает Mini App
	// «Мой профиль» прямо из чата (появляется после выдачи отчёта, чтобы
	// пользователь мог сразу открыть сохранённый результат).
	BtnOpenHealthSummary = "📊 Открыть Мой профиль"

	// BtnProfileUse / BtnProfileChange - inline-кнопки экрана «Данные уже
	// известны?» перед запуском опросника. «Использовать» подставляет
	// сохранённый профиль (пропускает вопросы имя/возраст/пол/рост/вес),
	// «Изменить» запускает опросник заново.
	BtnProfileUse    = "✅ Использовать"
	BtnProfileChange = "✏️ Изменить"
)

// UserAgreementText - текст пользовательского соглашения (версия 2.0).
// Определён в internal/locales/agreement.go.

// Lowercase button texts (for case-insensitive matching)
const (
	BtnProcessAnalysisLower      = "✅ обработать анализы"
	BtnProcessAnalysisLowerShort = "обработать анализы"
	BtnCancelLower               = "❌ отмена"
	BtnCancelLowerShort          = "отмена"
)

// Misc messages not yet in other files
const (
	MsgAnalysisComplete      = "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:"
	MsgBioscanIntro          = "📸 Bioscan - комплексный анализ тела\n\nЯ проведу детальный анализ вашей фигуры и дам персональные рекомендации.\n\n📋 Шаг 1 из 10: Введите ваше имя"
	MsgUserDataSummaryHeader = "📋 Ваши данные:"
	// MsgResultSavedSummary - сообщение после выдачи ЛЮБОГО отчёта
	// (обычный/расширенный анализ, базовый/PRO Bioscan). Говорит, что
	// результат сохранён в «Мой профиль», оформлен там аккуратно и его
	// можно открыть прямо сейчас (кнопка ниже) либо позже через меню
	// Здоровье → Мой профиль. Короткое, без форматирования.
	MsgResultSavedSummary = "✅ Результат сохранён в «Моём профиле». Там он оформлен аккуратно и его легко найти позже. Открыть можно прямо сейчас по кнопке ниже, либо в меню: Здоровье → Мой профиль."
	// MsgDemoResultSavedSummary - пояснение в ДЕМО-режиме (предпросмотр
	// результатов «как заполнено»). Данные здесь НЕ сохраняются, поэтому
	// текст говорит именно об этом, но показывает, куда в обычном режиме
	// попадёт результат и даёт кнопку открыть демо-Мой профиль (чтобы
	// увидеть, как там оформлены сохранённые отчёты).
	MsgDemoResultSavedSummary = "🧪 Это демо-режим: результаты здесь не сохраняются. Но в обычном режиме каждый анализ и Bioscan попадают в «Мой профиль» - оформленные аккуратно и доступные позже через меню Здоровье → Мой профиль. Можно посмотреть, как это выглядит:"
)

// Bioscan animation statuses
const (
	BioscanStatusAnalyzingProportions    = "🔍 Анализирую пропорции тела..."
	BioscanStatusCheckingMuscleBalance   = "💪 Проверяю мышечный баланс..."
	BioscanStatusAnalyzingPosture        = "🦴 Анализирую осанку..."
	BioscanStatusEvaluatingComposition   = "📊 Оцениваю композицию тела..."
	BioscanStatusFormingProfile          = "🧬 Формирую профиль развития..."
	BioscanStatusCreatingRecommendations = "📝 Создаю рекомендации..."
)

// BtnBioscanBasicMale / BtnBioscanBasicFemale - inline-кнопки выбора пола
// в мини-опроснике базового (бесплатного) Bioscan.
const (
	BtnBioscanBasicMale   = "👨 Мужской"
	BtnBioscanBasicFemale = "👩 Женский"
)

// BtnBioscanBasicGoal* - inline-кнопки выбора цели в мини-опроснике базового
// (бесплатного) Bioscan.
const (
	BtnBioscanBasicGoalMass   = "💪 Набор массы"
	BtnBioscanBasicGoalCut    = "🔥 Снижение веса"
	BtnBioscanBasicGoalKeep   = "⚖️ Поддержание формы"
	BtnBioscanBasicGoalEndure = "🏃 Выносливость"
	BtnBioscanBasicGoalFlex   = "🧘 Гибкость"
)

// BtnBioscanProGoal* - inline-кнопки выбора цели в опроснике Bioscan PRO.
const (
	BtnBioscanProGoalMass = "💪 Набор массы"
	BtnBioscanProGoalCut  = "🔥 Снижение веса"
	BtnBioscanProGoalKeep = "⚖️ Поддержание формы"
)

// BtnBioscanProLevel* - inline-кнопки выбора уровня тренированности в
// опроснике Bioscan PRO.
const (
	BtnBioscanProLevelNovice  = "🌱 Новичок"
	BtnBioscanProLevelAmateur = "💪 Любитель"
	BtnBioscanProLevelPro     = "🔥 Профи"
)

// BtnHealthGender* - inline-кнопки выбора пола в опроснике «Общая оценка
// здоровья».
const (
	BtnHealthGenderMale   = "👨 Мужской"
	BtnHealthGenderFemale = "👩 Женский"
)

// BtnHabits* - inline-кнопки выбора вредных привычек в опроснике
// «Общая оценка здоровья».
const (
	BtnHabitsNone    = "✅ Нет"
	BtnHabitsSmoke   = "🚬 Курю"
	BtnHabitsAlcohol = "🍺 Алкоголь"
	BtnHabitsBoth    = "🚬🍺 И то и другое"
)
