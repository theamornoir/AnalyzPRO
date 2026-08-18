package dashboard

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/monitoring"
)

// ReportsResponse - ответ API /api/reports: архив отчётов расширенного
// анализа и Bioscan PRO, сохранённые в истории пользователя.
//
// Модель Free/Premium: для не-Premium пользователей архив ограничен
// 3 последними записями (лимит применяется здесь, на backend, а не
// скрытием на фронте - чтобы Free не мог получить старые записи
// прямым запросом к API). Тренд-бейджи считаются по ПОЛНОЙ истории, но
// раскрывают только направление изменения, без значений скрытых записей.
type ReportsResponse struct {
	PremiumRequired bool         `json:"premiumRequired"`
	IsPremium       bool         `json:"isPremium"`
	TrendBadges     []TrendBadge `json:"trendBadges"`
	Analysis        ReportsGroup `json:"analysis"`
	Bioscan         ReportsGroup `json:"bioscan"`
}

// TrendBadge - компактный бейдж направления тренда показателя,
// вычисленный по полной истории пользователя. Раскрывает ТОЛЬКО
// направление (стрелка/слово) и период - не сами значения записей,
// поэтому безопасен для показа Free-пользователю даже по скрытым записям.
type TrendBadge struct {
	Indicator string `json:"indicator"`
	Arrow     string `json:"arrow"`
	Direction string `json:"direction"`
	Period    string `json:"period"`
}

// ReportsGroup - группа отчётов одного типа (analysis / bioscan).
type ReportsGroup struct {
	Reports     []ReportBlock `json:"reports"`
	Latest      ReportBlock   `json:"latest"`
	Count       int           `json:"count"`
	TotalCount  int           `json:"totalCount"`
	HiddenCount int           `json:"hiddenCount"`
}

// ReportBlock - нормализованное представление одного отчёта для графиков
// дашборда: индекс, набор имён-оценок (для radar/bar), зоны (для биоскана),
// индикаторы (для списка).
type ReportBlock struct {
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

type trendPoint struct {
	Date time.Time
	Num  float64
}

// buildReportsData собирает архив analysis и bioscan из истории
// пользователя, применяет лимит истории для Free (3 последние записи),
// вычисляет тренд-бейджи по полной истории и дельту индекса.
func (h *Handler) buildReportsData(ctx context.Context, telegramID int64, isPremium bool) ReportsResponse {
	resp := ReportsResponse{
		IsPremium:   isPremium,
		TrendBadges: []TrendBadge{},
	}
	resp.Analysis = h.buildGroup(ctx, telegramID, "analysis", isPremium)
	resp.Bioscan = h.buildGroup(ctx, telegramID, "bioscan", isPremium)
	resp.TrendBadges = computeTrendBadges(ctx, h.repo, telegramID)
	return resp
}

// freeHistoryLimit - сколько последних записей показываем Free-пользователю.
const freeHistoryLimit = 3

func (h *Handler) buildGroup(ctx context.Context, telegramID int64, entryType string, isPremium bool) ReportsGroup {
	entries, total, err := h.repo.ListHistory(ctx, telegramID, entryType, 0, 0)
	g := ReportsGroup{Count: total, TotalCount: total, HiddenCount: 0, Reports: []ReportBlock{}}
	if err != nil || len(entries) == 0 {
		return g
	}

	reports := make([]ReportBlock, 0, len(entries))
	for _, e := range entries {
		reports = append(reports, parseReportBlock(e, entryType))
	}

	if !isPremium && len(reports) > freeHistoryLimit {
		reports = reports[:freeHistoryLimit]
	}
	g.HiddenCount = total - len(reports)
	g.Reports = reports
	if len(g.Reports) > 0 {
		g.Latest = g.Reports[0]
	}
	return g
}

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

type indicatorShape struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Normal string `json:"normal"`
	Score  int    `json:"score"`
}

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

