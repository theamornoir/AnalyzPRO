package gemini

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	httpclient "github.com/theamornoir/analyzpro/internal/ai/httpclient"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// rawResponse - сырой HTTP-ответ от Gemini.
type rawResponse struct {
	status int
	body   []byte
}

// doRequest - выполняет HTTP-запрос к Gemini API.
func (c *GeminiClient) doRequest(ctx context.Context, body []byte) (*rawResponse, error) {
	url := fmt.Sprintf(
		locales.GeminiAPIURL,
		c.model,
		c.apiKey,
	)

	log.Printf(locales.LogGeminiRequestModel, c.model)

	log.Printf(locales.LogGeminiSendingRequest)
	startTime := time.Now()

	respBody, err := httpclient.FetchWithRetry(ctx, url, bytes.NewReader(body), 3)
	if err != nil {
		log.Printf(locales.LogGeminiHTTPFailed, err)
		return &rawResponse{status: 503, body: nil}, nil
	}

	elapsed := time.Since(startTime)
	log.Printf(locales.LogGeminiRequestDuration, elapsed)

	return &rawResponse{
		status: 200,
		body:   respBody,
	}, nil
}
