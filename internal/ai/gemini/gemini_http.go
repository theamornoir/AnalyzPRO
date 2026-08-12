package gemini

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

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

	if len(c.apiKey) > 10 {
		loggedURL := fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s...%s",
			c.model,
			c.apiKey[:10],
			c.apiKey[len(c.apiKey)-4:],
		)
		log.Printf(locales.LogGeminiRequestURL, loggedURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf(locales.LogGeminiRequestErr, err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf(locales.LogGeminiSendingRequest)
	startTime := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf(locales.LogGeminiHTTPFailed, err)
		return &rawResponse{status: http.StatusServiceUnavailable, body: nil}, nil
	}
	defer resp.Body.Close()

	elapsed := time.Since(startTime)
	log.Printf(locales.LogGeminiRequestDuration, elapsed)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf(locales.LogGeminiReadBodyErr, err)
		return nil, err
	}

	return &rawResponse{
		status: resp.StatusCode,
		body:   respBody,
	}, nil
}
