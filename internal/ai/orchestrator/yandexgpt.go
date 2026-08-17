package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/httpclient"
	"github.com/theamornoir/analyzpro/internal/locales"
)

const (
	yandexGPTEndpoint = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	// yandexDefaultTextModel — дешевле и входит в бесплатную квоту; подходит
	// для текстовых/JSON-задач. Переопределяется через YANDEX_GPT_MODEL.
	yandexDefaultTextModel = "yandexgpt-lite"
	// yandexDefaultVisionModel — мультимодальность (анализ фото) есть только
	// у полной модели yandexgpt, у lite её нет. Переопределяется через
	// YANDEX_GPT_VISION_MODEL.
	yandexDefaultVisionModel = "yandexgpt"
)

// YandexGPTProvider — провайдер YandexGPT (Yandex Cloud Foundation Models).
// Работает из РФ без гео-блоков (в отличие от Gemini), есть бесплатная квота,
// поэтому используется как основной провайдер. Gemini/DeepSeek/Claude
// остаются фоллбэками на случай его недоступности.
type YandexGPTProvider struct {
	apiKey      string
	folderID    string
	textModel   string
	visionModel string
}

// NewYandexGPTProvider создаёт провайдер с ключами из окружения:
//
//	YANDEX_GPT_API_KEY    — API-ключ каталога Yandex Cloud
//	YANDEX_GPT_FOLDER_ID  — ID каталога (folder-id)
//	YANDEX_GPT_MODEL      — (опц.) текстовая модель, по умолчанию yandexgpt-lite
//	YANDEX_GPT_VISION_MODEL — (опц.) модель для фото, по умолчанию yandexgpt
//
// Возвращает nil, если не задан API-ключ или folder-id — провайдер тогда не
// добавляется в оркестратор.
func NewYandexGPTProvider() *YandexGPTProvider {
	apiKey := os.Getenv("YANDEX_GPT_API_KEY")
	folderID := os.Getenv("YANDEX_GPT_FOLDER_ID")
	if apiKey == "" || folderID == "" {
		return nil
	}
	textModel := os.Getenv("YANDEX_GPT_MODEL")
	if textModel == "" {
		textModel = yandexDefaultTextModel
	}
	visionModel := os.Getenv("YANDEX_GPT_VISION_MODEL")
	if visionModel == "" {
		visionModel = yandexDefaultVisionModel
	}
	return &YandexGPTProvider{
		apiKey:      apiKey,
		folderID:    folderID,
		textModel:   textModel,
		visionModel: visionModel,
	}
}

// yandexMessage — одно сообщение в запросе к YandexGPT.
// Images (необязательно) содержит data-URL изображений для мультимодального
// (vision) режима.
type yandexMessage struct {
	Role   string   `json:"role"`
	Text   string   `json:"text"`
	Images []string `json:"images,omitempty"`
}

// yandexCompletionOptions — параметры генерации.
type yandexCompletionOptions struct {
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
}

// yandexRequest — тело запроса к YandexGPT completion API.
type yandexRequest struct {
	ModelURI          string                  `json:"modelUri"`
	CompletionOptions yandexCompletionOptions `json:"completionOptions"`
	Messages          []yandexMessage         `json:"messages"`
}

// yandexResponse — ответ YandexGPT completion API.
type yandexResponse struct {
	Result struct {
		Alternatives []struct {
			Message struct {
				Role string `json:"role"`
				Text string `json:"text"`
			} `json:"message"`
			Status string `json:"status"`
		} `json:"alternatives"`
	} `json:"result"`
	Error *struct {
		GrpcCode int    `json:"grpcCode"`
		HTTPCode int    `json:"httpCode"`
		Message  string `json:"message"`
	} `json:"error"`
}

