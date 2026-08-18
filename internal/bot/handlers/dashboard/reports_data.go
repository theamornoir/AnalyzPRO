package dashboard

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/theamornoir/analyzpro/internal/monitoring"
)

// ReportsResponse - ответ API /api/reports: последний отчёт
// расширенного анализа и Bioscan PRO, сохранённые в истории пользователя.
type ReportsResponse struct {
	PremiumRequired bool         `json:"premiumRequired"`
	Analysis        ReportsGroup `json:"analysis"`
	Bioscan         ReportsGroup `json:"bioscan"`
}

// ReportsGroup - группа отчётов одного типа (analysis / bioscan).
type ReportsGroup struct {
	Reports []ReportBlock `json:"reports"`
	Latest  ReportBlock   `json:"latest"`
	Count   int           `json:"count"`
}

// ReportBlock - нормализованное представление одного отчёта для графиков
// дашборда: индекс, набор имён-оценок (для radar/bar), зоны (для биоскана),
// индикаторы (для списка).
type ReportBlock struct {
	// ID - идентификатор записи в истории; фронтенд запрашивает этот
	// конкретный отчёт как PDF через /api/reports/file.
	ID         int64           `json:"id"`
	Available  bool            `json:"available"`
	Title      string          `json:"title"`
	Date       string          `json:"date"`
	MainScore  int             `json:"mainScore"`
	ScoreLabel string          `json:"scoreLabel"`
	Scores     map[string]int  `json:"scores"`
	Indicators []IndicatorView `json:"indicators"`
	Zones      []ZoneView      `json:"zones"`
	Summary    string          `json:"summary"`
}

// IndicatorView - один показатель списка (имя/значение/статус).
// Поля Num/RefMin/RefMax/Normal нужны фронтенду, чтобы рисовать
// вертикальный индикатор с зонами (норма/внимание/критично) и ставить
// метку текущего значения. Если референс неизвестен (RefMax == 0) -
// фронт рисует только цифру и текстовый статус, без шкалы.
type IndicatorView struct {
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	Status string  `json:"status"`
	Normal string  `json:"normal"`
	Num    float64 `json:"num"`
	RefMin float64 `json:"refMin"`
	RefMax float64 `json:"refMax"`
}

// ZoneView - одна зона тела биоскана (для круговых диаграмм).
type ZoneView struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Status  string `json:"status"`
	Comment string `json:"comment"`
}

// buildReportsData собирает последний+предыдущий отчёты analysis и bioscan
// из истории пользователя и вычисляет дельту индекса.
func (h *Handler) buildReportsData(ctx context.Context, telegramID int64) ReportsResponse {
	resp := ReportsResponse{}
	resp.Analysis = h.buildGroup(ctx, telegramID, "analysis")
	resp.Bioscan = h.buildGroup(ctx, telegramID, "bioscan")
	return resp
}

func (h *Handler) buildGroup(ctx context.Context, telegramID int64, entryType string) ReportsGroup {
	// Берём ВСЕ отчёты этого типа (pageSize=0 → без пагинации), чтобы
	// собрать полный архив для «Мой профиль» (история прогресса).
	entries, total, err := h.repo.ListHistory(ctx, telegramID, entryType, 0, 0)
	g := ReportsGroup{Count: total, Reports: []ReportBlock{}}
	if err != nil || len(entries) == 0 {
		return g
	}

	for _, e := range entries {
		g.Reports = append(g.Reports, parseReportBlock(e, entryType))
	}
	g.Latest = g.Reports[0]
	return g
}

// parseReportBlock толерантно парсит JSON отчёта обеих схем (analysis:
// sections/categories/profile; bioscan: BodyScanReport) и нормализует его
// в ReportBlock для графиков. На ошибках/пустоте возвращает пустой блок
// (Available=false) - дашборд корректно покажет «нет данных».
func parseReportBlock(e monitoring.HistoryEntry, entryType string) ReportBlock {
	jsonStr := e.JsonData
	dateStr := e.Date.Format("2006-01-02")
	out := ReportBlock{
		ID:        e.ID,
		Available: false,
		Scores:    map[string]int{},
	}
	if strings.TrimSpace(jsonStr) == "" {
		return out
	}
	out.Available = true
	out.Date = dateStr

	if entryType == "bioscan" {
		parseBioscanBlock(jsonStr, &out)
	} else {
		parseAnalysisBlock(jsonStr, &out)
	}
	return out
}

// indicatorShape - общая форма блока indicators (для analysis).
type indicatorShape struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Normal string `json:"normal"`
	Score  int    `json:"score"`
}

