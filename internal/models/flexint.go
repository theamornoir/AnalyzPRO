package models

import (
	"strconv"
	"strings"
)

// FlexInt - целое число, устойчивое к тому, что LLM (YandexGPT) иногда
// возвращает числовое поле строкой (например "score": "82" вместо "score": 82).
// Структуры отчётов (Bioscan/BodyScan/анализ/досье) десериализуются из JSON,
// который модель формирует по промпту; чтобы такой ответ не ломал json.Unmarshal,
// FlexInt принимает и int, и строку с целым/дробным числом, и null/"" (-> 0).
type FlexInt int

// UnmarshalJSON реализует json.Unmarshaler: нормализует число, пришедшее как
// JSON-число, как строку ("82", "7.5", " 90 ") или как null/пустоту.
func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	switch s {
	case "", "null":
		*f = 0
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		*f = FlexInt(n)
		return nil
	}
	if fl, err := strconv.ParseFloat(s, 64); err == nil {
		*f = FlexInt(int(fl))
		return nil
	}
	// Не число - оставляем 0, чтобы не ломать весь отчёт из-за одного поля.
	*f = 0
	return nil
}
