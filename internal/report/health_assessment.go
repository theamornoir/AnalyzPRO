package report

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/theamornoir/analyzpro/internal/models"
)

// stripHealthJSONFences убирает markdown-ограждения (```json ... ```) и случайный
// текст вокруг JSON, которые модель иногда добавляет даже при явном запросе
// «строго JSON». Без этого json.Unmarshal падает, и структурированный отчёт
// не парсится.
func stripHealthJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.TrimSpace(s)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return strings.TrimSpace(s)
}

// reTrailingComma - «мусор» вроде лишних запятых перед закрывающей скобкой
// ({ "a": 1, }) или внутри массива, из-за которого encoding/json (строгий)
// отказывается парсить. Убираем только запятые, за которыми идут пробелы и
// `}`/`]` (внутри строк таких последовательностей практически не бывает).
var reTrailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// relaxJSON пробует привести нестрогий JSON к валидному виду: срезает
// лишние запятые перед } / ]. Применяется как запасной путь, если прямой
// json.Unmarshal упал (модель вернула почти-валидный JSON).
func relaxJSON(s string) string {
	return reTrailingComma.ReplaceAllString(s, "$1")
}

// ParseHealthAssessmentJSON парсит сырой JSON-ответ ИИ в структуру
// HealthAssessment. Устойчив к markdown-обёртке, поясняющему тексту вокруг
// и нестрогому JSON (лишние запятые). Сначала пробует распарсить «как есть»,
// при неудаче - после срезки мусора и релаксации запятых.
func ParseHealthAssessmentJSON(raw string) (models.HealthAssessment, error) {
	if strings.TrimSpace(raw) == "" {
		return models.HealthAssessment{}, fmt.Errorf("empty health assessment JSON from AI")
	}
	cleaned := stripHealthJSONFences(raw)
	if cleaned == "" {
		return models.HealthAssessment{}, fmt.Errorf("empty health assessment JSON from AI")
	}
	var ha models.HealthAssessment
	if err := json.Unmarshal([]byte(cleaned), &ha); err == nil {
		return ha, nil
	}
	// Запасной путь: модель вернула почти-валидный JSON (лишние запятые и т.п.).
	if relaxed := relaxJSON(cleaned); relaxed != cleaned {
		if err := json.Unmarshal([]byte(relaxed), &ha); err == nil {
			return ha, nil
		}
	}
	return models.HealthAssessment{}, fmt.Errorf("failed to parse health assessment JSON: %w", errFirst(cleaned))
}

// errFirst возвращает ошибку прямого парсинга (для понятного сообщения об
// ошибке). Используется только при окончательном провале.
func errFirst(s string) error {
	var ha models.HealthAssessment
	return json.Unmarshal([]byte(s), &ha)
}

// ValidateHealthAssessment проверяет, что отчёт «Общая оценка здоровья»
// содержателен и не является «пугающим» пустым результатом (garbage-in).
// ИИ иногда возвращает JSON с нулевыми/пустыми полями - если отдать такой
// отчёт пользователю, он увидит карточки «Критично · 0» с пустыми
// комментариями. Чтобы не пугать пользователя и не выдавать мусор за
// аналитику, проверяем минимальную наполненность:
//   - общий индекс в диапазоне 1..100 (0 трактуем как неудавшийся разбор);
//   - хотя бы 3 сферы образа жизни с непустым комментарием и баллом >= 1.
//
// Возвращает ошибку, если отчёт не прошёл проверку (вызывающий код должен
// показать дружелюбное сообщение вместо некачественного PDF/HTML).
func ValidateHealthAssessment(ha models.HealthAssessment) error {
	if ha.HealthIndex < 1 || ha.HealthIndex > 100 {
		return fmt.Errorf("health index out of meaningful range: %d", ha.HealthIndex)
	}
	meaningful := 0
	for _, dim := range ha.Lifestyle {
		if strings.TrimSpace(dim.Comment) != "" && dim.Score >= 1 {
			meaningful++
		}
	}
	if meaningful < 3 {
		return fmt.Errorf("too few meaningful lifestyle spheres: %d", meaningful)
	}
	return nil
}

