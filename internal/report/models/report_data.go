package models

// AdaptiveReportData - данные для адаптивного HTML-отчёта.
// Генерируется AI и содержит только те секции, по которым есть данные.
type AdaptiveReportData struct {
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Sections        []Section `json:"sections"`
	Recommendations []string  `json:"recommendations,omitempty"`
	Disclaimer      string    `json:"disclaimer,omitempty"`
}

// Section - одна секция отчёта.
type Section struct {
	Type       string      `json:"type"` // blood, lifestyle, nutrition, recommendation, warning, profile
	Title      string      `json:"title"`
	Indicators []Indicator `json:"indicators,omitempty"`
	Summary    string      `json:"summary,omitempty"`
	List       []string    `json:"list,omitempty"`
	Score      int         `json:"score,omitempty"` // для круговых диаграмм (0-100)
}

// Indicator - один показатель внутри секции.
type Indicator struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Unit   string `json:"unit,omitempty"`
	Normal string `json:"normal,omitempty"`
	Status string `json:"status"` // normal, warning, critical
	Score  int    `json:"score,omitempty"`
}