func parseIndicatorRef(value, normal string) (num, refMin, refMax float64) {
	num = leadingFloat(value)
	if rmin, rmax, ok := parseRefRange(normal); ok {
		refMin = rmin
		refMax = rmax
	}
	return
}

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
	if m := regexp.MustCompile(`(?:менее|до|не более|<)\s*(\d+(?:[.,]\d+)?)`).FindStringSubmatch(s); m != nil {
		if x, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
			return 0, x, true
		}
	}
	if m := regexp.MustCompile(`(?:более|свыше|больше|>)\s*(\d+(?:[.,]\d+)?)`).FindStringSubmatch(s); m != nil {
		if x, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
			return x, math.Max(x*2, x+1), true
		}
	}
	return 0, 0, false
}

var priorityBiomarkers = []string{
	"Глюкоза", "glucose",
	"Ферритин", "ferritin",
	"АЛТ", "ALT", "алт",
	"АСТ", "AST", "аст",
	"Холестерин", "cholesterol",
	"ЛПВП", "HDL",
	"ЛПНП", "LDL",
	"Триглицериды", "triglycerides",
	"ТТГ", "TSH",
	"Креатинин", "creatinine",
	"Гемоглобин", "hemoglobin",
	"Мочевина", "urea",
	"Билирубин", "bilirubin",
	"Витамин D", "vitamin d",
}

func computeTrendBadges(ctx context.Context, repo monitoring.Repository, telegramID int64) []TrendBadge {
	entries, _, err := repo.ListHistory(ctx, telegramID, "", 0, 0)
	if err != nil || len(entries) == 0 {
		return []TrendBadge{}
	}

	series := map[string][]trendPoint{}
	for _, e := range entries {
		block := parseReportBlock(e, e.Type)
		for _, ind := range block.Indicators {
			if ind.Num == 0 {
				continue
			}
			name := strings.TrimSpace(ind.Name)
			if name == "" {
				continue
			}
			series[name] = append(series[name], trendPoint{Date: e.Date, Num: ind.Num})
		}
	}
	if len(series) == 0 {
		return []TrendBadge{}
	}

	type candidate struct {
		badge TrendBadge
		count int
	}
	candidates := []candidate{}

	for name, pts := range series {
		if len(pts) < 2 {
			continue
		}
		sort.SliceStable(pts, func(i, j int) bool {
			return pts[i].Date.Before(pts[j].Date)
		})
		first := pts[0]
		last := pts[len(pts)-1]

		delta := last.Num - first.Num
		rel := delta / (math.Abs(first.Num) + 1e-9)
		var dir, arrow string
		switch {
		case math.Abs(rel) < 0.1:
			dir, arrow = "stable", "→"
		case delta > 0:
			dir, arrow = "up", "↑"
		default:
			dir, arrow = "down", "↓"
		}
		months := int(math.Round(last.Date.Sub(first.Date).Hours() / 24 / 30.44))
		if months < 1 {
			months = 1
		}

		candidates = append(candidates, candidate{
			badge: TrendBadge{
				Indicator: name,
				Arrow:     arrow,
				Direction: dir,
				Period:    monthsPhrase(months),
			},
			count: len(pts),
		})
	}
	if len(candidates) == 0 {
		return []TrendBadge{}
	}

	priorityRank := func(name string) int {
		low := strings.ToLower(name)
		for i, p := range priorityBiomarkers {
			if strings.EqualFold(p, name) || strings.EqualFold(p, low) {
				return -1000 + i
			}
		}
		return 0
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		ri, rj := priorityRank(candidates[i].badge.Indicator), priorityRank(candidates[j].badge.Indicator)
		if ri != rj {
			return ri < rj
		}
		return candidates[i].count > candidates[j].count
	})

	const maxBadges = 4
	limit := maxBadges
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := make([]TrendBadge, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, candidates[i].badge)
	}
	return out
}

func monthsPhrase(n int) string {
	if n <= 0 {
		n = 1
	}
	unit := "месяцев"
	switch {
	case n%10 == 1 && n%100 != 11:
		unit = "месяц"
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20):
		unit = "месяца"
	}
	return "за последние " + strconv.Itoa(n) + " " + unit
}
