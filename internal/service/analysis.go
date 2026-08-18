package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/report"
	reportmodels "github.com/theamornoir/analyzpro/internal/report/models"
)

// AIClient - интерфейс AI-клиента. Реализуется единым мультимодальным
// Claude-клиентом (internal/ai/claude). Выделен как интерфейс, чтобы сервис
// не зависел от конкретного провайдера и оставался тестируемым.
type AIClient interface {
	GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error)
	GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error)
	GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error)
	// GenerateAnalysisFromFilesWithContext - анализ НЕСКОЛЬКИХ файлов
	// (изображения/PDF) одним мультимодальным запросом. Все вложения
	// передаются в ОДНО сообщение вместе с промптом, как в Bioscan PRO, -
	// это гарантирует, что ни один файл/показатель не теряется и Claude видит
	// связь между несколькими анализами сразу.
	GenerateAnalysisFromFilesWithContext(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error)
	// GenerateAnalysisFromFilesJSON - структурированный JSON-анализ НЕСКОЛЬКИХ
	// файлов одним мультимодальным запросом (показатели для дашборда).
	GenerateAnalysisFromFilesJSON(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error)
	GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error)
	GenerateDossierJSON(ctx context.Context, userInput string) (string, error)
}

// AnalysisService - интерфейс сервиса анализа
type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	HandleBioscan(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	// HandleBioscanText - базовый (бесплатный) Bioscan: 1 фото -> plain-text
	// отчёт (без markdown) для вывода обычным сообщением в чат.
	HandleBioscanText(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	// HandleBioscanPDF - расширенный (Premium) Bioscan: 4 фото -> PDF-отчёт
	// (для пользователя) + HTML (для сохранения в историю/профиль). Один
	// вызов ИИ: JSON -> models.Report -> PDF + HTML.
	HandleBioscanPDF(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (pdf []byte, filename string, htmlReport string, err error)
	// HandleBioscanPro - расширенный (Premium) Bioscan PRO: 4 фото -> подробный
	// premium HTML-отчёт Body Intelligence (из фото + опросника). Один вызов
	// ИИ: JSON -> models.BodyScanReport -> HTML (для документа и истории/профиля).
	// Возвращает также jsonReport - «чистый» JSON отчёта, который сохраняется
	// в историю (для графиков дашборда «Мой профиль») и используется при
	// сравнительном повторном анализе.
	HandleBioscanPro(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (html string, jsonReport string, err error)
	// Методы для работы с JSON
	HandleAnalysisJSON(ctx context.Context, text string) (string, error)
	HandleAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	// HandleAnalysisFromFilesWithContext - обрабатывает НЕСКОЛЬКО файлов
	// единым мультимодальным запросом (все вложения в одном сообщении).
	HandleAnalysisFromFilesWithContext(ctx context.Context, data [][]byte, mimeTypes []string, contextInfo string) (string, error)
	// HandleAnalysisFromFilesJSON - структурированный JSON-анализ нескольких
	// файлов одним запросом (для блоков дашборда «Мой профиль»).
	HandleAnalysisFromFilesJSON(ctx context.Context, data [][]byte, mimeTypes []string, contextInfo string) (string, error)
	// HandleExtendedDossierJSON - строит JSON универсального отчёта-досье
	// здоровья (анализы пользователя + 20-вопросный опросник) и возвращает
	// сырой JSON для последующего рендера в HTML (report.Renderer.RenderDossier).
	HandleExtendedDossierJSON(ctx context.Context, combinedText string) (string, error)
}

// analysisService - реализация AnalysisService
type analysisService struct {
	aiClient AIClient
	renderer *report.Renderer
}

// NewAnalysisService создает новый AnalysisService
func NewAnalysisService(
	aiClient AIClient,
	renderer *report.Renderer,
) AnalysisService {
	return &analysisService{
		aiClient: aiClient,
		renderer: renderer,
	}
}

// HandleAnalysis - обрабатывает текст и возвращает текстовый отчёт
func (s *analysisService) HandleAnalysis(
	ctx context.Context,
	text string,
) (string, error) {
	return s.aiClient.GenerateAnalysisSummary(ctx, text)
}

// HandleAnalysisWithContext - обрабатывает текст с контекстом
func (s *analysisService) HandleAnalysisWithContext(
	ctx context.Context,
	text string,
	contextInfo string,
) (string, error) {
	fullText := s.formatTextWithContext(text, contextInfo)
	return s.aiClient.GenerateAnalysisSummary(ctx, fullText)
}

// HandleAnalysisFromFileWithContext - обрабатывает файл с контекстом
func (s *analysisService) HandleAnalysisFromFileWithContext(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	textPrompt := s.formatTextWithContext(locales.MsgDocumentContent, contextInfo)
	return s.aiClient.GenerateAnalysisFromFileWithContext(ctx, data, mimeType, textPrompt)
}

// HandleAnalysisFromFilesWithContext - обрабатывает НЕСКОЛЬКО файлов единым
// мультимодальным запросом. Все вложения уходят в одно сообщение к ИИ
// (как в Bioscan PRO), что исключает потерю данных между файлами.
func (s *analysisService) HandleAnalysisFromFilesWithContext(
	ctx context.Context,
	data [][]byte,
	mimeTypes []string,
	contextInfo string,
) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("no files provided for analysis")
	}
	textPrompt := s.formatTextWithContext(locales.MsgDocumentContent, contextInfo)
	return s.aiClient.GenerateAnalysisFromFilesWithContext(ctx, data, mimeTypes, textPrompt)
}

// HandleAnalysisFromFilesJSON - структурированный JSON-анализ нескольких
// файлов одним запросом (показатели для блоков «Мой профиль»).
func (s *analysisService) HandleAnalysisFromFilesJSON(
	ctx context.Context,
	data [][]byte,
	mimeTypes []string,
	contextInfo string,
) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("no files provided for analysis")
	}
	textPrompt := s.formatTextWithContext(locales.MsgDocumentContent, contextInfo)
	return s.aiClient.GenerateAnalysisFromFilesJSON(ctx, data, mimeTypes, textPrompt)
}

