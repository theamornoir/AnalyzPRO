package report

import (
	"bytes"
	_ "embed"
	"strconv"

	"github.com/jung-kurt/gofpdf"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

//go:embed fonts/arial.ttf
var arialFontBytes []byte

const (
	pdfMargin = 15.0
	pdfPageW  = 210.0
	pdfPageH  = 297.0
)

// RenderBioscanPDF строит PDF-отчёт Bioscan (кириллица через встроенный
// Arial). Возвращает сырые байты готового PDF-документа. Рисует текстовые
// секции и простые столбчатые диаграммы (графики) для оценок профиля и зон.
func RenderBioscanPDF(rep models.Report) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, pdfMargin)
	pdf.AddUTF8FontFromBytes("Arial", "", arialFontBytes)
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.AddPage()

	w := &pdfWriter{pdf: pdf}

	// Шапка (цветная полоса + заголовок).
	pdf.SetFillColor(33, 70, 110)
	pdf.Rect(0, 0, pdfPageW, 22, "F")
	pdf.SetXY(pdfMargin, 6)
	pdf.SetFont("Arial", "", 16)
	pdf.SetTextColor(255, 255, 255)
	pdf.Cell(0, 8, locales.RptMsgPdfHeader)

	pdf.SetXY(pdfMargin, 15)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(220, 230, 245)
	sub := locales.RptMsgPdfSubHeader
	if rep.Profile.Name != "" {
		sub += " · " + rep.Profile.Name
	}
	pdf.Cell(0, 5, sub)

	pdf.SetY(28)

	// Общий балл и уровень.
	if rep.Score > 0 || rep.Level != "" {
		w.heading(locales.RptMsgPdfOverallAssessment)
		line := ""
		if rep.Score > 0 {
			line += locales.RptMsgPdfScoreLabel + strconv.Itoa(rep.Score)
		}
		if rep.Level != "" {
			if line != "" {
				line += "  ·  "
			}
			line += rep.Level
		}
		w.paragraph(line)
	}

	// Параметры тела.
	if rep.Body.Height != "" || rep.Body.Weight != "" || rep.Body.MuscleMass != "" || rep.Body.Fat != "" {
		w.heading(locales.RptMsgPdfBodyParams)
		parts := []string{}
		if rep.Body.Height != "" {
			parts = append(parts, locales.RptMsgPdfHeightLabel+rep.Body.Height)
		}
		if rep.Body.Weight != "" {
			parts = append(parts, locales.RptMsgPdfWeightLabel+rep.Body.Weight)
		}
		if rep.Body.MuscleMass != "" {
			parts = append(parts, locales.RptMsgPdfMuscleMassLabel+rep.Body.MuscleMass)
		}
		if rep.Body.Fat != "" {
			parts = append(parts, locales.RptMsgPdfBodyFatLabel+rep.Body.Fat)
		}
		w.paragraph(joinParts(parts, "   "))
	}

	// Композиция.
	if rep.Composition != "" {
		w.heading(locales.RptMsgPdfComposition)
		w.paragraph(rep.Composition)
	}

	// Профиль развития (диаграммы).
	if rep.Profile.Composition > 0 || rep.Profile.MuscleDevelopment > 0 ||
		rep.Profile.Balance > 0 || rep.Profile.Potential > 0 {
		w.heading(locales.RptMsgPdfProfileDev)
		w.barRow(locales.RptMsgPdfCompositionLabel, rep.Profile.Composition)
		w.barRow(locales.RptMsgPdfMuscleDevLabel, rep.Profile.MuscleDevelopment)
		w.barRow(locales.RptMsgPdfBalanceLabel, rep.Profile.Balance)
		w.barRow(locales.RptMsgPdfPotentialLabel, rep.Profile.Potential)
		w.pdf.Ln(2)
	}

	// Резюме.
	if rep.Summary != "" {
		w.heading(locales.RptMsgPdfSummary)
		w.paragraph(rep.Summary)
	}

	// Зоны.
	if len(rep.Zones) > 0 {
		w.heading(locales.RptMsgPdfZoneAssessment)
		for _, z := range rep.Zones {
			w.barRow(truncate(z.Name, 40), z.Score)
			if z.Status != "" {
				w.paragraph(locales.RptMsgPdfStatusLabel + z.Status)
			}
			if z.Description != "" {
				w.paragraph(z.Description)
			}
			if z.Recommendation != "" {
				w.paragraph(locales.RptMsgPdfRecLabel + z.Recommendation)
			}
			w.pdf.Ln(1)
		}
	}

	// Мышцы.
	if len(rep.Muscles) > 0 {
		w.heading(locales.RptMsgPdfMuscleGroups)
		for _, m := range rep.Muscles {
			s := "• " + m.Name
			if m.Level != "" {
				s += " - " + m.Level
			}
			w.paragraph(s)
			if m.Assessment != "" {
				w.paragraph(locales.RptMsgPdfAssessmentLabel + m.Assessment)
			}
			if m.Symmetry != "" {
				w.paragraph(locales.RptMsgPdfSymmetryLabel + m.Symmetry)
			}
			if m.Recommendation != "" {
				w.paragraph(locales.RptMsgPdfRecLabel + m.Recommendation)
			}
		}
	}

	// Осанка.
	if rep.Posture.Type != "" || rep.Posture.Description != "" {
		w.heading(locales.RptMsgPdfPosture)
		if rep.Posture.Type != "" {
			w.paragraph(locales.RptMsgPdfTypeLabel + rep.Posture.Type)
		}
		if rep.Posture.Head != "" {
			w.paragraph(locales.RptMsgPdfHeadLabel + rep.Posture.Head)
		}
		if rep.Posture.Shoulders != "" {
			w.paragraph(locales.RptMsgPdfShouldersLabel + rep.Posture.Shoulders)
		}
		if rep.Posture.Pelvis != "" {
			w.paragraph(locales.RptMsgPdfPelvisLabel + rep.Posture.Pelvis)
		}
		if rep.Posture.Description != "" {
			w.paragraph(rep.Posture.Description)
		}
	}

	// Зоны внимания.
	if len(rep.AttentionZones) > 0 {
		w.heading(locales.RptMsgPdfAttentionZones)
		for _, a := range rep.AttentionZones {
			w.paragraph("• " + a.Name)
			if a.Problem != "" {
				w.paragraph(locales.RptMsgPdfProblemLabel + a.Problem)
			}
			if a.Solution != "" {
				w.paragraph(locales.RptMsgPdfSolutionLabel + a.Solution)
			}
		}
	}

	// Приоритеты.
	if len(rep.Priorities) > 0 {
		w.heading(locales.RptMsgPdfPriorities)
		for _, p := range rep.Priorities {
			s := "• " + p.Title
			if p.Description != "" {
				s += "\n  " + p.Description
			}
			w.paragraph(s)
		}
	}

	// Тренировки.
	if len(rep.TrainingDays) > 0 {
		w.heading(locales.RptMsgPdfTrainingProgram)
		for _, d := range rep.TrainingDays {
			w.paragraph("• " + d.Day)
			for _, ex := range d.Exercises {
				reps := ex.Reps
				if reps == "" {
					reps = ex.Sets
				} else if ex.Sets != "" {
					reps = ex.Sets + " × " + ex.Reps
				}
				w.paragraph("    - " + ex.Name + ": " + reps)
			}
		}
	}

	// Питание.
	if len(rep.Nutrition) > 0 {
		w.heading(locales.RptMsgPdfNutrition)
		for _, n := range rep.Nutrition {
			w.bullet(n)
		}
	}

	// Восстановление.
	if len(rep.Recovery) > 0 {
		w.heading(locales.RptMsgPdfRecovery)
		for _, n := range rep.Recovery {
			w.bullet(n)
		}
	}

	// Прогресс.
	if rep.Progress.Recheck != "" || len(rep.Progress.Targets) > 0 {
		w.heading(locales.RptMsgPdfProgressControl)
		if rep.Progress.Recheck != "" {
			w.paragraph(locales.RptMsgPdfRecheckLabel + rep.Progress.Recheck)
		}
		for _, t := range rep.Progress.Targets {
			w.bullet(t)
		}
	}

	// Дисклеймер.
	w.pdf.Ln(2)
	w.pdf.SetFont("Arial", "", 8)
	w.pdf.SetTextColor(120, 120, 120)
	w.pdf.MultiCell(pdfPageW-2*pdfMargin, 4,
		locales.RptMsgPdfDisclaimer, "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pdfWriter - обёртка над gofpdf с удобными помощниками вёрстки.
type pdfWriter struct {
	pdf *gofpdf.Fpdf
}

func (w *pdfWriter) ensureSpace(h float64) {
	if w.pdf.GetY()+h > pdfPageH-pdfMargin {
		w.pdf.AddPage()
	}
}

func (w *pdfWriter) heading(s string) {
	w.ensureSpace(10)
	w.pdf.SetFont("Arial", "", 13)
	w.pdf.SetTextColor(33, 70, 110)
	w.pdf.MultiCell(pdfPageW-2*pdfMargin, 7, s, "", "L", false)
	w.pdf.Ln(1)
}

func (w *pdfWriter) paragraph(s string) {
	if s == "" {
		return
	}
	w.ensureSpace(8)
	w.pdf.SetFont("Arial", "", 10)
	w.pdf.SetTextColor(30, 30, 30)
	w.pdf.MultiCell(pdfPageW-2*pdfMargin, 5, s, "", "L", false)
	w.pdf.Ln(1)
}

func (w *pdfWriter) bullet(s string) {
	if s == "" {
		return
	}
	w.ensureSpace(8)
	w.pdf.SetFont("Arial", "", 10)
	w.pdf.SetTextColor(30, 30, 30)
	w.pdf.SetX(pdfMargin)
	w.pdf.Cell(5, 5, "•")
	w.pdf.MultiCell(pdfPageW-2*pdfMargin-5, 5, s, "", "L", false)
}

// barRow рисует подпись и цветной столбик-диаграмму для оценки (0..100).
func (w *pdfWriter) barRow(label string, score int) {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	w.ensureSpace(12)
	y := w.pdf.GetY()
	w.pdf.SetFont("Arial", "", 10)
	w.pdf.SetTextColor(30, 30, 30)
	w.pdf.Text(pdfMargin, y+4, truncate(label, 38))

	x0 := pdfMargin + 48
	barMax := pdfPageW - pdfMargin - x0 - 10
	if barMax < 10 {
		barMax = 10
	}
	w.pdf.SetFillColor(225, 225, 225)
	w.pdf.Rect(x0, y+1, barMax, 5, "F")
	c := scoreColor(score)
	w.pdf.SetFillColor(c[0], c[1], c[2])
	w.pdf.Rect(x0, y+1, barMax*float64(score)/100.0, 5, "F")
	w.pdf.SetTextColor(0, 0, 0)
	w.pdf.Text(x0+barMax+1, y+4, strconv.Itoa(score))

	w.pdf.SetXY(pdfMargin, y+8)
}

// scoreColor возвращает цвет столбика по величине оценки.
func scoreColor(score int) [3]int {
	switch {
	case score >= 80:
		return [3]int{76, 175, 80} // зелёный
	case score >= 60:
		return [3]int{255, 152, 0} // оранжевый
	default:
		return [3]int{244, 67, 54} // красный
	}
}

func joinParts(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// truncate обрезает строку до max рун, добавляя «…» при необходимости.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