// parseAnalysisBlock парсит расширенный анализ (sections/categories/profile).
func parseAnalysisBlock(jsonStr string, out *ReportBlock) {
	var doc struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Profile struct {
			Composition       int `json:"composition"`
			MuscleDevelopment int `json:"muscle_development"`
			Balance           int `json:"balance"`
			Potential         int `json:"potential"`
			Score             int `json:"score"`
		} `json:"profile"`
		Categories []struct {
			Name       string           `json:"name"`
			Indicators []indicatorShape `json:"indicators"`
		} `json:"categories"`
		Sections []struct {
			Title      string           `json:"title"`
			Indicators []indicatorShape `json:"indicators"`
		} `json:"sections"`
		LabSystems []struct {
			Title      string           `json:"title"`
			Indicators []indicatorShape `json:"indicators"`
		} `json:"lab_systems"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return
	}

	out.Title = strings.TrimSpace(doc.Title)
	out.Summary = strings.TrimSpace(doc.Summary)

	if doc.Profile.Composition > 0 {
		out.Scores["Композиция"] = doc.Profile.Composition
	}
	if doc.Profile.MuscleDevelopment > 0 {
		out.Scores["Мышцы"] = doc.Profile.MuscleDevelopment
	}
	if doc.Profile.Balance > 0 {
		out.Scores["Баланс"] = doc.Profile.Balance
	}
	if doc.Profile.Potential > 0 {
		out.Scores["Потенциал"] = doc.Profile.Potential
	}

	scoreSum, scoreN := 0, 0
	addIndicators := func(indicators []indicatorShape) {
		for _, ind := range indicators {
			if ind.Name == "" {
				continue
			}
			num, refMin, refMax := parseIndicatorRef(ind.Value, ind.Normal)
			out.Indicators = append(out.Indicators, IndicatorView{
				Name:   ind.Name,
				Value:  ind.Value,
				Status: ind.Status,
				Normal: ind.Normal,
				Num:    num,
				RefMin: refMin,
				RefMax: refMax,
			})
			if ind.Score > 0 {
				if _, ok := out.Scores[ind.Name]; !ok {
					out.Scores[ind.Name] = ind.Score
				}
				scoreSum += ind.Score
				scoreN++
			}
		}
	}
	for _, c := range doc.Categories {
		addIndicators(c.Indicators)
	}
	for _, s := range doc.Sections {
		addIndicators(s.Indicators)
	}
	// Досье хранит лабораторные показатели в lab_systems (а не в
	// categories/sections) - подхватываем их, чтобы в карточке отчёта
	// тоже рисовался вертикальный индикатор с зонами.
	for _, ls := range doc.LabSystems {
		addIndicators(ls.Indicators)
	}

	switch {
	case doc.Profile.Score > 0:
		out.MainScore = doc.Profile.Score
		out.ScoreLabel = "Индекс здоровья"
	case doc.Profile.Composition > 0:
		out.MainScore = doc.Profile.Composition
		out.ScoreLabel = "Композиция"
	case doc.Profile.Potential > 0:
		out.MainScore = doc.Profile.Potential
		out.ScoreLabel = "Потенциал"
	case scoreN > 0:
		out.MainScore = scoreSum / scoreN
		out.ScoreLabel = "Индекс (среднее)"
	default:
		out.MainScore = 0
		out.ScoreLabel = "Индекс"
	}

	// Если не нашли ни sections/categories, ни profile - возможно, это
	// досье здоровья (HealthDossier) с картой scores (Health/Potential/...).
	// Используем его для radar и индекса (применяется для файлового
	// расширенного анализа, который сохраняется как HealthDossier).
	if len(out.Scores) == 0 {
		var dossier struct {
			Scores    map[string]int `json:"scores"`
			Synthesis string         `json:"synthesis"`
		}
		if json.Unmarshal([]byte(jsonStr), &dossier) == nil && len(dossier.Scores) > 0 {
			out.Scores = dossier.Scores
			out.MainScore = dossier.Scores["Health"]
			out.ScoreLabel = "Индекс здоровья"
			if out.Summary == "" {
				out.Summary = strings.TrimSpace(dossier.Synthesis)
			}
		}
	}

	if out.Summary == "" && len(out.Indicators) > 0 {
		out.Summary = "Показатели загружены из отчёта. Подробности - в PDF-документе."
	}

}

// parseBioscanBlock парсит Bioscan PRO (BodyScanReport).
func parseBioscanBlock(jsonStr string, out *ReportBlock) {
	var doc struct {
		Title   string `json:"title"`
		Score   int    `json:"score"`
		Level   string `json:"level"`
		Summary string `json:"summary"`
		Posture struct {
			PostureScore    int `json:"posture_score"`
			Symmetry        int `json:"symmetry"`
			ShoulderBalance int `json:"shoulder_balance"`
			PelvicBalance   int `json:"pelvic_balance"`
			SpinalAlignment int `json:"spinal_alignment"`
			Mobility        int `json:"mobility"`
			Stability       int `json:"stability"`
		} `json:"posture"`
		Zones []struct {
			Name    string `json:"name"`
			Score   int    `json:"score"`
			Status  string `json:"status"`
			Comment string `json:"comment"`
		} `json:"zones"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return
	}

	out.Title = strings.TrimSpace(doc.Title)
	out.Summary = strings.TrimSpace(doc.Summary)
	out.MainScore = doc.Score
	out.ScoreLabel = "Body Score"

	if doc.Posture.PostureScore > 0 {
		out.Scores["Осанка"] = doc.Posture.PostureScore
	}
	if doc.Posture.Symmetry > 0 {
		out.Scores["Симметрия"] = doc.Posture.Symmetry
	}
	if doc.Posture.ShoulderBalance > 0 {
		out.Scores["Плечи"] = doc.Posture.ShoulderBalance
	}
	if doc.Posture.PelvicBalance > 0 {
		out.Scores["Таз"] = doc.Posture.PelvicBalance
	}
	if doc.Posture.SpinalAlignment > 0 {
		out.Scores["Позвоночник"] = doc.Posture.SpinalAlignment
	}
	if doc.Posture.Mobility > 0 {
		out.Scores["Мобильность"] = doc.Posture.Mobility
	}
	if doc.Posture.Stability > 0 {
		out.Scores["Стабильность"] = doc.Posture.Stability
	}

	for _, z := range doc.Zones {
		if z.Name == "" {
			continue
		}
		out.Zones = append(out.Zones, ZoneView{
			Name:    z.Name,
			Score:   z.Score,
			Status:  z.Status,
			Comment: z.Comment,
		})
	}

	if out.Title == "" {
		out.Title = "Bioscan PRO"
	}

}

