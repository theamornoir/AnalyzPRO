package helpers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tgbot "github.com/go-telegram/bot"
)

// detectRealMimeType определяет MIME-тип по байтам файла (Magic Numbers).
func detectRealMimeType(data []byte, fileName string) string {
	if len(data) >= 4 {
		// PDF: %PDF
		if data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'F' {
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

// DownloadFileByID - скачивает файл по file_id.
func DownloadFileByID(
	ctx context.Context,
	b *tgbot.Bot,
	fileID string,
	uploadDir string,
) ([]byte, string, error) {
	file, err := b.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	resp, err := http.Get(b.FileDownloadLink(file))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	mimeType := detectRealMimeType(data, "file")

	if uploadDir != "" {
		_ = os.MkdirAll(uploadDir, 0o755)
		_ = os.WriteFile(filepath.Join(uploadDir, "photo.jpg"), data, 0o644)
	}

	return data, mimeType, nil
}
