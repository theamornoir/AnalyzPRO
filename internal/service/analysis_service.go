package service

import (
	"context"

	"github.com/theamornoir/analyzpro/internal/ai"
)

type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error)
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

func (s *analysisService) HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error) {
	return s.aiClient.GenerateAnalysisFromFile(ctx, data, mimeType)
}
