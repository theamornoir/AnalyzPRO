package monitoring

import (
	"encoding/json"
	"strings"
)

// extractMetrics извлекает из JSON-результата анализа плоский набор
// числовых показателей (метрик), пригодных для построения графиков.
//
// Стратегия:
//  1. Если объект похож на «маркер лабораторного анализа» (есть строковое
//     поле-название и числовое поле-значение) — берём пару
//     (название → значение) и НЕ спускаемся внутрь, чтобы не задвоить.
//  2. Иначе рекурсивно обходим объекты (ключи добавляются в путь) и
//     массивы (без индексов, чтобы не раздувать имена).
//  3. Числовой лист сохраняется под текущим путём.
//
// Результат детерминированный; при коллизиях имён побеждает последнее
// значение. Для MVP этого достаточно — пользователь сам выбирает нужные
// метрики в веб-аппе.
func extractMetrics(jsonStr string) map[string]float64 {
	out := map[string]float64{}
	if strings.TrimSpace(jsonStr) == "" {
		return out
	}
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return out
	}
	walk(data, "", out)
	return out
}

// Ключи, которые трактуются как «название показателя».
var labelKeys = []string{"name", "label", "title", "parameter", "test", "analyte", "marker", "indicator", "metric"}

// Ключи, которые трактуются как «значение показателя».
var valueKeys = []string{"value", "result", "level", "amount", "score", "concentration", "reading"}

func walk(node interface{}, path string, out map[string]float64) {
	switch v := node.(type) {
	case map[string]interface{}:
		// 1. Маркер-паттерн: name+value в одном объекте.
		label := findString(v, labelKeys)
		val, hasVal := findNumber(v, valueKeys)
		if label != "" && hasVal {
			out[label] = val
			return
		}
		for k, child := range v {
			childPath := k
			if path != "" {
				childPath = path + " / " + k
			}
			walk(child, childPath, out)
		}
	case []interface{}:
		for _, item := range v {
			walk(item, path, out)
		}
	case float64:
		if path != "" {
			out[path] = v
		}
	case json.Number:
		if f, err := v.Float64(); err == nil && path != "" {
			out[path] = f
		}
	}
}

func findString(m map[string]interface{}, keys []string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func findNumber(m map[string]interface{}, keys []string) (float64, bool) {
	for _, k := range keys {
		switch n := m[k].(type) {
		case float64:
			return n, true
		case json.Number:
			if f, err := n.Float64(); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// ExtractTitle пытается найти осмысленное название результата в JSON.
// Возвращает fallback, если подходящего поля нет.
func ExtractTitle(jsonStr, fallback string) string {
	if strings.TrimSpace(jsonStr) == "" {
		return fallback
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return fallback
	}
	for _, k := range []string{"title", "name", "test_name", "analysis_name", "patient_name", "full_name", "report_title"} {
		if s, ok := data[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return fallback
}
