package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai"
)

type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
}

type analysisService struct {
	client *ai.Client
}

func NewAnalysisService(client *ai.Client) AnalysisService {
	return &analysisService{client: client}
}

func (s *analysisService) HandleAnalysis(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty analysis input")
	}

	if s.client == nil {
		return ai.BuildDoctorTemplate(
			"Cервис недоступен.",
			"Данные получены, но обработка пока не подключена.",
			"Необходимо настроить сервис.",
			"Подключите сервис и повторите запрос.",
			"Этот ответ не является диагностикой.",
		), nil
	}

	response, err := s.client.GenerateAnalysisSummary(ctx, text)
	if err != nil {
		return ai.BuildDoctorTemplate(
			"Сервис временно недоступен.",
			"Данные получены, но их обработка сейчас ограничена внешним сервисом.",
			"Необходима повторная попытка позже.",
			"Подождите 1–2 минуты и отправьте анализ ещё раз.",
			"Этот ответ не является диагнозом и используется только как безопасная временная заглушка.",
		), nil
	}

	return response, nil
}
