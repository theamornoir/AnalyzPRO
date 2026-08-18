package locales

// ============================================================================
// Extracted from internal/bot/handlers/bioscan/*.go (non-test).
//
// Centralised user-facing / log / AI-prompt string literals that were
// previously hard-coded inside the bioscan handlers. Several literals already
// existed in the locales package and are REUSED here (not redefined):
//   - MsgUserSummaryName / Age / Height / Weight / Goal  (bioscan questionnaire summary rows)
//   - RptMsgPdfRecovery / RptMsgPdfNutrition              (questionnaire label values)
// This file only defines the NEW constants. Do not edit existing locales
// files to add these; everything new lives here.
// ============================================================================

// ---------------------------------------------------------------------------
// AI-prompt context (internal/bot/handlers/bioscan/bioscan_context.go).
// Header + per-question label values used when assembling the Bioscan PRO
// context block that is sent to the AI together with the photos.
// ---------------------------------------------------------------------------
const PromptBioscanContextHeader = "Данные пользователя (опросник Bioscan PRO):"

const (
	PromptBioscanQLabelTrainingExp      = "Стаж тренировок"
	PromptBioscanQLabelTrainingFreq     = "Частота тренировок"
	PromptBioscanQLabelTrainingType     = "Виды тренировок"
	PromptBioscanQLabelInjuries         = "Травмы и боли"
	PromptBioscanQLabelPostureIssues    = "Проблемы с осанкой"
	PromptBioscanQLabelImproveZones     = "Зоны для проработки"
	PromptBioscanQLabelMobility         = "Гибкость и мобильность"
	PromptBioscanQLabelSleep            = "Сон"
	PromptBioscanQLabelStress           = "Уровень стресса"
	PromptBioscanQLabelProtein          = "Белок"
	PromptBioscanQLabelWater            = "Питьевой режим"
	PromptBioscanQLabelSmoking          = "Курение"
	PromptBioscanQLabelAlcohol          = "Алкоголь"
	PromptBioscanQLabelSedentary        = "Сидячий образ жизни"
	PromptBioscanQLabelBodyFatGoal      = "Цель по композиции"
	PromptBioscanQLabelDietRestrictions = "Ограничения в питании"
)

// ---------------------------------------------------------------------------
// Logs (internal/bot/handlers/bioscan/bioscan_process.go) - conversion,
// monitoring and storage outcomes during Bioscan PRO processing.
// ---------------------------------------------------------------------------
const (
	LogBioscanPdfConvertFailed     = "⚠️ [BIOSCAN] не удалось конвертировать PRO-отчёт в PDF (chatID=%d): %v - отправляю HTML"
	LogBioscanMonitoringSaveFailed = "[MONITORING] не удалось сохранить биоскан chatID=%d: %v"
	LogBioscanMonitoringSaved      = "[MONITORING] история сохранена chatID=%d type=bioscan"
	LogBioscanStorageSaveFailed    = "[STORAGE] не удалось сохранить диагноз-биоскан chatID=%d: %v"
	LogBioscanStorageSaved         = "[STORAGE] диагноз сохранён chatID=%d type=bioscan"
)
