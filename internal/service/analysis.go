package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/orchestrator"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/report"
	reportmodels "github.com/theamornoir/analyzpro/internal/report/models"
)

// AnalysisService - интерфейс сервиса анализа
type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	HandleBioscan(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	// Методы для работы с JSON
	HandleAnalysisJSON(ctx context.Context, text string) (string, error)
	HandleAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
}

// analysisService - реализация AnalysisService
type analysisService struct {
	aiClient *orchestrator.Orchestrator
	renderer *report.Renderer
}

// NewAnalysisService создает новый AnalysisService
func NewAnalysisService(
	aiClient *orchestrator.Orchestrator,
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

// HandleBioscan — обрабатывает фото для Bioscan и возвращает HTML-отчёт
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

// formatTextWithContext объединяет базовый текст с дополнительным контекстом
func (s *analysisService) formatTextWithContext(baseText, contextInfo string) string {
	contextInfo = strings.TrimSpace(contextInfo)
	if contextInfo == "" {
		return baseText
	}
	return fmt.Sprintf("%s\n\n❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:\n%s", baseText, contextInfo)
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

// HandleAnalysisJSON — возвращает JSON-анализ текста
func (s *analysisService) HandleAnalysisJSON(
	ctx context.Context,
	text string,
) (string, error) {
	return s.aiClient.GenerateAnalysisJSON(ctx, text)
}

// HandleAnalysisFromFileJSON — возвращает JSON-анализ файла
func (s *analysisService) HandleAnalysisFromFileJSON(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	textPrompt := s.formatTextWithContext(locales.MsgDocumentContent, contextInfo)
	return s.aiClient.GenerateAnalysisFromFileJSON(ctx, data, mimeType, textPrompt)
}

// HandleAdaptiveAnalysis — возвращает адаптивный HTML-отчёт по тексту
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

// HandleAdaptiveAnalysisWithContext — адаптивный отчёт с контекстом
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

// renderAdaptiveFromJSON — парсит JSON от AI и рендерит адаптивный HTML
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
