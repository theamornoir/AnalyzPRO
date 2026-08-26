package tmpocrtest
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("KEY")
	folder := os.Getenv("FOLDER")
	if key == "" || folder == "" {
		fmt.Println("set KEY and FOLDER")
		os.Exit(1)
	}
	png, _ := os.ReadFile("/tmp/px.png")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	cfgBytes, _ := json.Marshal(map[string]any{
		"languageCodes": []string{"ru", "en"},
		"model":         "page",
	})
	if err := mw.WriteField("config", string(cfgBytes)); err != nil {
		panic(err)
	}
	part, err := mw.CreateFormFile("file", "document")
	if err != nil {
		panic(err)
	}
	part.Write(png)
	mw.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://ocr.api.cloud.yandex.net/ocr/v1/recognizeText", &buf)
	req.Header.Set("Authorization", "Api-Key "+key)
	req.Header.Set("x-folder-id", folder)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("transport err:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d\n%s\n", resp.StatusCode, string(b))
}
