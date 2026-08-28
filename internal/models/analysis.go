package models

// Report - основная структура отчёта анализа
type Report struct {
	Title           string     `json:"title"`
	Profile         Profile    `json:"profile"`
	Categories      []Category `json:"categories"`
	Summary         string     `json:"summary"`
	Attention       []string   `json:"attention"`
	Recommendations []string   `json:"recommendations"`
	Disclaimer      string     `json:"disclaimer"`
	IsBioscan       bool       `json:"-"`

	// Для Bioscan
	Score          FlexInt         `json:"score"`
	Level          string          `json:"level"`
	Body           Body            `json:"body"`
	Composition    string          `json:"composition"`
	Zones          []Zone          `json:"zones"`
	Muscles        []Muscle        `json:"muscles"`
	Posture        Posture         `json:"posture"`
	AttentionZones []AttentionZone `json:"attention_zones"`
	Priorities     []Priority      `json:"priorities"`
	TrainingDays   []TrainingDay   `json:"training_days"`
	Nutrition      []string        `json:"nutrition"`
	Recovery       []string        `json:"recovery"`
	Progress       Progress        `json:"progress"`
}

type Profile struct {
	Name              string  `json:"name"`
	Date              string  `json:"date"`
	Age               FlexInt `json:"age"`
	Gender            string  `json:"gender"`
	Composition       FlexInt `json:"composition"`
	MuscleDevelopment FlexInt `json:"muscle_development"`
	Balance           FlexInt `json:"balance"`
	Potential         FlexInt `json:"potential"`
	CompositionAngle  FlexInt `json:"-"`
	MuscleAngle       FlexInt `json:"-"`
	BalanceAngle      FlexInt `json:"-"`
	PotentialAngle    FlexInt `json:"-"`
}

