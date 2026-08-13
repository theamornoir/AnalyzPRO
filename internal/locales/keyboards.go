package locales

// Keyboard button texts
const (
	BtnProcessAnalysis  = "✅ Обработать анализы"
	BtnBack             = "⬅️ Назад"
	BtnCancel           = "❌ Отмена"
	BtnAgreement        = "📝 Пользовательское соглашение"
	BtnAbout            = "ℹ️ О сервисе"
	BtnDiagnostics      = "🏥 Диагностика анализов"
	BtnBioscan          = "📸 Bioscan"
	BtnDashboard        = "📊 Мой Дашборд"
	BtnFeedback         = "📝 Отзывы и предложения"
	BtnPremium          = "💎 Premium"
	BtnRegularAnalysis  = "📊 Обычный анализ"
	BtnExtendedAnalysis = "🔬 Расширенный анализ"
	BtnAcceptAgreement  = "✅ Принять соглашение"
	BtnBioscanConfirm   = "✅ Подтвердить и проанализировать"
	BtnBioscanRestart   = "🔄 Начать заново"
)

// Lowercase button texts (for case-insensitive matching)
const (
	BtnProcessAnalysisLower      = "✅ обработать анализы"
	BtnProcessAnalysisLowerShort = "обработать анализы"
	BtnCancelLower               = "❌ отмена"
	BtnCancelLowerShort          = "отмена"
)

// UserAgreementText - текст пользовательского соглашения.
const UserAgreementText = `📝 **ПОЛЬЗОВАТЕЛЬСКОЕ СОГЛАШЕНИЕ**

Я, AnalyzPRO, предоставляю услуги по интерпретации медицинских анализов с использованием искусственного интеллекта.

⚠️ **ВАЖНО:**
1. Бот НЕ ставит диагнозы и НЕ заменяет врача.
2. Результаты носят информационный характер.
3. Всегда консультируйтесь с квалифицированным врачом.
4. Ответственность за использование результатов лежит на пользователе.
5. Ваши данные используются только для анализа и не передаются третьим лицам.

📅 Версия соглашения: 1.0 от 05.08.2026

Нажмите кнопку ниже, чтобы принять соглашение.`

// Misc messages not yet in other files
const (
	MsgAnalysisComplete      = "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:"
	MsgBioscanIntro          = "📸 **Bioscan - комплексный анализ тела**\n\nЯ проведу детальный анализ вашей фигуры и дам персональные рекомендации.\n\n📋 **Шаг 1 из 6: Введите ваше имя**"
	MsgUserDataSummaryHeader = "📋 **Ваши данные:**"
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
