package locales

// Keyboard button texts
const (
	BtnProcessAnalysis = "✅ Обработать анализы"
	BtnBack            = "⬅️ Назад"
	BtnCancel          = "❌ Отмена"
	BtnAgreement       = "📝 Пользовательское соглашение"
	BtnAbout           = "ℹ️ О сервисе"
	// BtnAnalysisHub — верхняя кнопка-хаб «Анализы»: объединяет всё, что
	// связано с расшифровкой (лаб. анализы + фотографический Bioscan).
	BtnAnalysisHub = "📋 Анализы"
	BtnDiagnostics = "🏥 Диагностика анализов"
	BtnBioscan     = "📸 Bioscan"
	// Базовый (бесплатный) Bioscan: 1 фото -> текстовый результат в чат.
	BtnBioscanBasic = "🌐 Bioscan"
	// Расширенный (Premium) Bioscan PRO: 4 фото -> детальный PDF-отчёт.
	BtnBioscanExtended = "✨ Bioscan PRO"
	// BtnHealthHub — верхняя кнопка-хаб «Здоровье»: отслеживание состояния
	// (Сводка / Мониторинг) + консультация с ИИ.
	BtnHealthHub = "📊 Здоровье"
	// BtnServiceHub — верхняя кнопка-хаб «Сервис»: связь с разработчиком
	// (Отзывы) и информация о боте (О сервисе).
	BtnServiceHub = "⚙️ Сервис"
	// BtnHealthSummary — переименованный «Дашборд»: слово «дашборд»
	// пользователям непонятно, поэтому теперь это «Сводка здоровья» —
	// снимок состояния здоровья прямо сейчас.
	BtnHealthSummary = "💡 Сводка здоровья"
	BtnMonitoring    = "📊 Мониторинг"
	// 🧪 Демо-кнопки — открывают веб-аппы с ?demo=1, чтобы посмотреть
	// графики «как заполнено», без реальных анализов и без Premium.
	BtnHealthSummaryDemo = "🧪 Демо-Сводка"
	BtnMonitoringDemo    = "🧪 Демо-Мониторинг"
	// BtnHealthDynamics — новый раздел-хаб, объединяющий «Сводка здоровья»
	// и «Мониторинг» (отслеживание показателей в динамике).
	BtnHealthDynamics = "📈 Здоровье в динамике"
	BtnFeedback       = "📝 Отзывы и предложения"
	// BtnConsultation — новый раздел «Быстрая консультация (с ИИ)».
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

Я, Prisma, помогаю разобраться в медицинских анализах.

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
