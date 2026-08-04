package service

import (
	"context"
	"fmt"

	"github.com/theamornoir/analyzpro/internal/ai"
)

type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error)
	HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
}

type analysisService struct {
	aiClient *ai.GeminiClient
}

func NewAnalysisService(aiClient *ai.GeminiClient) AnalysisService {
	return &analysisService{
		aiClient: aiClient,
	}
}

func (s *analysisService) HandleAnalysis(ctx context.Context, text string) (string, error) {
	return s.aiClient.GenerateAnalysisSummary(ctx, text)
}

func (s *analysisService) HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error) {
	// Добавляем контекстную информацию к тексту
	fullText := text
	if contextInfo != "" {
		fullText = fmt.Sprintf("%s\n\nДополнительная информация о пациенте:\n%s", text, contextInfo)
	}
	return s.aiClient.GenerateAnalysisSummary(ctx, fullText)
}

func (s *analysisService) HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error) {
	return s.aiClient.GenerateAnalysisFromFile(ctx, data, mimeType)
}

func (s *analysisService) HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error) {
	// Добавляем контекстную информацию
	textPrompt := "Содержимое загруженного документа с медицинскими анализами."
	if contextInfo != "" {
		textPrompt = fmt.Sprintf("%s\n\nДополнительная информация о пациенте:\n%s", textPrompt, contextInfo)
	}
	return s.aiClient.GenerateAnalysisFromFileWithContext(ctx, data, mimeType, textPrompt)
}