// HandleBioscan - обрабатывает фото для Bioscan и возвращает HTML-отчёт
func (s *analysisService) HandleBioscan(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	jsonText, err := s.aiClient.GenerateBioscanJSON(ctx, photosData, mimeType, contextInfo)
	if err != nil {
		return "", fmt.Errorf(locales.ErrGenerateBioscanJSON, err)
	}

	return s.renderReportFromJSON(jsonText, true)
}

// bioscanReportFromJSON анмаршалит JSON от ИИ в models.Report, проставляя
// флаг IsBioscan и рассчитывая углы диаграмм.
func (s *analysisService) bioscanReportFromJSON(jsonText string) (models.Report, error) {
	if strings.TrimSpace(jsonText) == "" {
		return models.Report{}, fmt.Errorf(locales.ErrEmptyJSONFromAI)
	}

	var rep models.Report
	if err := json.Unmarshal([]byte(jsonText), &rep); err != nil {
		return models.Report{}, fmt.Errorf(locales.ErrParseAnalysisJSON, err)
	}

	rep.IsBioscan = true
	// Расчёт углов для круговых диаграмм Bioscan
	rep.Profile.CompositionAngle = rep.Profile.Composition * 360 / 100
	rep.Profile.MuscleAngle = rep.Profile.MuscleDevelopment * 360 / 100
	rep.Profile.BalanceAngle = rep.Profile.Balance * 360 / 100
	rep.Profile.PotentialAngle = rep.Profile.Potential * 360 / 100

	return rep, nil
}

