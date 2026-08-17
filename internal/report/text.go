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
func RenderBioscanPlainText(rep models.Report) string {
	var b strings.Builder

	b.WriteString(locales.RptMsgTextHeader)
	b.WriteString("--------------------\n\n")

	if rep.Score > 0 || rep.Level != "" {
		if rep.Score > 0 {
			b.WriteString(locales.RptMsgTextOverallAssessment + strconv.Itoa(rep.Score))
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
		b.WriteString(strings.Join(body, "\n") + "\n\n")
	}

	if rep.Composition != "" {
		b.WriteString(locales.RptMsgTextComposition + rep.Composition + "\n\n")
	}

	if rep.Profile.Composition > 0 || rep.Profile.MuscleDevelopment > 0 ||
		rep.Profile.Balance > 0 || rep.Profile.Potential > 0 {
		b.WriteString(locales.RptMsgTextProfileDev)
		b.WriteString("• " + locales.RptMsgTextCompLabel + strconv.Itoa(rep.Profile.Composition) + "/100\n")
		b.WriteString("• " + locales.RptMsgTextMuscleDevLabel + strconv.Itoa(rep.Profile.MuscleDevelopment) + "/100\n")
		b.WriteString("• " + locales.RptMsgTextBalanceLabel + strconv.Itoa(rep.Profile.Balance) + "/100\n")
		b.WriteString("• " + locales.RptMsgTextPotentialLabel + strconv.Itoa(rep.Profile.Potential) + "/100\n\n")
	}

	if rep.Summary != "" {
		b.WriteString(locales.RptMsgTextSummary + rep.Summary + "\n\n")
	}

	if len(rep.Zones) > 0 {
		b.WriteString(locales.RptMsgTextZoneAssessment)
		for _, z := range rep.Zones {
			b.WriteString("• " + z.Name + " - " + strconv.Itoa(z.Score) + "/100")
			if z.Status != "" {
				b.WriteString(" (" + z.Status + ")")
			}
			b.WriteString("\n")
			if z.Recommendation != "" {
				b.WriteString("  " + locales.RptMsgPdfRecLabel + z.Recommendation + "\n")
			}
		}
		b.WriteString("\n")
	}

	if rep.Posture.Type != "" || rep.Posture.Description != "" {
		b.WriteString(locales.RptMsgTextPosture)
		if rep.Posture.Type != "" {
			b.WriteString("• " + locales.RptMsgPdfTypeLabel + rep.Posture.Type + "\n")
		}
		if rep.Posture.Head != "" {
			b.WriteString("• " + locales.RptMsgPdfHeadLabel + rep.Posture.Head + "\n")
		}
		if rep.Posture.Shoulders != "" {
			b.WriteString("• " + locales.RptMsgPdfShouldersLabel + rep.Posture.Shoulders + "\n")
		}
		if rep.Posture.Pelvis != "" {
			b.WriteString("• " + locales.RptMsgPdfPelvisLabel + rep.Posture.Pelvis + "\n")
		}
		if rep.Posture.Description != "" {
			b.WriteString("• " + rep.Posture.Description + "\n")
		}
		b.WriteString("\n")
	}

	if len(rep.AttentionZones) > 0 {
		b.WriteString(locales.RptMsgTextAttentionZones)
		for _, a := range rep.AttentionZones {
			b.WriteString("• " + a.Name + "\n")
			if a.Problem != "" {
				b.WriteString("  " + locales.RptMsgPdfProblemLabel + a.Problem + "\n")
			}
			if a.Solution != "" {
				b.WriteString("  " + locales.RptMsgPdfSolutionLabel + a.Solution + "\n")
			}
		}
		b.WriteString("\n")
	}

	if len(rep.Recommendations) > 0 {
		b.WriteString(locales.RptMsgTextRecommendations)
		for _, r := range rep.Recommendations {
			b.WriteString("• " + r + "\n")
		}
		b.WriteString("\n")
	}

	if len(rep.Progress.Targets) > 0 || rep.Progress.Recheck != "" {
		b.WriteString(locales.RptMsgTextProgressControl)
		if rep.Progress.Recheck != "" {
			b.WriteString("• " + locales.RptMsgPdfRecheckLabel + rep.Progress.Recheck + "\n")
		}
		for _, t := range rep.Progress.Targets {
			b.WriteString("• " + t + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(locales.RptMsgTextDisclaimer)

	return b.String()
}
