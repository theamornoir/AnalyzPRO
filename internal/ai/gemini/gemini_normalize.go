package gemini

import (
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// extractGeminiText - извлекает текст из ответа Gemini.
func extractGeminiText(resp *geminiResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}

	var parts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.Join(parts, "\n")
}

// normalizeAIResponse - нормализует текстовый ответ AI.
func normalizeAIResponse(text string) string {
	text = strings.TrimSpace(text)

	if !strings.Contains(text, locales.NormalizeMarkerChart) {
		text = locales.MsgNormalizeAddHeader + text
	}

	if !strings.Contains(text, locales.NormalizeMarkerConclusion) {
		text += locales.MsgNormalizeAddConclusion
	}

	if !strings.Contains(text, locales.NormalizeMarkerWarning) {
		text += locales.MsgNormalizeAddDisclaimer
	}

	text = strings.ReplaceAll(text, locales.NormalizeSeparator, "")
	text = strings.ReplaceAll(text, locales.NormalizeSeparator+"\n", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "**", "")

	lines := strings.Split(text, "\n")
	var result []string
	var lastEmpty bool
	for _, line := range lines {
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty && lastEmpty {
			continue
		}
		result = append(result, line)
		lastEmpty = isEmpty
	}

	return strings.Join(result, "\n")
}

// normalizeJSONResponse - очищает JSON-ответ от markdown-обёртки.
func normalizeJSONResponse(text string) string {
	text = strings.TrimSpace(text)

	text = strings.ReplaceAll(text, "```json", "")
	text = strings.ReplaceAll(text, "```", "")

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	return strings.TrimSpace(text)
}
