package ocr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func ExtractTextFromFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty file path")
	}

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractPDFText(path)
	case ".png", ".jpg", ".jpeg", ".bmp", ".webp":
		return extractImageText(path)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

func extractPDFText(path string) (string, error) {
	cmd := exec.Command("python3", "-c", `from pypdf import PdfReader
import sys, re
p = sys.argv[1]
reader = PdfReader(p)
out = []
for page in reader.pages:
    text = page.extract_text() or ""
    out.append(text)
text = "\n".join(out)
print(text)
`, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdf extraction failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	clean := SanitizeExtractedText(string(output))
	if clean == "" {
		return "", fmt.Errorf("pdf extraction returned empty content")
	}
	return clean, nil
}

func extractImageText(path string) (string, error) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", fmt.Errorf("tesseract not installed: %w", err)
	}

	cmd := exec.Command("python3", "-c", `import sys, pytesseract
from PIL import Image
p = sys.argv[1]
img = Image.open(p)
text = pytesseract.image_to_string(img, config='--psm 6')
print(text)
`, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ocr failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	clean := SanitizeExtractedText(string(output))
	if clean == "" {
		return "", fmt.Errorf("ocr returned empty content")
	}
	return clean, nil
}

func SanitizeExtractedText(raw string) string {
	clean := strings.ReplaceAll(raw, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	clean = regexp.MustCompile(`\n{3,}`).ReplaceAllString(clean, "\n\n")
	clean = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(clean, "\n")
	clean = regexp.MustCompile(`\n{2,}`).ReplaceAllString(clean, "\n\n")
	clean = strings.TrimSpace(clean)
	return clean
}
