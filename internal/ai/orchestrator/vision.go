package orchestrator

import "strings"

// visionImage представляет одно изображение для мультимодального (vision) анализа.
type visionImage struct {
	data     []byte
	mimeType string
}

// isImageMime сообщает, поддерживает ли vision-анализ DeepSeek данный MIME-тип.
// DeepSeek (OpenAI-совместимый API) умеет работать только с изображениями;
// PDF/текст здесь не обрабатываются - для них методы возвращают ошибку, и
// оркестратор переходит к следующему провайдеру (Claude).
func isImageMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// isSupportedMime сообщает, умеет ли Claude анализировать данный MIME-тип.
// Claude (Anthropic) поддерживает изображения и PDF-документы напрямую
// через Messages API (блоки image/document).
func isSupportedMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf"
}
