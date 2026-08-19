package report

import (
	"strconv"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

// RenderBioscanPlainText формирует plain-text (без markdown/форматирования)
// версию отчёта Bioscan для вывода обычным сообщением в чат. Используется в
// базовом (бесплатном) режиме Bioscan, где результат приходит текстом.
//
// Для максимальной эффективности всё накопление идёт через strings.Builder:
// внутри циклов никакой конкатенации строк ("a" + b + "c"), только
// последовательные WriteString каждого фрагмента. Это исключает лишние
// аллокации промежуточных строк (равнозначно тому, что требует gopls S1021).
func RenderBioscanPlainText(rep models.Report) string {
	var b strings.Builder

	b.WriteString(locales.RptMsgTextHeader)
	b.WriteString("--------------------\n\n")

	if rep.Score > 0 || rep.Level != "" {
		if rep.Score > 0 {
			b.WriteString(locales.RptMsgTextOverallAssessment)
			b.WriteString(strconv.Itoa(rep.Score))
		}
		if rep.Level != "" {
			if rep.Score > 0 {
				b.WriteString("  ·  ")
			}
			b.WriteString(rep.Level)
		}
		b.WriteString("\n\n")
	}

	body := []string{}
	if rep.Body.Height != "" {
		body = append(body, locales.RptMsgPdfHeightLabel+rep.Body.Height)
	}
	if rep.Body.Weight != "" {
		body = append(body, locales.RptMsgPdfWeightLabel+rep.Body.Weight)
	}
	if rep.Body.MuscleMass != "" {
		body = append(body, locales.RptMsgPdfMuscleMassLabel+rep.Body.MuscleMass)
	}
	if rep.Body.Fat != "" {
		body = append(body, locales.RptMsgPdfBodyFatLabel+rep.Body.Fat)
	}
	if len(body) > 0 {
		b.WriteString(strings.Join(body, "\n"))
		b.WriteString("\n\n")
	}

	if rep.Composition != "" {
		b.WriteString(locales.RptMsgTextComposition)
		b.WriteString(rep.Composition)
		b.WriteString("\n\n")
	}

	if rep.Profile.Composition > 0 || rep.Profile.MuscleDevelopment > 0 ||
		rep.Profile.Balance > 0 || rep.Profile.Potential > 0 {
		b.WriteString(locales.RptMsgTextProfileDev)
		b.WriteString("• ")
		b.WriteString(locales.RptMsgTextCompLabel)
		b.WriteString(strconv.Itoa(rep.Profile.Composition))
		b.WriteString("/100\n")
		b.WriteString("• ")
		b.WriteString(locales.RptMsgTextMuscleDevLabel)
		b.WriteString(strconv.Itoa(rep.Profile.MuscleDevelopment))
		b.WriteString("/100\n")
		b.WriteString("• ")
		b.WriteString(locales.RptMsgTextBalanceLabel)
		b.WriteString(strconv.Itoa(rep.Profile.Balance))
		b.WriteString("/100\n")
		b.WriteString("• ")
		b.WriteString(locales.RptMsgTextPotentialLabel)
		b.WriteString(strconv.Itoa(rep.Profile.Potential))
		b.WriteString("/100\n\n")
	}

	if rep.Summary != "" {
		b.WriteString(locales.RptMsgTextSummary)
		b.WriteString(rep.Summary)
		b.WriteString("\n\n")
	}

	if len(rep.Zones) > 0 {
		b.WriteString(locales.RptMsgTextZoneAssessment)
		for _, z := range rep.Zones {
			b.WriteString("• ")
			b.WriteString(z.Name)
			b.WriteString(" - ")
			b.WriteString(strconv.Itoa(z.Score))
			b.WriteString("/100")
			if z.Status != "" {
				b.WriteString(" (")
				b.WriteString(z.Status)
				b.WriteString(")")
			}
			b.WriteString("\n")
			if z.Recommendation != "" {
				b.WriteString("  ")
				b.WriteString(locales.RptMsgPdfRecLabel)
				b.WriteString(z.Recommendation)
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	if rep.Posture.Type != "" || rep.Posture.Description != "" {
		b.WriteString(locales.RptMsgTextPosture)
		if rep.Posture.Type != "" {
			b.WriteString("• ")
			b.WriteString(locales.RptMsgPdfTypeLabel)
			b.WriteString(rep.Posture.Type)
			b.WriteString("\n")
		}
		if rep.Posture.Head != "" {
			b.WriteString("• ")
			b.WriteString(locales.RptMsgPdfHeadLabel)
			b.WriteString(rep.Posture.Head)
			b.WriteString("\n")
		}
		if rep.Posture.Shoulders != "" {
			b.WriteString("• ")
			b.WriteString(locales.RptMsgPdfShouldersLabel)
			b.WriteString(rep.Posture.Shoulders)
			b.WriteString("\n")
		}
		if rep.Posture.Pelvis != "" {
			b.WriteString("• ")
			b.WriteString(locales.RptMsgPdfPelvisLabel)
			b.WriteString(rep.Posture.Pelvis)
			b.WriteString("\n")
		}
		if rep.Posture.Description != "" {
			b.WriteString("• ")
			b.WriteString(rep.Posture.Description)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rep.AttentionZones) > 0 {
		b.WriteString(locales.RptMsgTextAttentionZones)
		for _, a := range rep.AttentionZones {
			b.WriteString("• ")
			b.WriteString(a.Name)
			b.WriteString("\n")
			if a.Problem != "" {
				b.WriteString("  ")
				b.WriteString(locales.RptMsgPdfProblemLabel)
				b.WriteString(a.Problem)
				b.WriteString("\n")
			}
			if a.Solution != "" {
				b.WriteString("  ")
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
			b.WriteString("• ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(rep.Progress.Targets) > 0 || rep.Progress.Recheck != "" {
		b.WriteString(locales.RptMsgTextProgressControl)
		if rep.Progress.Recheck != "" {
			b.WriteString("• ")
			b.WriteString(locales.RptMsgPdfRecheckLabel)
			b.WriteString(rep.Progress.Recheck)
			b.WriteString("\n")
		}
		for _, t := range rep.Progress.Targets {
			b.WriteString("• ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(locales.RptMsgTextDisclaimer)

	return b.String()
}
