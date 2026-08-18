package models

// HealthDossier - универсальный расширенный отчёт-досье здоровья.
// Строится на основе присланных пользователем анализов и 20-вопросного
// опросника об образе жизни. Генерируется ИИ в виде JSON и рендерится в
// богатый print-ready HTML (templates/health_dossier.html).
type HealthDossier struct {
	Title      string             `json:"title"`
	Patient    DossierPatient     `json:"patient"`
	Scores     DossierScores      `json:"scores"`
	Survey     []SurveyQA         `json:"survey"`
	Lifestyle  []LifestyleSection `json:"lifestyle"`
	LabSystems []LabSystem        `json:"lab_systems"`
	// Synthesis - общая картина здоровья: связный развёрнутый разбор,
	// объединяющий образ жизни и присланные анализы в единый «портрет».
	// Рендерится на обложке отчёта как отдельный блок «Общая картина».
	Synthesis  string            `json:"synthesis"`
	Priorities []DossierPriority `json:"priorities"`
	References []Reference       `json:"references"`
	Disclaimer string            `json:"disclaimer"`
}

// DossierPatient - идентификационные данные из опросника.
type DossierPatient struct {
	Name   string `json:"name"`
	Age    string `json:"age"`
	Gender string `json:"gender"`
	Date   string `json:"date"`
}

// DossierScores - интегральные оценки для обложки (каждая 0-100).
type DossierScores struct {
	Health    int `json:"health"`    // состояние здоровья
	Potential int `json:"potential"` // потенциал улучшения
	// Размерности образа жизни (для баров/донатов на обложке).
	Wellbeing int `json:"wellbeing"`
	Sleep     int `json:"sleep"`
	Stress    int `json:"stress"`
	Activity  int `json:"activity"`
	Nutrition int `json:"nutrition"`
	Focus     int `json:"focus"`
}

// SurveyQA - один вопрос/ответ из 20-вопросного опросника.
type SurveyQA struct {
	Num      int    `json:"num"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// LifestyleSection - один раздел образа жизни (сон, стресс, питание,
// активность, семейный анамнез и т.п.).
type LifestyleSection struct {
	Key             string   `json:"key"`
	Title           string   `json:"title"`
	Score           int      `json:"score"`     // 0-100
	Status          string   `json:"status"`    // good | warning | risk
	Narrative       string   `json:"narrative"` // связный разбор
	Recommendations []string `json:"recommendations"`
}

// LabSystem - одна лабораторная система (углеводный обмен, липиды, печень,
// почки, гематология, гормоны и т.п.).
type LabSystem struct {
	Key             string             `json:"key"`
	Title           string             `json:"title"`
	Status          string             `json:"status"` // normal | warning | critical
	Summary         string             `json:"summary"`
	Indicators      []DossierIndicator `json:"indicators"`
	Narrative       string             `json:"narrative"`
	Recommendations []string           `json:"recommendations"`
}

// DossierIndicator - один лабораторный показатель внутри системы.
type DossierIndicator struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Unit   string `json:"unit"`
	Normal string `json:"normal"`
	Status string `json:"status"` // normal | warning | critical
}

// DossierPriority - один приоритетный блок интегральных рекомендаций.
type DossierPriority struct {
	Title  string   `json:"title"`
	Period string   `json:"period"`
	Items  []string `json:"items"`
}

// Reference - научный источник/методология.
type Reference struct {
	Source string `json:"source"`
	Detail string `json:"detail"`
}
