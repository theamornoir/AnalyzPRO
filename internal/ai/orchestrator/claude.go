package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

// claudeModel — модель Claude для vision/документного анализа.
const claudeModel = "claude-3-5-sonnet-20241022"

// claudeHTTPClient — HTTP-клиент для прямых вызовов Anthropic Messages API.
// Использует ProxyFromEnvironment (как и остальные AI-клиенты) и увеличенный
// таймаут, т.к. анализ PDF/изображений может быть долгим.
var claudeHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
	},
}

// claudeAttachment — одно вложение (изображение или PDF) для мультимодального запроса.
type claudeAttachment struct {
	data     []byte
	mimeType string
}

// ClaudeProvider — провайдер Claude через langchaingo (текст) и прямой
// HTTP к Anthropic Messages API (изображения и PDF-документы).
// Является рабочим фоллбэком, когда Gemini недоступен (например, гео-блок),
// и единственным провайдером, умеющим анализировать PDF в обход Gemini.
type ClaudeProvider struct {
	llm    *anthropic.LLM
	apiKey string
	model  string
}

// NewClaudeProvider создаёт ClaudeProvider с ключом из окружения.
// Возвращает nil если ключ пустой.
func NewClaudeProvider() *ClaudeProvider {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}
	client, err := anthropic.New(
		anthropic.WithToken(apiKey),
		anthropic.WithModel(claudeModel),
	)
	if err != nil {
		return nil
	}
	return &ClaudeProvider{
		llm:    client,
		apiKey: apiKey,
		model:  claudeModel,
	}
}

// GenerateAnalysisSummary — генерирует текстовый анализ.
func (p *ClaudeProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	if p.llm == nil {
		return "", fmt.Errorf("claude client not initialized")
	}

	result, err := p.llm.Call(ctx,
		"Ты — медицинский аналитик. Проанализируй данные и дай рекомендации.\n\n"+userInput,
		llms.WithMaxTokens(3000),
		llms.WithTemperature(0.2),
	)
	if err != nil {
		return "", err
	}

	return result, nil
}

// GenerateAnalysisJSON — генерирует JSON-анализ.
func (p *ClaudeProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	if p.llm == nil {
		return "", fmt.Errorf("claude client not initialized")
	}

	result, err := p.llm.Call(ctx,
		"Ты — медицинский аналитик. Верни ответ в формате JSON.\n\n"+userInput,
		llms.WithMaxTokens(4000),
		llms.WithTemperature(0.1),
	)
	if err != nil {
		return "", err
	}

	return result, nil
}

// GenerateAnalysisFromFileWithContext — анализирует файл (изображение или PDF) с контекстом.
func (p *ClaudeProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf("claude supports only image/pdf files for analysis")
	}
	return p.callClaudeMessages(ctx,
		"Ты — опытный врач-диагност. Проанализируй приложенные медицинские изображения/документы и дай развёрнутый анализ с рекомендациями.",
		contextText,
		[]claudeAttachment{{data: data, mimeType: mimeType}},
		4000)
}

// GenerateBioscanJSON — анализирует фото/PDF для bioscan и возвращает JSON.
func (p *ClaudeProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("claude api key not set")
	}
	atts := make([]claudeAttachment, 0, len(photosData))
	for _, d := range photosData {
		if len(d) > 0 {
			atts = append(atts, claudeAttachment{data: d, mimeType: mimeType})
		}
	}
	if len(atts) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return p.callClaudeMessages(ctx,
		"Ты — опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForBioscan(contextInfo),
		atts,
		8000)
}

// GenerateAnalysisFromFileJSON — анализирует файл (изображение или PDF) и возвращает JSON.
func (p *ClaudeProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf("claude supports only image/pdf files for analysis")
	}
	return p.callClaudeMessages(ctx,
		"Ты — опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForAnalysisJSON(contextText),
		[]claudeAttachment{{data: data, mimeType: mimeType}},
		8000)
}

// GenerateDossierJSON — генерирует JSON универсального отчёта-досье здоровья.
func (p *ClaudeProvider) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	if p.llm == nil {
		return "", fmt.Errorf("claude client not initialized")
	}
	result, err := p.llm.Call(ctx,
		"Ты — опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown и комментариев.\n\n"+userInput,
		llms.WithMaxTokens(8000),
		llms.WithTemperature(0.1),
	)
	if err != nil {
		return "", err
	}
	return result, nil
}

// callClaudeMessages — выполняет мультимодальный запрос к Anthropic Messages API
// напрямую (обходит langchaingo, который не умеет отдавать PDF-документы).
// Поддерживает одновременно изображения (image-блок) и PDF (document-блок).
func (p *ClaudeProvider) callClaudeMessages(ctx context.Context, systemPrompt, textPrompt string, attachments []claudeAttachment, maxTokens int) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("claude api key not set")
	}

	content := []any{
		map[string]any{"type": "text", "text": textPrompt},
	}
	hasAttachment := false
	for _, att := range attachments {
		if len(att.data) == 0 {
			continue
		}
		block, err := claudeContentBlock(att)
		if err != nil {
			return "", err
		}
		content = append(content, block)
		hasAttachment = true
	}
	if !hasAttachment {
		return "", fmt.Errorf("no attachment data provided")
	}

	body := map[string]any{
		"model":      p.model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []any{
			map[string]any{"role": "user", "content": content},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := claudeHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("claude api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("claude response parse error: %w (body: %s)", err, string(respBody))
	}

	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("claude returned empty response")
	}

	return text, nil
}

// claudeContentBlock преобразует вложение в content-блок Anthropic API:
// image/* → image-блок, application/pdf → document-блок.
func claudeContentBlock(att claudeAttachment) (map[string]any, error) {
	base64Data := base64.StdEncoding.EncodeToString(att.data)

	if strings.HasPrefix(att.mimeType, "image/") {
		return map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": att.mimeType,
				"data":       base64Data,
			},
		}, nil
	}

	if att.mimeType == "application/pdf" {
		return map[string]any{
			"type": "document",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "application/pdf",
				"data":       base64Data,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported mime type for claude: %s", att.mimeType)
}
