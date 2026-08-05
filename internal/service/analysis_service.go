package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/theamornoir/analyzpro/internal/ai"
	"github.com/theamornoir/analyzpro/internal/report"
)

type AnalysisService interface {
	HandleAnalysis(ctx context.Context, text string) (string, error)
	HandleAnalysisWithContext(ctx context.Context, text string, contextInfo string) (string, error)
	HandleAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error)
	HandleAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
	HandleBioscan(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error)
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

	return s.aiClient.GenerateAnalysisSummary(
		ctx,
		text,
	)
}

func (s *analysisService) HandleAnalysisWithContext(
	ctx context.Context,
	text string,
	contextInfo string,
) (string, error) {

	fullText := text

	if contextInfo != "" {
		fullText = fmt.Sprintf(
			"%s\n\nДополнительная информация о пациенте:\n%s",
			text,
			contextInfo,
		)
	}

	return s.aiClient.GenerateAnalysisSummary(
		ctx,
		fullText,
	)
}

func (s *analysisService) HandleAnalysisFromFile(
	ctx context.Context,
	data []byte,
	mimeType string,
) (string, error) {

	return s.aiClient.GenerateAnalysisFromFile(
		ctx,
		data,
		mimeType,
	)
}

func (s *analysisService) HandleAnalysisFromFileWithContext(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	textPrompt := "Содержимое загруженного документа с медицинскими анализами."

	if contextInfo != "" {
		textPrompt = fmt.Sprintf(
			"%s\n\nДополнительная информация о пациенте:\n%s",
			textPrompt,
			contextInfo,
		)
	}

	return s.aiClient.GenerateAnalysisFromFileWithContext(
		ctx,
		data,
		mimeType,
		textPrompt,
	)
}

func (s *analysisService) HandleBioscan(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	jsonText, err := s.aiClient.GenerateBioscanJSON(
		ctx,
		data,
		mimeType,
		contextInfo,
	)

	if err != nil {
		return "", err
	}

	fmt.Println("===================================")
	fmt.Println("JSON LEN:", len(jsonText))
	fmt.Println(jsonText)
	fmt.Println("===================================")

	// Получаем JSON от Gemini
	// jsonText, err := s.aiClient.GenerateBioscanJSON(
	// 	ctx,
	// 	data,
	// 	mimeType,
	// 	contextInfo,
	// )

	if err != nil {
		return "", err
	}

	// JSON -> структура отчета
	var bioscanReport report.Report

	err = json.Unmarshal(
		[]byte(jsonText),
		&bioscanReport,
	)

	if err != nil {
		return "", fmt.Errorf(
			"parse bioscan report: %w",
			err,
		)
	}

	bioscanReport.Profile.CompositionAngle =
		bioscanReport.Profile.Composition * 360 / 100

	bioscanReport.Profile.MuscleAngle =
		bioscanReport.Profile.MuscleDevelopment * 360 / 100

	bioscanReport.Profile.BalanceAngle =
		bioscanReport.Profile.Balance * 360 / 100

	bioscanReport.Profile.PotentialAngle =
		bioscanReport.Profile.Potential * 360 / 100

	// Структура -> HTML
	html, err := s.renderer.Render(
		bioscanReport,
	)

	if err != nil {
		return "", fmt.Errorf(
			"render bioscan html: %w",
			err,
		)
	}

	return html, nil
}
