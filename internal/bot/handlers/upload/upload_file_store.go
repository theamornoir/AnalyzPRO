package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/bot/states"
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

// cleanupUploadedFilesByChat - удаляет временные файлы пользователя,
// сохранённые в состоянии (uploaded_files). Используется при отмене
// загрузки и при старте новой сессии загрузки (очистка «висячих» файлов
// прошлой брошенной сессии), чтобы файлы не копились на диске.
func cleanupUploadedFilesByChat(stateManager states.StateManager, chatID int64) {
	files := getUploadedFiles(stateManager, chatID)
	cleanupUploadedFiles(files)
}

// StartUploadCleanupLoop запускает фоновый цикл очистки брошенных временных
// файлов загрузки (отмена / бросание чата / падение до анализа). Удаляет
// файлы старее maxAge по mtime, чтобы диск не забивался. Анализ длится не
// дольше ~120с, поэтому порог в часы безопасен для «живых» файлов.
// Завершается вместе с ctx (остановка бота).
func StartUploadCleanupLoop(ctx context.Context, uploadDir string, maxAge, interval time.Duration) {
	if uploadDir == "" {
		return
	}
	// Однократная уборка при старте - удаляем файлы, оставшиеся от
	// предыдущих (аварийно завершённых) запусков бота.
	cleanupOldUploads(uploadDir, maxAge)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupOldUploads(uploadDir, maxAge)
			}
		}
	}()
}

// cleanupOldUploads - удаляет файлы в uploadDir старше maxAge (по mtime).
func cleanupOldUploads(uploadDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(uploadDir, e.Name()))
		}
	}
}
