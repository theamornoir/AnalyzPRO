package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/mock"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// generate - основной запрос к Gemini для текстового анализа.
func (c *GeminiClient) generate(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf(locales.LogGeminiStartRequest)

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: locales.SystemInstructionAnalysis,
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.2,
			MaxOutputTokens: 3000,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf(locales.LogGeminiMarshalPayloadErr, err)
		return "", err
	}

	log.Printf(locales.LogGeminiBodySize, len(body))

	respBody, err := c.doRequest(ctx, body)
	if err != nil {
		return "", err
	}

	log.Printf(locales.LogGeminiResponseStatus, respBody.status)
	// НЕ логируем содержимое ответа: оно может содержать персональные
	// медицинские данные пользователя (PII). Логируем только статус и размер.
	log.Printf(locales.LogGeminiResponseBodySize, respBody.status, len(respBody.body))

	if respBody.status != http.StatusOK {
		return c.handleErrorResponse(respBody)
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody.body, &result); err != nil {
		log.Printf(locales.LogGeminiUnmarshalErr, err)
		return "", err
	}

	if result.Error != nil {
		log.Printf(locales.LogGeminiAPIError, result.Error.Code, result.Error.Message)
		// ВАЖНО: при ошибках API (429/401/403/500) возвращаем РЕАЛЬНУЮ
		// ошибку, а не заглушку-текст. Иначе оркестратор видит err==nil,
		// считает ответ успешным и НЕ переключается на запасного провайдера
		// (OpenRouter/Yandex) - пользователь тихо получал бы канонический
		// «сервис недоступен» вместо реального анализа.
		if result.Error.Code == 429 || result.Error.Code == 401 || result.Error.Code == 403 || result.Error.Code == 500 {
			return "", fmt.Errorf("gemini API error %d: %s", result.Error.Code, result.Error.Message)
		}
		return "", fmt.Errorf("gemini error: %s", result.Error.Message)
	}

	text := extractGeminiText(&result)
	log.Printf(locales.LogGeminiExtractedLen, len(text))

	if text == "" {
		// Модель вернула пустой текст - используем запасной мок-отчёт.
		// Тело ответа НЕ логируем (может содержать PII).
		log.Printf(locales.LogGeminiEmptyResponse)
		return mock.MockAnalysis(""), nil
	}

	log.Printf(locales.LogGeminiSuccess)
	return normalizeAIResponse(text), nil
}

// generateRaw - запрос к Gemini для JSON-ответа (без нормализации текста).
func (c *GeminiClient) generateRaw(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf(locales.LogGeminiStartRawRequest)

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: locales.SystemInstructionJSON,
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 4000,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	respBody, err := c.doRequest(ctx, body)
	if err != nil {
		return "", err
	}

	if respBody.status != http.StatusOK {
		return "", fmt.Errorf("gemini error (HTTP %d, body %d bytes)", respBody.status, len(respBody.body))
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody.body, &result); err != nil {
		return "", err
	}

	text := extractGeminiText(&result)
	log.Printf(locales.LogGeminiRawJSONLen, len(text))

	cleanedText := strings.TrimSpace(text)
	cleanedText = strings.TrimPrefix(cleanedText, "```json")
	cleanedText = strings.TrimPrefix(cleanedText, "```")
	cleanedText = strings.TrimSuffix(cleanedText, "```")
	cleanedText = strings.TrimSpace(cleanedText)

	return cleanedText, nil
}

// handleErrorResponse - обрабатывает не-OK ответы от Gemini.
func (c *GeminiClient) handleErrorResponse(respBody *rawResponse) (string, error) {
	log.Printf(locales.LogGeminiNonOKStatus, respBody.status)

	if respBody.status == 429 {
		return "", fmt.Errorf("gemini rate limit (HTTP 429)")
	}

	if respBody.status == 400 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody.body, &errResp); err == nil {
			if strings.Contains(errResp.Error.Message, "location is not supported") {
				return "", fmt.Errorf("gemini location not supported (HTTP 400)")
			}
		}
		// Возвращаем ошибку, чтобы оркестратор переключился на запасного
		// провайдера, вместо маскировки под успешный ответ-заглушку.
		return "", fmt.Errorf("gemini service unavailable (HTTP 400): %s", errResp.Error.Message)
	}

	var errResp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody.body, &errResp); err == nil {
		log.Printf(locales.LogGeminiErrorDetails, errResp.Error.Code, errResp.Error.Message)

		if errResp.Error.Code == 429 || errResp.Error.Code == 401 || errResp.Error.Code == 403 || errResp.Error.Code == 500 {
			return "", fmt.Errorf("gemini API error %d: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return "", fmt.Errorf("gemini error %d: %s", errResp.Error.Code, errResp.Error.Message)
	}

	if respBody.status == 401 || respBody.status == 403 || respBody.status == 500 {
		return "", fmt.Errorf("gemini request failed with status %d", respBody.status)
	}

	return "", fmt.Errorf("gemini request failed with status %d (body %d bytes)",
		respBody.status, len(respBody.body))
}