// complete — единая точка вызова YandexGPT completion API.
// Использует общий AIHTTPClient (таймаут/прокси как у остальных провайдеров).
func (p *YandexGPTProvider) complete(ctx context.Context, model string, messages []yandexMessage, maxTokens int, temperature float64) (string, error) {
	reqBody := yandexRequest{
		ModelURI: fmt.Sprintf("gpt://%s/%s", p.folderID, model),
		CompletionOptions: yandexCompletionOptions{
			Stream:      false,
			Temperature: temperature,
			MaxTokens:   maxTokens,
		},
		Messages: messages,
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("yandexgpt marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, yandexGPTEndpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("yandexgpt request build error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+p.apiKey)
	req.Header.Set("x-folder-id", p.folderID)

	log.Printf("📤 YandexGPT request: model=%s, messages=%d", model, len(messages))

	resp, err := httpclient.AIHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("yandexgpt http error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("yandexgpt read error: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("yandexgpt request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed yandexResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("yandexgpt decode error: %w (body: %s)", err, string(respBody))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("yandexgpt error: %s", parsed.Error.Message)
	}
	if len(parsed.Result.Alternatives) == 0 {
		return "", fmt.Errorf("yandexgpt returned no alternatives")
	}

	text := strings.TrimSpace(parsed.Result.Alternatives[0].Message.Text)
	if text == "" {
		return "", fmt.Errorf("yandexgpt returned empty response")
	}
	return text, nil
}

// GenerateAnalysisSummary — текстовый анализ (консультация/обычный анализ).
func (p *YandexGPTProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return p.complete(ctx, p.textModel, []yandexMessage{
		{Role: "system", Text: "Ты — опытный медицинский аналитик. Проанализируй данные и дай практичные рекомендации."},
		{Role: "user", Text: userInput},
	}, 3000, 0.3)
}

// GenerateAnalysisJSON — структурированный JSON-анализ по тексту.
func (p *YandexGPTProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	text, err := p.complete(ctx, p.textModel, []yandexMessage{
		{Role: "system", Text: "Ты — медицинский аналитик. Верни ответ строго в формате JSON, без markdown-разметки и пояснений."},
		{Role: "user", Text: userInput},
	}, 4000, 0.1)
	if err != nil {
		return "", err
	}
	return yandexStripJSON(text), nil
}

// GenerateAnalysisFromFileWithContext — анализ изображения (анализ/документ)
// с контекстом; возвращает текстовый разбор. YandexGPT умеет работать только
// с изображениями, поэтому не-изображения отвергаются (оркестратор уйдёт к
// следующему провайдеру).
func (p *YandexGPTProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isImageMime(mimeType) {
		return "", fmt.Errorf("yandexgpt supports only image files for analysis")
	}
	return p.complete(ctx, p.visionModel, []yandexMessage{
		{
			Role:   "user",
			Text:   contextText,
			Images: []string{"data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)},
		},
	}, 4000, 0.3)
}

// GenerateBioscanJSON — анализ фото для bioscan, возвращает JSON.
func (p *YandexGPTProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	var imgs []string
	for _, d := range photosData {
		if len(d) > 0 {
			imgs = append(imgs, "data:"+mimeType+";base64,"+base64.StdEncoding.EncodeToString(d))
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	text, err := p.complete(ctx, p.visionModel, []yandexMessage{
		{
			Role:   "user",
			Text:   locales.PromptForBioscan(contextInfo),
			Images: imgs,
		},
	}, 8000, 0.2)
	if err != nil {
		return "", err
	}
	return yandexStripJSON(text), nil
}

// GenerateBodyScanJSON — генерирует JSON премиального отчёта Bioscan PRO.
func (p *YandexGPTProvider) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	var imgs []string
	for _, d := range photosData {
		if len(d) > 0 {
			imgs = append(imgs, "data:"+mimeType+";base64,"+base64.StdEncoding.EncodeToString(d))
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	text, err := p.complete(ctx, p.visionModel, []yandexMessage{
		{
			Role:   "user",
			Text:   locales.PromptForBodyScanJSON(contextInfo),
			Images: imgs,
		},
	}, 8000, 0.2)
	if err != nil {
		return "", err
	}
	return yandexStripJSON(text), nil
}

// GenerateAnalysisFromFileJSON — анализ изображения, возвращает JSON.
func (p *YandexGPTProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isImageMime(mimeType) {
		return "", fmt.Errorf("yandexgpt supports only image files for analysis")
	}
	text, err := p.complete(ctx, p.visionModel, []yandexMessage{
		{
			Role:   "user",
			Text:   locales.PromptForAnalysisJSON(contextText),
			Images: []string{"data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)},
		},
	}, 8000, 0.1)
	if err != nil {
		return "", err
	}
	return yandexStripJSON(text), nil
}

// GenerateDossierJSON — генерирует JSON универсального отчёта-досье здоровья.
func (p *YandexGPTProvider) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	text, err := p.complete(ctx, p.textModel, []yandexMessage{
		{Role: "system", Text: "Ты — опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown-разметки и пояснений."},
		{Role: "user", Text: userInput},
	}, 8000, 0.1)
	if err != nil {
		return "", err
	}
	return yandexStripJSON(text), nil
}

// yandexStripJSON — убирает markdown-обёртку ```json ... ```, которую модель
// иногда добавляет вокруг JSON. Нужно, т.к. bioscan-парсер ожидает чистый JSON.
func yandexStripJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
