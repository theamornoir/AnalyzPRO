package handlers

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// GenerateBioscanPDF - создаёт профессиональный PDF с результатами Bioscan
func GenerateBioscanPDF(
	text string,
	userName string,
	age string,
	gender string,
	height string,
	weight string,
	sportType string,
	goal string,
) ([]byte, error) {
	// Очищаем текст от эмодзи и Markdown
	cleanText := cleanTextForPDF(text)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 25)
	pdf.AddPage()

	// ==========================================
	// ИСПОЛЬЗУЕМ ВСТРОЕННЫЙ ШРИФТ FREESERIF
	// ==========================================
	// freeserif поддерживает кириллицу
	pdf.SetFont("freeserif", "", 11)

	// ==========================================
	// ЦВЕТОВАЯ СХЕМА
	// ==========================================
	primaryColor := [3]int{41, 128, 185}
	secondaryColor := [3]int{52, 73, 94}
	accentColor := [3]int{231, 76, 60}
	lightGray := [3]int{236, 240, 241}

	// ==========================================
	// ШАПКА (HEADER)
	// ==========================================
	pdf.SetFillColor(primaryColor[0], primaryColor[1], primaryColor[2])
	pdf.Rect(0, 0, 210, 45, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("freeserif", "B", 22)
	pdf.Text(20, 22, "BIOSCAN")
	pdf.SetFont("freeserif", "", 13)
	pdf.Text(20, 32, "Professional Body Analysis Report")

	pdf.SetFont("freeserif", "", 10)
	pdf.Text(155, 12, "Date: "+time.Now().Format("02.01.2006"))
	pdf.Text(155, 18, "Time: "+time.Now().Format("15:04"))

	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(155, 24, fmt.Sprintf("Report #: %s", time.Now().Format("200601021504")))

	pdf.SetDrawColor(255, 255, 255)
	pdf.SetLineWidth(0.5)
	pdf.Line(20, 45, 190, 45)

	// ==========================================
	// ИНФОРМАЦИЯ О ПАЦИЕНТЕ
	// ==========================================
	y := 55.0

	pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
	pdf.Rect(20, y-2, 170, 38, "F")

	pdf.SetTextColor(secondaryColor[0], secondaryColor[1], secondaryColor[2])
	pdf.SetFont("freeserif", "B", 11)
	pdf.Text(25, y+5, "PATIENT INFORMATION")

	pdf.SetFont("freeserif", "", 10)
	pdf.SetTextColor(0, 0, 0)

	pdf.Text(25, y+15, "Patient:")
	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(55, y+15, userName)

	pdf.SetFont("freeserif", "", 10)
	pdf.Text(120, y+15, "Gender:")
	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(145, y+15, gender)

	pdf.SetFont("freeserif", "", 10)
	pdf.Text(25, y+23, "Age:")
	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(55, y+23, age+" years")

	pdf.SetFont("freeserif", "", 10)
	pdf.Text(120, y+23, "Height:")
	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(145, y+23, height+" cm")

	pdf.SetFont("freeserif", "", 10)
	pdf.Text(25, y+31, "Weight:")
	pdf.SetFont("freeserif", "B", 10)
	pdf.Text(55, y+31, weight+" kg")

	if height != "—" && weight != "—" {
		h, err1 := strconv.ParseFloat(height, 64)
		w, err2 := strconv.ParseFloat(weight, 64)
		if err1 == nil && err2 == nil && h > 0 && w > 0 {
			bmi := w / ((h / 100) * (h / 100))
			pdf.SetFont("freeserif", "", 10)
			pdf.Text(120, y+31, "BMI:")
			pdf.SetFont("freeserif", "B", 10)
			pdf.Text(145, y+31, fmt.Sprintf("%.1f", bmi))
		}
	}

	if sportType != "" || goal != "" {
		y += 38
		pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
		pdf.Rect(20, y-2, 170, 18, "F")

		if sportType != "" {
			pdf.SetFont("freeserif", "", 10)
			pdf.Text(25, y+7, "Sport:")
			pdf.SetFont("freeserif", "B", 10)
			pdf.Text(60, y+7, sportType)
		}
		if goal != "" {
			pdf.SetFont("freeserif", "", 10)
			pdf.Text(25, y+15, "Goal:")
			pdf.SetFont("freeserif", "B", 10)
			pdf.Text(60, y+15, goal)
		}
		y += 20
	} else {
		y += 38
	}

	// ==========================================
	// ОСНОВНОЙ ТЕКСТ
	// ==========================================
	pdf.SetFillColor(primaryColor[0], primaryColor[1], primaryColor[2])
	pdf.Rect(20, y-2, 8, 8, "F")
	pdf.SetTextColor(secondaryColor[0], secondaryColor[1], secondaryColor[2])
	pdf.SetFont("freeserif", "B", 14)
	pdf.Text(30, y+4, "BODY ASSESSMENT")

	y += 12
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("freeserif", "", 11)

	// Разбиваем текст на строки и вставляем
	lines := strings.Split(cleanText, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Проверяем заголовки
		isHeader := strings.Contains(line, "ГРУДНЫЕ") ||
			strings.Contains(line, "ДЕЛЬТОВИДНЫЕ") ||
			strings.Contains(line, "ТРАПЕЦИЯ") ||
			strings.Contains(line, "ШИРОЧАЙШАЯ") ||
			strings.Contains(line, "БИЦЕПС") ||
			strings.Contains(line, "ТРИЦЕПС") ||
			strings.Contains(line, "ПРЯМАЯ МЫШЦА") ||
			strings.Contains(line, "КОСЫЕ") ||
			strings.Contains(line, "КВАДРИЦЕПС") ||
			strings.Contains(line, "БИЦЕПС БЕДРА") ||
			strings.Contains(line, "ИКРОНОЖНЫЕ") ||
			strings.Contains(line, "ЯГОДИЦЫ") ||
			strings.Contains(line, "ПОЯСНИЧНЫЙ") ||
			strings.Contains(line, "ОБЩАЯ ОЦЕНКА") ||
			strings.Contains(line, "ДЕТАЛЬНЫЙ АНАЛИЗ") ||
			strings.Contains(line, "ВЕРХНЯЯ ЧАСТЬ") ||
			strings.Contains(line, "СРЕДНЯЯ ЧАСТЬ") ||
			strings.Contains(line, "НИЖНЯЯ ЧАСТЬ") ||
			strings.Contains(line, "ОСАНКА") ||
			strings.Contains(line, "РЕКОМЕНДАЦИИ") ||
			strings.Contains(line, "ПРОГРЕСС-ТРЕК") ||
			strings.Contains(line, "Тип фигуры") ||
			strings.Contains(line, "Пропорции") ||
			strings.Contains(line, "Общая оценка")

		if isHeader {
			pdf.SetFillColor(lightGray[0], lightGray[1], lightGray[2])
			pdf.Rect(20, pdf.GetY()-1, 170, 8, "F")
			pdf.SetFont("freeserif", "B", 11)
			pdf.SetTextColor(secondaryColor[0], secondaryColor[1], secondaryColor[2])
			pdf.Cell(0, 8, line)
			pdf.SetTextColor(0, 0, 0)
			pdf.Ln(8)
			continue
		}

		// Рекомендации - выделяем красным
		if strings.Contains(line, "Рекомендация:") {
			pdf.SetFont("freeserif", "B", 10)
			pdf.SetTextColor(accentColor[0], accentColor[1], accentColor[2])
			pdf.MultiCell(170, 5, line, "", "", false)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("freeserif", "", 11)
			pdf.Ln(2)
			continue
		}

		// Состояние, симметрия, оценка - курсивом
		if strings.Contains(line, "Состояние:") ||
			strings.Contains(line, "Симметрия:") ||
			strings.Contains(line, "Оценка:") {
			pdf.SetFont("freeserif", "I", 10)
			pdf.SetTextColor(secondaryColor[0], secondaryColor[1], secondaryColor[2])
			pdf.MultiCell(170, 5, line, "", "", false)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("freeserif", "", 11)
			continue
		}

		// Обычный текст
		pdf.SetFont("freeserif", "", 11)
		pdf.MultiCell(170, 6, line, "", "", false)
		pdf.Ln(1)
	}

	// ==========================================
	// ФУТЕР
	// ==========================================
	pdf.SetY(-30)

	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.5)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())

	pdf.Ln(5)
	pdf.SetFont("freeserif", "I", 9)
	pdf.SetTextColor(128, 128, 128)
	pdf.Cell(0, 5, "Analysis performed by AnalyzPRO")
	pdf.Ln(5)
	pdf.Cell(0, 5, "This report is for informational purposes only. Consult a professional for medical advice.")
	pdf.Ln(5)

	pdf.SetFont("freeserif", "I", 8)
	pdf.Text(185, pdf.GetY()-2, fmt.Sprintf("Page %d", pdf.PageNo()))

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// cleanTextForPDF - полностью очищает текст от эмодзи и Markdown
func cleanTextForPDF(text string) string {
	// Убираем Markdown
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, "#", "")
	text = strings.ReplaceAll(text, ">", "")
	text = strings.ReplaceAll(text, "-", " ")
	text = strings.ReplaceAll(text, "•", "  ")

	// Убираем эмодзи
	var result strings.Builder
	for _, r := range text {
		// Пропускаем эмодзи и спецсимволы
		if r >= 0x1F000 && r <= 0x1FFFF {
			continue
		}
		if r >= 0x2600 && r <= 0x27BF {
			continue
		}
		if r >= 0xFE00 && r <= 0xFE0F {
			continue
		}
		if r >= 0x1F300 && r <= 0x1F5FF {
			continue
		}
		if r >= 0x1F600 && r <= 0x1F64F {
			continue
		}
		if r >= 0x1F680 && r <= 0x1F6FF {
			continue
		}
		if r >= 0x1F700 && r <= 0x1F77F {
			continue
		}
		if r >= 0x1F780 && r <= 0x1F7FF {
			continue
		}
		if r >= 0x1F800 && r <= 0x1F8FF {
			continue
		}
		if r >= 0x1F900 && r <= 0x1F9FF {
			continue
		}
		if r >= 0x1FA00 && r <= 0x1FA6F {
			continue
		}
		if r >= 0x1FA70 && r <= 0x1FAFF {
			continue
		}
		result.WriteRune(r)
	}

	// Убираем лишние пробелы
	text = strings.Join(strings.Fields(result.String()), " ")
	return strings.TrimSpace(text)
}
