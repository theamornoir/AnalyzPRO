package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// ConvertHTMLToPDF - конвертирует HTML в PDF через html2pdf.app API
func ConvertHTMLToPDF(html string) ([]byte, error) {
	log.Printf(locales.LogStartingPDFConversion)

	apiKey := os.Getenv("HTML2PDF_API_KEY")
	if apiKey == "" {
		log.Printf(locales.LogPDFKeyNotFound)
		return nil, fmt.Errorf(locales.ErrHTML2PDFKeyNotSet)
	}

	log.Printf(locales.LogPDFKeyFound, apiKey[:10])

	// Пробуем передавать ключ в URL (без Authorization)
	url := fmt.Sprintf("https://api.html2pdf.app/v1/generate?apiKey=%s", apiKey)
	log.Printf(locales.LogPDFURL, url)

	payload := map[string]interface{}{
		"html": html,
		"options": map[string]interface{}{
			"format":              "A4",
			"margin":              10,
			"landscape":           false,
			"printBackground":     true,
			"displayHeaderFooter": false,
			"scale":               1.0,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/pdf")
	// Не добавляем Authorization, ключ уже в URL

	client := &http.Client{Timeout: 30 * time.Second}

	log.Printf(locales.LogSendingRequest)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf(locales.LogPDFStatus, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(locales.ErrAPIError, resp.StatusCode, string(body))
	}

	log.Printf(locales.LogPDFReceived, len(body))
	return body, nil
}
