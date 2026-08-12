package upload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadedFile - описание загруженного пользователем файла.
// Файл хранится на диске, в состоянии сохраняется только путь.
type UploadedFile struct {
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	FileName string `json:"file_name"`
}

// readData - читает содержимое файла с диска.
func (f UploadedFile) readData() ([]byte, error) {
	if f.Path == "" {
		return nil, fmt.Errorf("empty file path")
	}
	return os.ReadFile(f.Path)
}

// cleanup - удаляет файл с диска.
func (f UploadedFile) cleanup() {
	if f.Path != "" {
		_ = os.Remove(f.Path)
	}
}

// saveUploadedFile - сохраняет файл на диск под уникальным именем.
func saveUploadedFile(uploadDir, fileName string, data []byte) string {
	if uploadDir == "" {
		return ""
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		ext = ".bin"
	}
	base := strings.TrimSuffix(filepath.Base(fileName), ext)

	path := filepath.Join(uploadDir, fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), base, ext))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return path
}

// cleanupUploadedFiles - удаляет все временные файлы.
func cleanupUploadedFiles(files []UploadedFile) {
	for _, f := range files {
		f.cleanup()
	}
}
