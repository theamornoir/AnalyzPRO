package upload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// TestCleanupUploadedFilesByChat проверяет, что отмена загрузки удаляет
// реальные файлы пользователя с диска (устраняет утечку из аудита MAJOR #1).
func TestCleanupUploadedFilesByChat(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "1_a.pdf")
	f2 := filepath.Join(dir, "2_b.png")
	if err := os.WriteFile(f1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	sm := states.NewMemoryStateManager("")
	files := []UploadedFile{{Path: f1}, {Path: f2}}
	data, _ := json.Marshal(files)
	sm.SetUserData(1, "uploaded_files", string(data))

	cleanupUploadedFilesByChat(sm, 1)

	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed", f1)
	}
	if _, err := os.Stat(f2); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed", f2)
	}
}

// TestCleanupOldUploads проверяет фоновый сборщик брошенных файлов:
// удаляет только старше порога, оставляет «живые» файлы.
func TestCleanupOldUploads(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.pdf")
	newF := filepath.Join(dir, "new.pdf")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newF, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldT := time.Now().Add(-10 * time.Hour)
	if err := os.Chtimes(old, oldT, oldT); err != nil {
		t.Fatal(err)
	}

	cleanupOldUploads(dir, time.Hour)

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("expected old file removed")
	}
	if _, err := os.Stat(newF); err != nil {
		t.Fatalf("expected new file kept: %v", err)
	}
}
