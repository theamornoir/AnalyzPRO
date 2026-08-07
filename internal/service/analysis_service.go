package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai"
	"github.com/theamornoir/analyzpro/internal/report"
)

type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error)
	HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	HandleAnalysisWithHTML(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFileWithHTML(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	HandleBioscan(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	// Методы для работы с JSON
	HandleAnalysisJSON(ctx context.Context, text string) (string, error)
	HandleAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
}

type analysisService struct {
	aiClient *ai.GeminiClient
	renderer *report.Renderer
}

func NewAnalysisService(
	aiClient *ai.GeminiClient,
	renderer *report.Renderer,
) AnalysisService {
	return &analysisService{
		aiClient: aiClient,
		renderer: renderer,
	}
}

func (s *analysisService) HandleAnalysis(
	ctx context.Context,
	text string,
) (string, error) {
	return s.aiClient.GenerateAnalysisSummary(ctx, text)
}

func (s *analysisService) HandleAnalysisWithContext(
	ctx context.Context,
	text string,
	contextInfo string,
) (string, error) {
	fullText := s.formatTextWithContext(text, contextInfo)
	return s.aiClient.GenerateAnalysisSummary(ctx, fullText)
}

func (s *analysisService) HandleAnalysisFromFile(
	ctx context.Context,
	data []byte,
	mimeType string,
) (string, error) {
	return s.aiClient.GenerateAnalysisFromFile(ctx, data, mimeType)
}

func (s *analysisService) HandleAnalysisFromFileWithContext(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	textPrompt := s.formatTextWithContext("Содержимое загруженного документа с медицинскими анализами.", contextInfo)
	return s.aiClient.GenerateAnalysisFromFileWithContext(ctx, data, mimeType, textPrompt)
}

// HandleAnalysisWithHTML — обрабатывает текст и возвращает HTML-отчёт
func (s *analysisService) HandleAnalysisWithHTML(
	ctx context.Context,
	text string,
	contextInfo string,
) (string, error) {
	jsonText, err := s.HandleAnalysisJSON(ctx, text)
	if err != nil {
		return "", fmt.Errorf("generate json for html: %w", err)
	}

	return s.renderReportFromJSON(jsonText, false)
}

// HandleAnalysisFromFileWithHTML — обрабатывает файл и возвращает HTML-отчёт
func (s *analysisService) HandleAnalysisFromFileWithHTML(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	jsonText, err := s.HandleAnalysisFromFileJSON(ctx, data, mimeType, contextInfo)
	if err != nil {
		return "", fmt.Errorf("generate json from file for html: %w", err)
	}

	return s.renderReportFromJSON(jsonText, false)
}

// ============================================================
// МЕТОДЫ ДЛЯ РАБОТЫ С JSON
// ============================================================

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
	textPrompt := s.formatTextWithContext("Содержимое загруженного документа с медицинскими анализами.", contextInfo)
	return s.aiClient.GenerateAnalysisFromFileJSON(ctx, data, mimeType, textPrompt)
}

// HandleBioscan — обрабатывает фото для Bioscan и возвращает HTML-отчёт
func (s *analysisService) HandleBioscan(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {
	jsonText, err := s.aiClient.GenerateBioscanJSON(ctx, data, mimeType, contextInfo)
	if err != nil {
		return "", fmt.Errorf("generate bioscan json: %w", err)
	}

	return s.renderReportFromJSON(jsonText, true)
}

// ============================================================
// ВНУТРЕННИЕ ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ
// ============================================================

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
		return "", fmt.Errorf("received empty JSON from AI model")
	}

	var rep report.Report
	if err := json.Unmarshal([]byte(jsonText), &rep); err != nil {
		return "", fmt.Errorf("parse analysis report JSON: %w", err)
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
		return "", fmt.Errorf("render report html: %w", err)
	}

	return html, nil
}
