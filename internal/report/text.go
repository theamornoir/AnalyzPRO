package report

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
	reportmodels "github.com/theamornoir/analyzpro/internal/report/models"
)

// RenderBioscanPlainText формирует plain-text (без markdown/звёздочек) версию
// отчёта Bioscan для вывода обычным сообщением в чат. Используется в базовом
// (бесплатном) режиме Bioscan, где результат приходит текстом.
//
// Формат приведён к шаблону пользователя: emoji-маркеры (💪/📏/⚖️/🔥/●/🔸/⚡/
// ⚠️/✦), разделы через ●, зоны через 🔸 (с ⚡ при внимании), рекомендации
// через ✦. Накопление идёт через strings.Builder без конкатенации в циклах.
func RenderBioscanPlainText(rep models.Report) string {
	var b strings.Builder

	b.WriteString(locales.RptMsgTextHeader)

	if rep.Score > 0 || rep.Level != "" {
		b.WriteString(locales.RptMsgTextOverallAssessment)
		if rep.Score > 0 {
			b.WriteString(strconv.Itoa(rep.Score))
			b.WriteString("/100")
		}
		if rep.Level != "" {
			b.WriteString(" · ")
			b.WriteString(rep.Level)
		}
		b.WriteString("\n\n")
	}

	body := []string{}
	if rep.Body.Height != "" {
		body = append(body, locales.RptMsgTextHeight+rep.Body.Height)
	}
	if rep.Body.Weight != "" {
		body = append(body, locales.RptMsgTextWeight+rep.Body.Weight)
	}
	if rep.Body.MuscleMass != "" {
		mm := rep.Body.MuscleMass
		if !strings.Contains(strings.ToLower(mm), "мыш") {
			mm = mm + " мышц"
		}
		body = append(body, locales.RptMsgTextMuscle+mm)
	}
	if rep.Body.Fat != "" {
		body = append(body, locales.RptMsgTextFat+rep.Body.Fat)
	}
	if len(body) > 0 {
		b.WriteString(strings.Join(body, "\n"))
		b.WriteString("\n\n")
	}

	if rep.Composition != "" {
		b.WriteString(locales.RptMsgTextComposition)
		b.WriteString(rep.Composition)
		b.WriteString("\n")
		if rep.Profile.Composition > 0 || rep.Profile.MuscleDevelopment > 0 ||
			rep.Profile.Balance > 0 || rep.Profile.Potential > 0 {
			b.WriteString(locales.RptMsgTextScoreComp)
			b.WriteString(strconv.Itoa(rep.Profile.Composition))
			b.WriteString("/100\n")
			b.WriteString(locales.RptMsgTextScoreMuscle)
			b.WriteString(strconv.Itoa(rep.Profile.MuscleDevelopment))
			b.WriteString("/100\n")
			b.WriteString(locales.RptMsgTextScoreBalance)
			b.WriteString(strconv.Itoa(rep.Profile.Balance))
			b.WriteString("/100\n")
			b.WriteString(locales.RptMsgTextScorePotential)
			b.WriteString(strconv.Itoa(rep.Profile.Potential))
			b.WriteString("/100\n")
		}
		b.WriteString("\n")
	}

	if rep.Summary != "" {
		b.WriteString(locales.RptMsgTextSummary)
		b.WriteString(rep.Summary)
		b.WriteString("\n\n")
	}

	if len(rep.Zones) > 0 {
		b.WriteString(locales.RptMsgTextZoneAssessment)
		for _, z := range rep.Zones {
			b.WriteString(locales.RptMsgTextZoneBullet)
			b.WriteString(z.Name)
			b.WriteString(" — ")
			b.WriteString(strconv.Itoa(z.Score))
			b.WriteString("/100")
			if zoneNeedsAttention(z.Status) {
				b.WriteString(" ⚡")
			}
			b.WriteString("\n")
			if z.Status != "" {
				b.WriteString(z.Status)
				b.WriteString("\n")
			}
			if z.Recommendation != "" {
				b.WriteString(locales.RptMsgTextZoneRec)
				b.WriteString(z.Recommendation)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	if rep.Posture.Type != "" || rep.Posture.Description != "" {
		b.WriteString(locales.RptMsgTextPosture)
		if rep.Posture.Type != "" {
			b.WriteString(locales.RptMsgPdfTypeLabel)
			b.WriteString(rep.Posture.Type)
			b.WriteString("\n")
		}
		if rep.Posture.Head != "" {
			b.WriteString(locales.RptMsgPdfHeadLabel)
			b.WriteString(rep.Posture.Head)
			b.WriteString("\n")
		}
		if rep.Posture.Shoulders != "" {
			b.WriteString(locales.RptMsgPdfShouldersLabel)
			b.WriteString(rep.Posture.Shoulders)
			b.WriteString("\n")
		}
		if rep.Posture.Pelvis != "" {
			b.WriteString(locales.RptMsgPdfPelvisLabel)
			b.WriteString(rep.Posture.Pelvis)
			b.WriteString("\n")
		}
		if rep.Posture.Description != "" {
			b.WriteString(rep.Posture.Description)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rep.AttentionZones) > 0 {
		b.WriteString(locales.RptMsgTextAttentionZones)
		for _, a := range rep.AttentionZones {
			b.WriteString(locales.RptMsgTextAttentionPrefix)
			b.WriteString(a.Name)
			b.WriteString("\n")
			if a.Problem != "" {
				b.WriteString(locales.RptMsgPdfProblemLabel)
				b.WriteString(a.Problem)
				b.WriteString("\n")
			}
			if a.Solution != "" {
				b.WriteString(locales.RptMsgPdfSolutionLabel)
				b.WriteString(a.Solution)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	if len(rep.Recommendations) > 0 {
		b.WriteString(locales.RptMsgTextRecommendations)
		for _, r := range rep.Recommendations {
			b.WriteString(locales.RptMsgTextRecBullet)
			b.WriteString(r)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rep.Progress.Targets) > 0 || rep.Progress.Recheck != "" {
		b.WriteString(locales.RptMsgTextProgressControl)
		if rep.Progress.Recheck != "" {
			b.WriteString(locales.RptMsgTextRecBullet)
			b.WriteString(locales.RptMsgPdfRecheckLabel)
			b.WriteString(rep.Progress.Recheck)
			b.WriteString("\n")
		}
		for _, t := range rep.Progress.Targets {
			b.WriteString(locales.RptMsgTextRecBullet)
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(locales.RptMsgTextDisclaimer)

	return b.String()
}

// zoneNeedsAttention определяет, нужно ли ставить маркер ⚡ напротив зоны
// (статус содержит указание на внимание/детализацию).
func zoneNeedsAttention(status string) bool {
	s := strings.ToLower(status)
	return strings.Contains(s, "треб") ||
		strings.Contains(s, "вниман") ||
		strings.Contains(s, "детализ") ||
		strings.Contains(s, "нужно")
}

// ============================================================================
// Детерминированный рендер анализа крови/лабораторных показателей (как у
// Bioscan): чат-текст строится КОДОМ из структурированного JSON, а не
// «настроением» LLM. Формат гарантирован (emoji-маркеры 🔬/●/✅/❌/✦/⚠️),
// без markdown-разметки и звёздочек.
// ============================================================================

// ParseAdaptiveReportJSON очищает ответ ИИ от markdown-обёртки (```json ... ```)
// и десериализует в reportmodels.AdaptiveReportData. То же преобразование, что
// в service.renderAdaptiveFromJSON, вынесено в пакет report для переиспользования
// детерминированным рендером чата.
func ParseAdaptiveReportJSON(s string) (reportmodels.AdaptiveReportData, error) {
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return reportmodels.AdaptiveReportData{}, fmt.Errorf("empty JSON from AI")
	}
	var data reportmodels.AdaptiveReportData
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		return reportmodels.AdaptiveReportData{}, err
	}
	return data, nil
}

// RenderAnalysisPlainText формирует детерминированный plain-text отчёт
// анализа (кровь/биохимия/липиды и т.п.) для вывода в чат в эмодзи-формате.
// На вход - уже структурированный JSON (reportmodels.AdaptiveReportData),
// полученный от ИИ; формат гарантирован кодом, как в RenderBioscanPlainText.
//
// Структура вывода (строго):
//
//	🔬 АНАЛИЗ КРОВИ · [дата]
//	● РАЗДЕЛ
//	✅/❌ [показатель]
//	[значение] [единица] ([референс])
//	[описание 1-2 предложения]
//	...
//	● ВЫВОДЫ
//	[summary]
//	● ЗОНЫ ВНИМАНИЯ (если есть)
//	⚠️ [пункт]
//	● РЕКОМЕНДАЦИИ
//	✦ [пункт]
//	[disclaimer]
func RenderAnalysisPlainText(data reportmodels.AdaptiveReportData) string {
	var b strings.Builder

	// Заголовок (эмодзи-маркер, дата из бланка, если есть).
	b.WriteString("🔬 АНАЛИЗ КРОВИ")
	if d := strings.TrimSpace(data.Date); d != "" {
		b.WriteString(" · ")
		b.WriteString(d)
	}
	b.WriteString("\n")

	// Интегральный балл здоровья (type=profile) - выводим наверху, как
	// «Общая оценка» у Bioscan. Это субъективная сводка ИИ по всем
	// присланным анализам сразу; общий дисклеймер в конце её покрывает.
	for _, sec := range data.Sections {
		if sec.Type != "profile" {
			continue
		}
		b.WriteString("\n● ПРОФИЛЬ ЗДОРОВЬЯ\n")
		if sec.Score > 0 {
			b.WriteString("Общая оценка: ")
			b.WriteString(strconv.Itoa(sec.Score))
			b.WriteString("/100\n")
		}
		if s := strings.TrimSpace(sec.Summary); s != "" {
			b.WriteString(s)
			b.WriteString("\n")
		}
		break
	}

	// Кровяные секции (показатели) - в порядке из JSON.
	for _, sec := range data.Sections {
		if sec.Type != "blood" {
			continue
		}
		writeAnalysisBloodSection(&b, sec)
	}

	// ВЫВОДЫ (общее заключение по картине в целом).
	if s := strings.TrimSpace(data.Summary); s != "" {
		b.WriteString("\n● ВЫВОДЫ\n")
		b.WriteString(s)
		b.WriteString("\n")
	}

	// Прочие секции: зоны внимания (warning) и рекомендации, затем остальное.
	var warnings, recommendations, others []reportmodels.Section
	for _, sec := range data.Sections {
		if sec.Type == "blood" {
			continue
		}
		switch sec.Type {
		case "profile":
			// выводится наверху (см. выше), здесь пропускаем, чтобы не дублировать
		case "warning":
			warnings = append(warnings, sec)
		case "recommendation":
			recommendations = append(recommendations, sec)
		default:
			others = append(others, sec)
		}
	}

	for _, sec := range warnings {
		if len(sec.List) == 0 {
			continue
		}
		b.WriteString("\n● ЗОНЫ ВНИМАНИЯ\n")
		for _, it := range sec.List {
			b.WriteString("⚠️ ")
			b.WriteString(it)
			b.WriteString("\n")
		}
	}

	for _, sec := range recommendations {
		if len(sec.List) == 0 {
			continue
		}
		b.WriteString("\n● РЕКОМЕНДАЦИИ\n")
		for _, it := range sec.List {
			b.WriteString("✦ ")
			b.WriteString(it)
			b.WriteString("\n")
		}
	}

	for _, sec := range others {
		writeAnalysisListSection(&b, sec.Title, sec.List)
		if s := strings.TrimSpace(sec.Summary); s != "" {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}

	// Дисклеймер (последняя строка, без лишних переводов строк после).
	disc := strings.TrimSpace(data.Disclaimer)
	if disc == "" {
		disc = "Разбор информационный, не заменяет консультацию врача"
	}
	b.WriteString("\n")
	b.WriteString(disc)

	return b.String()
}

// writeAnalysisBloodSection рендерит одну секцию с показателями (type=blood):
// маркер раздела ●, при наличии отклонений - пометка «(главное внимание)»,
// затем блоки показателей.
func writeAnalysisBloodSection(b *strings.Builder, sec reportmodels.Section) {
	title := strings.ToUpper(strings.TrimSpace(sec.Title))
	if title == "" {
		title = "АНАЛИЗ"
	}

	// Помечаем раздел, в котором есть отклонения от нормы.
	hasDev := false
	for _, ind := range sec.Indicators {
		if ind.Status != "normal" && ind.Status != "" {
			hasDev = true
			break
		}
	}

	b.WriteString("\n● ")
	b.WriteString(title)
	if hasDev {
		// Избегаем двойных скобок, если заголовок уже содержит «(...)».
		if strings.Contains(title, "(") {
			b.WriteString(" · главное внимание")
		} else {
			b.WriteString(" (главное внимание)")
		}
	}
	b.WriteString("\n")

	for _, ind := range sec.Indicators {
		marker := "✅"
		if ind.Status != "normal" && ind.Status != "" {
			marker = "❌"
		}
		b.WriteString(marker)
		b.WriteString(" ")
		b.WriteString(ind.Name)
		b.WriteString("\n")

		// Строка значения: [значение] [единица] ([референс]).
		valLine := ind.Value
		if ind.Unit != "" {
			if valLine != "" {
				valLine += " "
			}
			valLine += ind.Unit
		}
		if n := strings.TrimSpace(ind.Normal); n != "" {
			if valLine != "" {
				valLine += " "
			}
			valLine += "(" + n + ")"
		}
		if valLine != "" {
			b.WriteString(valLine)
			b.WriteString("\n")
		}

		if d := strings.TrimSpace(ind.Description); d != "" {
			b.WriteString(d)
			b.WriteString("\n")
		}
	}
}

// writeAnalysisListSection рендерит секцию-список (lifestyle/nutrition/прочее)
// с маркером раздела ● и пунктами через ✦.
func writeAnalysisListSection(b *strings.Builder, title string, list []string) {
	if len(list) == 0 {
		return
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = "РЕКОМЕНДАЦИИ"
	}
	b.WriteString("\n● ")
	b.WriteString(t)
	b.WriteString("\n")
	for _, it := range list {
		b.WriteString("✦ ")
		b.WriteString(it)
		b.WriteString("\n")
	}
}