// parseIndicatorRef извлекает числовое значение показателя и границы
// референсного интервала (нормы) из строк. Возвращает (num, refMin, refMax);
// если норма не распознана - refMin/refMax = 0 (фронт тогда не дёргает
// шкалу и показывает только цифру со статусом).
func parseIndicatorRef(value, normal string) (num, refMin, refMax float64) {
	num = leadingFloat(value)
	if rmin, rmax, ok := parseRefRange(normal); ok {
		refMin = rmin
		refMax = rmax
	}
	return
}

// leadingFloat извлекает первое вещественное число из строки
// ("255.00 Ед/мл" -> 255.0, "5.2 ммоль/л" -> 5.2, "-3.1" -> -3.1).
func leadingFloat(s string) float64 {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
	m := re.FindString(s)
	if m == "" {
		return 0
	}
	m = strings.ReplaceAll(m, ",", ".")
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseRefRange распознаёт референсный интервал из строки вида
// "0-100", "0 - 100", "0.00 - 100.00 Ед/мл", "(3.9-6.1)", "менее 100".
// Возвращает (min, max, ok). Для "более X" верхняя граница аппроксимируется
// как X*2 (чтобы маркер уходил в красную зону, но шкала оставалась конечной).
func parseRefRange(s string) (min, max float64, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	s = strings.Trim(s, "()[]")
	nums := regexp.MustCompile(`\d+(?:[.,]\d+)?`).FindAllString(s, -1)
	if len(nums) >= 2 {
		a, err1 := strconv.ParseFloat(strings.ReplaceAll(nums[0], ",", "."), 64)
		b, err2 := strconv.ParseFloat(strings.ReplaceAll(nums[1], ",", "."), 64)
		if err1 == nil && err2 == nil && b >= a {
			return a, b, true
		}
	}
	// "менее X" / "до X" / "< X" -> (0, X)
	if m := regexp.MustCompile(`(?:менее|до|не более|<)\s*(\d+(?:[.,]\d+)?)`).FindStringSubmatch(s); m != nil {
		if x, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
			return 0, x, true
		}
	}
	// "более X" / "> X" -> (X, X*2)
	if m := regexp.MustCompile(`(?:более|свыше|больше|>)\s*(\d+(?:[.,]\d+)?)`).FindStringSubmatch(s); m != nil {
		if x, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
			return x, math.Max(x*2, x+1), true
		}
	}
	return 0, 0, false
}
