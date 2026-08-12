package models

// Report - основная структура отчёта анализа
type Report struct {
	Profile         Profile    `json:"profile"`
	Categories      []Category `json:"categories"`
	Summary         string     `json:"summary"`
	Attention       []string   `json:"attention"`
	Recommendations []string   `json:"recommendations"`
	Disclaimer      string     `json:"disclaimer"`
	IsBioscan       bool       `json:"-"`

	// Для Bioscan
	Score          int             `json:"score"`
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
	Name              string `json:"name"`
	Date              string `json:"date"`
	Age               int    `json:"age"`
	Gender            string `json:"gender"`
	Composition       int    `json:"composition"`
	MuscleDevelopment int    `json:"muscle_development"`
	Balance           int    `json:"balance"`
	Potential         int    `json:"potential"`
	CompositionAngle  int    `json:"-"`
	MuscleAngle       int    `json:"-"`
	BalanceAngle      int    `json:"-"`
	PotentialAngle    int    `json:"-"`
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
	Name           string `json:"name"`
	Score          int    `json:"score"`
	Status         string `json:"status"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
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
