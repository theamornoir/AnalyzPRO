package httpclient

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

var AIHTTPClient *http.Client

func init() {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
	}

	AIHTTPClient = &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
	}

	log.Printf("HTTP client initialized (timeout=45s, MaxIdleConns=50)")
}

func FetchWithRetry(ctx context.Context, url string, body io.Reader, maxRetries int) ([]byte, error) {
	var lastErr error
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := AIHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 600 {
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, &HTTPError{StatusCode: resp.StatusCode, Message: "rate limit exceeded"}
			}
			return nil, &HTTPError{StatusCode: resp.StatusCode, Message: string(respBody)}
		}

		return respBody, nil
	}

	return nil, lastErr
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}
