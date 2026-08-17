package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/mock"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// GenerateBioscanJSON - генерирует JSON-результат bioscan по фотографиям.
func (c *GeminiClient) GenerateBioscanJSON(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	if c.isMock() {
		log.Printf(locales.LogGeminiMockBioscanJSON)
		return mock.MockBioscanJSON(contextInfo), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("empty gemini api key")
	}

	prompt := locales.PromptForBioscan(contextInfo)

	parts := make([]geminiPart, 0, len(photosData)+1)
	for _, data := range photosData {
		if len(data) == 0 {
			continue
		}
		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		})
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}

	parts = append(parts, geminiPart{Text: prompt})

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

// GenerateBodyScanJSON - генерирует JSON премиального отчёта Bioscan PRO
// (Body Intelligence) по фотографиям + опроснику.
func (c *GeminiClient) GenerateBodyScanJSON(
	ctx context.Context,
	photosData [][]byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	if c.isMock() {
		log.Printf(locales.LogGeminiMockBioscanJSON)
		return locales.MockBodyScanJSON, nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("empty gemini api key")
	}

	prompt := locales.PromptForBodyScanJSON(contextInfo)

	parts := make([]geminiPart, 0, len(photosData)+1)
	for _, data := range photosData {
		if len(data) == 0 {
			continue
		}
		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		})
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}

	parts = append(parts, geminiPart{Text: prompt})

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}
