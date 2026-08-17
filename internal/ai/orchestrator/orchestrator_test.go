package orchestrator

import (
	"context"
	"errors"
	"testing"
)

// mockProviderError - провайдер, который всегда возвращает ошибку.
type mockProviderError struct{}

func (m *mockProviderError) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", errors.New("mock failure")
}
func (m *mockProviderError) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return "", errors.New("mock failure")
}

// mockProviderSuccess - провайдер, который всегда возвращает успех.
type mockProviderSuccess struct{}

func (m *mockProviderSuccess) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "success", nil
}
func (m *mockProviderSuccess) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return "success", nil
}

// TestOrchestratorFallback - при падении первого провайдера оркестратор
// должен корректно переключиться на второй (успешный).
func TestOrchestratorFallback(t *testing.T) {
	o := NewOrchestratorWithProviders([]AIProvider{
		&mockProviderError{},
		&mockProviderSuccess{},
	})

	res, err := o.GenerateAnalysisSummary(context.Background(), "текст анализа")
	if err != nil {
		t.Fatalf("ожидался успех после фоллбэка на второй провайдер, получена ошибка: %v", err)
	}
	if res != "success" {
		t.Fatalf("ожидался результат 'success', получен %q", res)
	}
}

// TestOrchestratorFirstProviderSuccess - если первый провайдер успешен,
// оркестратор не должен вызывать остальные (фоллбэк не нужен).
func TestOrchestratorFirstProviderSuccess(t *testing.T) {
	o := NewOrchestratorWithProviders([]AIProvider{
		&mockProviderSuccess{},
		&mockProviderError{},
	})

	res, err := o.GenerateAnalysisJSON(context.Background(), "текст")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if res != "success" {
		t.Fatalf("ожидался результат 'success', получен %q", res)
	}
}

// TestOrchestratorAllFail - если все провайдеры падают, возвращается ошибка.
func TestOrchestratorAllFail(t *testing.T) {
	o := NewOrchestratorWithProviders([]AIProvider{
		&mockProviderError{},
		&mockProviderError{},
	})

	_, err := o.GenerateDossierJSON(context.Background(), "текст")
	if err == nil {
		t.Fatal("ожидалась ошибка при падении всех провайдеров, получен nil")
	}
}
