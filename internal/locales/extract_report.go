package locales

// ============================================================================
// Report rendering strings (internal/report/*.go).
//
// Centralised user-facing / error string literals that were previously
// hard-coded inside the report renderer (adaptive HTML, PDF, plain-text and
// the renderer helpers). Kept here so the report output can be localised.
// ============================================================================

// ---------------------------------------------------------------------------
// PDF report (RenderBioscanPDF) - section titles and field labels.
// ---------------------------------------------------------------------------
const (
	RptMsgPdfHeader            = "BIOSCAN - отчёт о телосложении"
	RptMsgPdfSubHeader         = "Персональный фотографический анализ тела"
	RptMsgPdfOverallAssessment = "Общая оценка"
	RptMsgPdfScoreLabel        = "Балл: "
	RptMsgPdfBodyParams        = "Параметры тела"
	RptMsgPdfHeightLabel       = "Рост: "
	RptMsgPdfWeightLabel       = "Вес: "
	RptMsgPdfMuscleMassLabel   = "Мышечная масса: "
	RptMsgPdfBodyFatLabel      = "Процент жира: "
	RptMsgPdfComposition       = "Композиция тела"
	RptMsgPdfProfileDev        = "Профиль развития"
	RptMsgPdfCompositionLabel  = "Композиция"
	RptMsgPdfMuscleDevLabel    = "Развитие мышц"
	RptMsgPdfBalanceLabel      = "Баланс"
	RptMsgPdfPotentialLabel    = "Потенциал"
	RptMsgPdfSummary           = "Резюме"
	RptMsgPdfZoneAssessment    = "Оценка зон тела"
	RptMsgPdfStatusLabel       = "Статус: "
	RptMsgPdfRecLabel          = "Рекомендация: "
	RptMsgPdfAssessmentLabel   = "Оценка: "
	RptMsgPdfSymmetryLabel     = "Симметрия: "
	RptMsgPdfMuscleGroups      = "Мышечные группы"
	RptMsgPdfPosture           = "Осанка"
	RptMsgPdfTypeLabel         = "Тип: "
	RptMsgPdfHeadLabel         = "Голова: "
	RptMsgPdfShouldersLabel    = "Плечи: "
	RptMsgPdfPelvisLabel       = "Таз: "
	RptMsgPdfAttentionZones    = "Зоны внимания"
	RptMsgPdfProblemLabel      = "Проблема: "
	RptMsgPdfSolutionLabel     = "Решение: "
	RptMsgPdfPriorities        = "Приоритеты развития"
	RptMsgPdfTrainingProgram   = "Программа тренировок"
	RptMsgPdfNutrition         = "Питание"
	RptMsgPdfRecovery          = "Восстановление"
	RptMsgPdfProgressControl   = "Контроль прогресса"
	RptMsgPdfRecheckLabel      = "Повторная проверка: "
	RptMsgPdfDisclaimer        = "Отчёт сформирован автоматически на основе фотографий и носит " +
		"информационный характер. Он не является медицинским диагнозом и не " +
		"заменяет консультацию специалиста."
)

// ---------------------------------------------------------------------------
// Adaptive HTML report (RenderAdaptiveReport) - status labels and errors.
// ---------------------------------------------------------------------------
const (
	RptMsgAdaptiveStatusNormal   = "Норма"
	RptMsgAdaptiveStatusWarning  = "Внимание"
	RptMsgAdaptiveStatusCritical = "Критично"
	// RptErrAdaptiveRender - ошибка рендера адаптивного HTML-отчёта.
	RptErrAdaptiveRender = "<h1>Ошибка рендера: %v</h1>"
)

// ---------------------------------------------------------------------------
// Renderer (NewRenderer) - template parse errors.
// ---------------------------------------------------------------------------
const (
	RptErrParseBioscanTemplate  = "parse bioscan template: %w"
	RptErrParseDossierTemplate  = "parse health dossier template: %w"
	RptErrParseBodyScanTemplate = "parse body scan report template: %w"
)

// ---------------------------------------------------------------------------
// Renderer helpers - human-readable status labels (used by templates).
// ---------------------------------------------------------------------------
const (
	// Body Intelligence (Bioscan PRO) status labels.
	RptMsgBodyStatusNormal   = "в норме"
	RptMsgBodyStatusWarning  = "внимание"
	RptMsgBodyStatusCritical = "риск"

	// Generic status labels used by the analysis template.
	RptMsgStatusLabelNormal   = "норма"
	RptMsgStatusLabelWarning  = "внимание"
	RptMsgStatusLabelCritical = "риск"
)

// ---------------------------------------------------------------------------
// Renderer helpers - posture/balance radar chart axis labels.
// ---------------------------------------------------------------------------
const (
	RptMsgRadarAxisSymmetry  = "Симметрия"
	RptMsgRadarAxisShoulders = "Плечи"
	RptMsgRadarAxisPelvis    = "Таз"
	RptMsgRadarAxisSpine     = "Позвоночник"
	RptMsgRadarAxisMobility  = "Мобильность"
	RptMsgRadarAxisStability = "Стабильность"
)

// ---------------------------------------------------------------------------
// Plain-text report (RenderBioscanPlainText) - section titles and labels.
// Формат вывода в чат приведён к шаблону пользователя: emoji-маркеры
// (💪/📏/⚖️/🔥/●/🔸/⚡/⚠️/✦), без markdown-разметки и звёздочек.
// ---------------------------------------------------------------------------
const (
	RptMsgTextHeader            = "💪 BIOSCAN — результат анализа тела\n"
	RptMsgTextOverallAssessment = "Общая оценка: "
	RptMsgTextHeight            = "📏 "
	RptMsgTextWeight            = "⚖️ "
	RptMsgTextMuscle            = "💪 "
	RptMsgTextFat               = "🔥 "
	RptMsgTextComposition       = "● КОМПОЗИЦИЯ ТЕЛА\n"
	RptMsgTextScoreComp         = "Композиция "
	RptMsgTextScoreMuscle       = "Мышцы "
	RptMsgTextScoreBalance      = "Баланс "
	RptMsgTextScorePotential    = "Потенциал "
	RptMsgTextZoneAssessment    = "● ОЦЕНКА ПО ЗОНАМ\n"
	RptMsgTextZoneBullet        = "🔸 "
	RptMsgTextZoneRec           = "Совет: "
	RptMsgTextPosture           = "● ОСАНКА\n"
	RptMsgTextAttentionZones    = "● ЗОНЫ ВНИМАНИЯ\n"
	RptMsgTextAttentionPrefix   = "⚠️ "
	RptMsgTextRecommendations   = "● ГЛАВНЫЕ РЕКОМЕНДАЦИИ\n"
	RptMsgTextRecBullet         = "✦ "
	RptMsgTextProgressControl   = "● КОНТРОЛЬ ПРОГРЕССА\n"
	RptMsgTextDisclaimer        = "⚠️ Отчёт носит информационный характер и не заменяет консультацию врача."

	// Сохранён для обратной совместимости, в новом формате не используется.
	RptMsgTextProfileDev = "Профиль развития:\n"

	// Резюме (краткий вывод ИИ по биоскану), выводится блоком ● РЕЗЮМЕ
	// сразу после оценок композиции в чат-версии отчёта.
	RptMsgTextSummary = "● РЕЗЮМЕ\n"
)
