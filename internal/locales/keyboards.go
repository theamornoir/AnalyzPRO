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
	// 🧪 Демо-кнопки - открывают веб-аппы с ?demo=1, чтобы посмотреть
	// графики «как заполнено», без реальных анализов и без Premium.
	BtnHealthSummaryDemo = "🧪 Демо: Мой профиль"
	// BtnHealthDynamics - новый раздел-хаб, объединяющий «Мой профиль»
	// и «Мониторинг» (отслеживание показателей в динамике).
	BtnHealthDynamics = "📈 Здоровье в динамике"
	BtnFeedback       = "📝 Отзывы и предложения"
	// BtnTestNotify - кнопка входа в тестовое меню проверки уведомлений
	// (раздел «Сервис»). Позволяет разработчику отправить себе образец
	// реального уведомления (напоминание/мотивация/фича) спустя 30 секунд.
	BtnTestNotify     = "🧪 Тест уведомлений"
	BtnTestReminder   = "🔔 Тест напоминание (30 сек)"
	BtnTestMotivation = "💪 Тест мотивация (30 сек)"
	BtnTestFeature    = "📣 Тест фича (30 сек)"
	// BtnConsultation - новый раздел «Быстрая консультация (с ИИ)».
	// Пользователю непонятно «консультация с ИИ», поэтому названо как
	// «Быстрая консультация»: можно прислать фото травмы или задать вопрос,
	// и ИИ даст консультацию с рекомендациями. Premium-функция (3 бесплатно).
	BtnConsultation     = "💬 Быстрая консультация"
	BtnPremium          = "💎 Premium"
	BtnPremiumChange    = "🔄 Сменить тариф"
	BtnRegularAnalysis  = "📊 Обычный анализ"
	BtnExtendedAnalysis = "🔬 Расширенный анализ"
	// 🧪 Демо-кнопки для предпросмотра результатов «как заполнено»,
	// без реальных файлов и без вызовов ИИ. Работают всем (и без Premium).
	BtnRegularAnalysisDemo  = "🧪 Демо: Обычный"
	BtnExtendedAnalysisDemo = "🧪 Демо: Расширенный"
	BtnBioscanBasicDemo     = "🧪 Демо: Bioscan"
	BtnBioscanExtendedDemo  = "🧪 Демо: Bioscan PRO"
	BtnAcceptAgreement      = "✅ Принять соглашение"
	BtnBioscanConfirm       = "✅ Подтвердить и проанализировать"
	BtnBioscanRestart       = "🔄 Начать заново"
	// BtnOpenHealthSummary - inline-кнопка, которая открывает Mini App
	// «Мой профиль» прямо из чата (появляется после выдачи отчёта, чтобы
	// пользователь мог сразу открыть сохранённый результат).
	BtnOpenHealthSummary = "📊 Открыть Мой профиль"
)

// UserAgreementText - текст пользовательского соглашения (версия 1.0).
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