// RenderHealthAssessmentText формирует читаемый текстовый отчёт «Общая оценка
// здоровья» для вывода в чат Telegram (без markdown-разметки). Использует
// данные структуры HealthAssessment: общий индекс, разбор образа жизни,
// зоны риска и план на 3 месяца.
func RenderHealthAssessmentText(ha models.HealthAssessment) string {
	var b strings.Builder

	b.WriteString("🩺 Общая оценка здоровья\n\n")

	idx := ha.HealthIndex
	if idx < 0 {
		idx = 0
	}
	if idx > 100 {
		idx = 100
	}
	b.WriteString(fmt.Sprintf("📊 Общий индекс здоровья: %d из 100\n", idx))
	b.WriteString(levelLabel(idx))
	b.WriteString("\n")

	if strings.TrimSpace(ha.Summary) != "" {
		b.WriteString("🧭 Разбор образа жизни\n")
		b.WriteString(ha.Summary)
		b.WriteString("\n\n")
	}

	// Разбор по 5 сферам (сон, питание, общее самочувствие, стресс,
	// вредные привычки).
	b.WriteString("🌿 Оценка по сферам\n")
	order := []string{"sleep", "nutrition", "wellbeing", "stress", "habits"}
	labels := map[string]string{
		"sleep":     "Сон",
		"nutrition": "Питание",
		"wellbeing": "Общее самочувствие",
		"stress":    "Стресс",
		"habits":    "Вредные привычки",
	}
	seen := map[string]bool{}
	for _, key := range order {
		if seen[key] {
			continue
		}
		dim, ok := ha.Lifestyle[key]
		if !ok {
			continue
		}
		seen[key] = true
		label := labels[key]
		if label == "" {
			label = key
		}
		b.WriteString(fmt.Sprintf("\n• %s: %d/100\n", label, dim.Score))
		if strings.TrimSpace(dim.Comment) != "" {
			b.WriteString(trimSpaces(dim.Comment) + "\n")
		}
	}
	// Любые прочие ключи lifestyle, не попавшие в известный порядок.
	for key, dim := range ha.Lifestyle {
		if seen[key] {
			continue
		}
		seen[key] = true
		label := labels[key]
		if label == "" {
			label = key
		}
		b.WriteString(fmt.Sprintf("\n• %s: %d/100\n", label, dim.Score))
		if strings.TrimSpace(dim.Comment) != "" {
			b.WriteString(trimSpaces(dim.Comment) + "\n")
		}
	}

	if len(ha.RiskZones) > 0 {
		b.WriteString("\n⚠️ Зоны риска\n")
		for _, z := range ha.RiskZones {
			name := strings.TrimSpace(z.Name)
			if name == "" {
				name = "Внимание"
			}
			b.WriteString(fmt.Sprintf("\n• %s", name))
			if strings.TrimSpace(z.Level) != "" {
				b.WriteString(fmt.Sprintf(" (%s)", z.Level))
			}
			b.WriteString("\n")
			if strings.TrimSpace(z.Description) != "" {
				b.WriteString(trimSpaces(z.Description) + "\n")
			}
		}
	}

	b.WriteString("\n🗓 Персональный план на 3 месяца\n")
	plan := ha.Plan
	if strings.TrimSpace(plan.Sleep) != "" {
		b.WriteString("\n😴 Сон\n" + trimSpaces(plan.Sleep) + "\n")
	}
	if strings.TrimSpace(plan.Nutrition) != "" {
		b.WriteString("\n🥗 Питание\n" + trimSpaces(plan.Nutrition) + "\n")
	}
	if strings.TrimSpace(plan.Wellbeing) != "" {
		b.WriteString("\n🌿 Общее самочувствие\n" + trimSpaces(plan.Wellbeing) + "\n")
	}
	if strings.TrimSpace(plan.Stress) != "" {
		b.WriteString("\n🧘 Стресс\n" + trimSpaces(plan.Stress) + "\n")
	}

	b.WriteString("\n" + disclaimText)
	return b.String()
}

const disclaimText = "Результат носит информационный характер и не является медицинским диагнозом. При ухудшении состояния обратитесь к врачу."

// levelLabel возвращает словесную оценку общего индекса здоровья.
func levelLabel(idx int) string {
	switch {
	case idx >= 80:
		return "Отличный уровень - держите планку."
	case idx >= 65:
		return "Хороший уровень - есть куда расти."
	case idx >= 50:
		return "Средний уровень - стоит подкорректировать образ жизни."
	case idx >= 35:
		return "Сниженный уровень - нужны заметные изменения."
	default:
		return "Низкий уровень - рекомендуется обратиться к врачу."
	}
}

func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}

// nameStripRe строит регэксп для вырезания имени из текста. Имя может
// приходить в любом регистре и быть окружено пробелами/знаками препинания;
// границы слова задаём через «не-буква» ([:alpha:]), чтобы не вырезать имя
// внутри другого слова (например, «Анна» из «банан»). Само имя - отдельная
// группа (group 2), которая при замене отбрасывается, а граничные символы
// (group 1 и group 3: начало строки / не-буква / конец строки) сохраняются.
func nameStripRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^[:alpha:]])(` + regexp.QuoteMeta(name) + `)([^[:alpha:]]|$)`)
}

// SanitizeHealthAssessment вырезает имя пользователя из текстовых полей
// отчёта (summary, comment сфер, описания зон риска и план), если модель
// всё же вписала его («Влад спит 8 часов» -> «спит 8 часов»). Имя
// показывается только в шапке дашборда (ha.Name), в самих разборах оно не
// нужно. Это защитная мера поверх инструкций промпта: даже если ИИ добавит
// имя, оно будет удалено перед рендером HTML/текста.
func SanitizeHealthAssessment(ha *models.HealthAssessment, name string) {
	name = strings.TrimSpace(name)
	if name == "" || ha == nil {
		return
	}
	re := nameStripRe(name)
	clean := func(s string) string {
		if s == "" {
			return s
		}
		return strings.TrimSpace(re.ReplaceAllString(s, "$1$3"))
	}
	ha.Summary = clean(ha.Summary)
	for k, dim := range ha.Lifestyle {
		dim.Comment = clean(dim.Comment)
		ha.Lifestyle[k] = dim
	}
	for i := range ha.RiskZones {
		ha.RiskZones[i].Description = clean(ha.RiskZones[i].Description)
	}
	ha.Plan.Sleep = clean(ha.Plan.Sleep)
	ha.Plan.Nutrition = clean(ha.Plan.Nutrition)
	ha.Plan.Wellbeing = clean(ha.Plan.Wellbeing)
	ha.Plan.Stress = clean(ha.Plan.Stress)
}
