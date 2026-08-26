package helpers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
)

// maxDownloadBytes - жёсткий лимит размера скачиваемого файла (25 МБ с
// запасом над лимитом Telegram в 20 МБ на документ). Без ограничения
// злонамеренный/огромный файл целиком читался бы в память (OOM) либо
// зависал бы горутина при медленном/зависшем сервере.
const maxDownloadBytes = 25 * 1024 * 1024

// detectRealMimeType определяет MIME-тип по байтам файла (Magic Numbers).
func detectRealMimeType(data []byte, fileName string) string {
	if len(data) >= 4 {
		// PDF: %PDF
		if data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'D' {
			return "application/pdf"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			return "image/png"
		}
		// WEBP: RIFF...WEBP
		if len(data) >= 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
			data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
			return "image/webp"
		}
	}

	detected := http.DetectContentType(data)
	if detected != "application/octet-stream" && detected != "" {
		return detected
	}

	return detectMimeType(fileName)
}

// detectMimeType - определяет MIME тип по расширению файла.
func detectMimeType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// downloadHTTPClient - переиспользуемый HTTP-клиент со строгим таймаутом.
// http.DefaultClient НЕ имеет таймаута, поэтому зависший сервер Telegram
// держал бы горутину вечно.
var downloadHTTPClient = &http.Client{Timeout: 30 * time.Second}

// DownloadFileByID - скачивает файл по file_id. Возвращает содержимое и
// реальный MIME-тип (по magic numbers). Файл НЕ пишется на диск - вызывающий
// сам решает, сохранять ли его (см. saveUploadedFile в пакете upload).
//
// Защита от DoS: запрос несёт context (отмена по таймауту вызывающего) и
// собственный Timeout клиента, а тело читается через io.LimitReader, чтобы
// огромный/злонамеренный файл не ушёл в OOM.
func DownloadFileByID(
	ctx context.Context,
	b *tgbot.Bot,
	fileID string,
) ([]byte, string, error) {
	file, err := b.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(file), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("загрузка файла: HTTP %d", resp.StatusCode)
	}

	// Лимит размера: читаем на 1 байт больше лимита, чтобы детектить
	// превышение (без полного считывания огромного тела в память).
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxDownloadBytes {
		return nil, "", fmt.Errorf("файл слишком большой: превышен лимит %d байт", maxDownloadBytes)
	}

	mimeType := detectRealMimeType(data, "file")

	return data, mimeType, nil
}