type Category struct {
	Name        string      `json:"name"`
	Indicators  []Indicator `json:"indicators"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	Color       string      `json:"color"`
}

type Indicator struct {
	Name           string `json:"name"`
	Value          string `json:"value"`
	Unit           string `json:"unit"`
	Normal         string `json:"normal"`
	Status         string `json:"status"`
	Description    string `json:"description"`
	Explanation    string `json:"explanation"`
	Risk           string `json:"risk"`
	Recommendation string `json:"recommendation"`
	Role           string `json:"role"`
	ShortDesc      string `json:"short_desc"`
	FullDesc       string `json:"full_desc"`
	Function       string `json:"function"`
}

type Body struct {
	Height     string `json:"height"`
	Weight     string `json:"weight"`
	MuscleMass string `json:"muscle_mass"`
	Fat        string `json:"fat"`
}

type Zone struct {
	Name           string  `json:"name"`
	Score          FlexInt `json:"score"`
	Status         string  `json:"status"`
	Description    string  `json:"description"`
	Recommendation string  `json:"recommendation"`
}

type Muscle struct {
	Name           string `json:"name"`
	Level          string `json:"level"`
	Assessment     string `json:"assessment"`
	Symmetry       string `json:"symmetry"`
	Recommendation string `json:"recommendation"`
}

type Posture struct {
	Type        string `json:"type"`
	Head        string `json:"head"`
	Shoulders   string `json:"shoulders"`
	Pelvis      string `json:"pelvis"`
	Description string `json:"description"`
}

type AttentionZone struct {
	Name     string `json:"name"`
	Problem  string `json:"problem"`
	Solution string `json:"solution"`
}

type Priority struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type TrainingDay struct {
	Day       string     `json:"day"`
	Exercises []Exercise `json:"exercises"`
}

type Exercise struct {
	Name string `json:"name"`
	Sets string `json:"sets"`
	Reps string `json:"reps"`
}

type Progress struct {
	Recheck string   `json:"recheck"`
	Targets []string `json:"targets"`
}

// ============================================================================
// BodyScanReport - структура премиального отчёта Bioscan PRO (Body Intelligence).
// Строится ИИ из 4 фото + опросника и рендерится в подробный print-ready HTML
// (templates/body_scan_report.html). Значения композиции - ЭКСПЕРТНЫЕ ОЦЕНКИ
// по фото + данным пользователя, не заменяют инструментальные замеры.
// ============================================================================

type BodyScanReport struct {
	Title     string  `json:"title"`
	ReportID  string  `json:"report_id"`
	Date      string  `json:"date"`
	Name      string  `json:"name"`
	Age       string  `json:"age"`
	Gender    string  `json:"gender"`
	Score     FlexInt `json:"score"`
	Level     string  `json:"level"`
	Summary   string  `json:"summary"`
	Potential FlexInt `json:"potential"`
	// Gap - разрыв между текущим и потенциальным (вычисляется в Go).
	Gap FlexInt `json:"gap"`

	// CoverMetrics - 5 ключевых метрик обложки.
	CoverMetrics []BodyScanMetric `json:"cover_metrics"`
	// Composition - детальная композиция тела (до 9 показателей).
	Composition []BodyScanMetric `json:"composition"`

	Strengths []BodyScanCard `json:"strengths"`
	Improve   []BodyScanCard `json:"improve"`

	Zones []BodyScanZone `json:"zones"`

	Posture BodyScanPosture `json:"posture"`

	PotentialAreas []BodyScanPotential `json:"potential_areas"`

	// TrainingProgram - детальная программа тренировок (фазы/недели,
	// сессии, упражнения). Рендерится отдельной страницей отчёта.
	TrainingProgram []BodyScanTrainingPhase `json:"training_program"`

	Recommendations []BodyScanRecommendation `json:"recommendations"`

	Disclaimer string `json:"disclaimer"`
}

// BodyScanTrainingPhase - одна фаза/блок программы тренировок (например,
// «Недели 1-2 · База и техника») с набором сессий.
type BodyScanTrainingPhase struct {
	Phase    string                    `json:"phase"`
	Title    string                    `json:"title"`
	Goal     string                    `json:"goal"`
	Sessions []BodyScanTrainingSession `json:"sessions"`
}

// BodyScanExercise - одно упражнение в сессии тренировки (Bioscan PRO).
// Детальная карточка: название, целевая мышца, подходы/повторения, отдых
// и техника выполнения.
type BodyScanExercise struct {
	Name      string `json:"name"`      // название, напр. "Жим штанги лёжа"
	Target    string `json:"target"`    // целевая мышца/группа, напр. "Грудь, Трицепс"
	SetsReps  string `json:"sets_reps"` // подходы и повторения, напр. "4 подхода × 8-10 повторений"
	Rest      string `json:"rest"`      // отдых между подходами, напр. "Отдых 60-90 сек"
	Technique string `json:"technique"` // техника выполнения
}

// BodyScanTrainingSession - одна тренировочная сессия внутри фазы.
type BodyScanTrainingSession struct {
	Name      string             `json:"name"`
	Focus     string             `json:"focus"`
	Exercises []BodyScanExercise `json:"exercises"`
}

// BodyScanMetric - один показатель композиции (значение + статус + референс).
type BodyScanMetric struct {
	Name           string `json:"name"`
	Value          string `json:"value"`
	Unit           string `json:"unit"`
	Status         string `json:"status"` // good, warning, critical
	Ref            string `json:"ref"`
	Interpretation string `json:"interpretation"`
}

// BodyScanCard - карточка «сильная сторона» или «зона роста».
type BodyScanCard struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// BodyScanZone - зона тела (плечи, пресс, ноги и т.п.).
type BodyScanZone struct {
	Name    string  `json:"name"`
	Score   FlexInt `json:"score"`
	Status  string  `json:"status"` // good, warning, critical
	Comment string  `json:"comment"`
}

// BodyScanPosture - осанка и баланс (7 осей для radar).
type BodyScanPosture struct {
	PostureScore    FlexInt `json:"posture_score"`
	Symmetry        FlexInt `json:"symmetry"`
	ShoulderBalance FlexInt `json:"shoulder_balance"`
	PelvicBalance   FlexInt `json:"pelvic_balance"`
	SpinalAlignment FlexInt `json:"spinal_alignment"`
	Mobility        FlexInt `json:"mobility"`
	Stability       FlexInt `json:"stability"`
	Narrative       string  `json:"narrative"`
}

// BodyScanPotential - направление раскрытия потенциала.
type BodyScanPotential struct {
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Effect   string `json:"effect"`
}

// BodyScanRecommendation - персональная рекомендация (4 категории).
type BodyScanRecommendation struct {
	Category string `json:"category"` // TRAINING, NUTRITION, RECOVERY, LIFESTYLE
	Icon     string `json:"icon"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Effect   string `json:"effect"`
}
