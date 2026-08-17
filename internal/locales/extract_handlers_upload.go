package locales

// ============================================================================
// Extracted string literals from internal/bot/handlers/upload/*.go.
//
// Centralises the remaining inline user-facing / log / error / AI-prompt
// string literals that were still hard-coded inside the upload handlers.
// Constants reuse the existing Msg*/Log*/Err*/Btn*/Prompt* naming scheme so
// they can be localised together with the rest of the project strings.
// ============================================================================

// ---------------------------------------------------------------------------
// Log strings (internal/bot/handlers/upload/upload_report.go).
// ---------------------------------------------------------------------------
const (
	LogUploadMonitoringSaveErr = "[MONITORING] не удалось сохранить историю chatID=%d: %v"
	LogUploadMonitoringSaved   = "[MONITORING] история сохранена chatID=%d type=analysis"
	LogUploadStorageSaveErr    = "[STORAGE] не удалось сохранить диагноз chatID=%d: %v"
	LogUploadStorageSaved      = "[STORAGE] диагноз сохранён chatID=%d type=analysis"

	LogUploadPdfConvertErr        = "⚠️ [UPLOAD] не удалось конвертировать анализ в PDF (chatID=%d): %v - отправляю HTML"
	LogUploadPdfSent              = "✅ [UPLOAD] PDF-отчёт (расширенный анализ) отправлен chatID=%d: %d байт"
	LogUploadDossierPdfConvertErr = "⚠️ [UPLOAD] не удалось конвертировать досье в PDF (chatID=%d): %v - отправляю HTML"
	LogUploadDossierPdfSent       = "✅ [UPLOAD] PDF-отчёт (досье/Биоскан PRO) отправлен chatID=%d: %d байт"
)

// ---------------------------------------------------------------------------
// Default report titles - fallback for monitoring.ExtractTitle (upload_report.go).
// ---------------------------------------------------------------------------
const (
	MsgUploadDefaultTitleAnalysis = "Анализ"
	MsgUploadDefaultTitleDossier  = "Досье здоровья"
)

// ---------------------------------------------------------------------------
// AI-prompt building strings (internal/bot/handlers/upload/upload_process.go).
// These labels are concatenated into the text sent to the analysis AI.
// ---------------------------------------------------------------------------
const (
	PromptUploadPatientLifestyle = "\n\nДанные пациента и опросника об образе жизни:\n"
	PromptUploadFileSection      = "=== Данные из файла %d (%s) ===\n%s"
)

// ---------------------------------------------------------------------------
// Internal error (internal/bot/handlers/upload/upload_file_store.go).
// ---------------------------------------------------------------------------
const ErrUploadEmptyFilePath = "empty file path"