// HandleBioscanText - базовый (бесплатный) Bioscan: 1 фото -> plain-text
// отчёт без markdown (для вывода обычным сообщением в чат).
func (s *analysisService) HandleBioscanText(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	jsonText, err := s.aiClient.GenerateBioscanJSON(ctx, photosData, mimeType, contextInfo)
	if err != nil {
		return "", fmt.Errorf(locales.ErrGenerateBioscanJSON, err)
	}

	rep, err := s.bioscanReportFromJSON(jsonText)
	if err != nil {
		return "", err
	}

	return report.RenderBioscanPlainText(rep), nil
}

// HandleBioscanPDF - расширенный (Premium) Bioscan: 4 фото -> PDF-отчёт
// (для пользователя) + HTML (для сохранения в историю/профиль). Один вызов ИИ.
func (s *analysisService) HandleBioscanPDF(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (pdf []byte, filename string, htmlReport string, err error) {
	jsonText, err := s.aiClient.GenerateBioscanJSON(ctx, photosData, mimeType, contextInfo)
	if err != nil {
		return nil, "", "", fmt.Errorf(locales.ErrGenerateBioscanJSON, err)
	}

	rep, jerr := s.bioscanReportFromJSON(jsonText)
	if jerr != nil {
		return nil, "", "", jerr
	}

	htmlReport, rerr := s.renderer.Render(rep)
	if rerr != nil {
		return nil, "", "", fmt.Errorf(locales.ErrRenderReportHTML, rerr)
	}

	pdfBytes, perr := report.RenderBioscanPDF(rep)
	if perr != nil {
		return nil, "", "", fmt.Errorf(locales.ErrRenderReportPDF, perr)
	}

	return pdfBytes, "Bioscan_report.pdf", htmlReport, nil
}

// HandleBioscanPro - расширенный (Premium) Bioscan PRO: строит подробный
// premium HTML-отчёт Body Intelligence из 4 фото + опросника. Один вызов ИИ:
// JSON -> models.BodyScanReport -> HTML (через renderer.RenderBodyScan).
// Возвращает также «чистый» JSON-отчёт (jsonReport) - он сохраняется в
// историю для графиков дашборда и используется при сравнительном анализе.
func (s *analysisService) HandleBioscanPro(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (string, string, error) {
	jsonText, err := s.aiClient.GenerateBodyScanJSON(ctx, photosData, mimeType, contextInfo)
	if err != nil {
		return "", "", fmt.Errorf(locales.ErrGenerateBioscanJSON, err)
	}

	cleaned := strings.TrimSpace(jsonText)
	// Снимаем возможную markdown-обёртку ```json ... ```.
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "", "", fmt.Errorf(locales.ErrEmptyJSONFromAI)
	}

	var rep models.BodyScanReport
	if err := json.Unmarshal([]byte(cleaned), &rep); err != nil {
		return "", "", fmt.Errorf(locales.ErrParseAnalysisJSON, err)
	}

	// Разрыв между текущим и потенциальным индексом.
	rep.Gap = rep.Potential - rep.Score
	if rep.Gap < 0 {
		rep.Gap = 0
	}

	htmlReport, rerr := s.renderer.RenderBodyScan(rep)
	if rerr != nil {
		return "", "", fmt.Errorf(locales.ErrRenderReportHTML, rerr)
	}

	// Сериализуем обратно в «чистый» JSON (без markdown-обёртки), чтобы
	// сохранять в историю именно валидный JSON-отчёт.
	jsonReport, merr := json.Marshal(rep)
	if merr != nil {
		// Не критично: отдаём хотя бы очищенный исходник от ИИ.
		jsonReport = []byte(cleaned)
	}

	return htmlReport, string(jsonReport), nil
}

// formatTextWithContext объединяет базовый текст с дополнительным контекстом
func (s *analysisService) formatTextWithContext(baseText, contextInfo string) string {
	contextInfo = strings.TrimSpace(contextInfo)
	if contextInfo == "" {
		return baseText
	}
	return fmt.Sprintf("%s%s%s", baseText, locales.PromptContextHeader, contextInfo)
}

// renderReportFromJSON анмаршалит JSON в структуры report.Report и рендерит HTML
func (s *analysisService) renderReportFromJSON(jsonText string, isBioscan bool) (string, error) {
	if strings.TrimSpace(jsonText) == "" {
		return "", fmt.Errorf(locales.ErrEmptyJSONFromAI)
	}

	var rep models.Report
	if err := json.Unmarshal([]byte(jsonText), &rep); err != nil {
		return "", fmt.Errorf(locales.ErrParseAnalysisJSON, err)
	}

	rep.IsBioscan = isBioscan

	if isBioscan {
		// Расчёт углов для круговых диаграмм Bioscan
		rep.Profile.CompositionAngle = rep.Profile.Composition * 360 / 100
		rep.Profile.MuscleAngle = rep.Profile.MuscleDevelopment * 360 / 100
		rep.Profile.BalanceAngle = rep.Profile.Balance * 360 / 100
		rep.Profile.PotentialAngle = rep.Profile.Potential * 360 / 100
	}

	html, err := s.renderer.Render(rep)
	if err != nil {
		return "", fmt.Errorf(locales.ErrRenderReportHTML, err)
	}

	return html, nil
}

// HandleAnalysisJSON - возвращает JSON-анализ текста
func (s *analysisService) HandleAnalysisJSON(
	ctx context.Context,
	text string,
) (string, error) {
	return s.aiClient.GenerateAnalysisJSON(ctx, text)
}

// HandleAnalysisFromFileJSON - возвращает JSON-анализ файла
func (s *analysisService) HandleAnalysisFromFileJSON(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	textPrompt := s.formatTextWithContext(locales.MsgDocumentContent, contextInfo)
	return s.aiClient.GenerateAnalysisFromFileJSON(ctx, data, mimeType, textPrompt)
}

// HandleExtendedDossierJSON - строит универсальное отчёт-досье здоровья.
// combinedText содержит текст присланных анализов (файлы→текст) и ответы
// 20-вопросного опросника об образе жизни. Возвращает сырой JSON-досье.
func (s *analysisService) HandleExtendedDossierJSON(
	ctx context.Context,
	combinedText string,
) (string, error) {
	return s.aiClient.GenerateDossierJSON(ctx, combinedText)
}

// HandleAdaptiveAnalysis - возвращает адаптивный HTML-отчёт по тексту
func (s *analysisService) HandleAdaptiveAnalysis(
	ctx context.Context,
	text string,
) (string, error) {
	jsonText, err := s.aiClient.GenerateAnalysisJSON(ctx, text)
	if err != nil {
		return "", err
	}

	return s.renderAdaptiveFromJSON(jsonText)
}

// HandleAdaptiveAnalysisWithContext - адаптивный отчёт с контекстом
func (s *analysisService) HandleAdaptiveAnalysisWithContext(
	ctx context.Context,
	text string,
	contextInfo string,
) (string, error) {
	fullText := s.formatTextWithContext(text, contextInfo)
	jsonText, err := s.aiClient.GenerateAnalysisJSON(ctx, fullText)
	if err != nil {
		return "", err
	}

	return s.renderAdaptiveFromJSON(jsonText)
}

// renderAdaptiveFromJSON - парсит JSON от AI и рендерит адаптивный HTML
func (s *analysisService) renderAdaptiveFromJSON(jsonText string) (string, error) {
	if strings.TrimSpace(jsonText) == "" {
		return "", fmt.Errorf(locales.ErrEmptyJSONFromAI)
	}

	// Очищаем JSON от markdown-обёртки
	cleaned := strings.TrimSpace(jsonText)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var data reportmodels.AdaptiveReportData
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		return "", fmt.Errorf(locales.ErrParseAnalysisJSON, err)
	}

	html := report.RenderAdaptiveReport(data)
	return html, nil
}
