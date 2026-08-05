package pdf

// BioscanReport - структура данных для HTML отчёта
type BioscanReport struct {
	Score int `json:"score"`

	Level string `json:"level"`

	Summary string `json:"summary"`

	Body BodyData `json:"body"`

	Composition string `json:"composition"`

	Profile ProfileData `json:"profile"`

	Zones []Zone `json:"zones"`

	Muscles []Muscle `json:"muscles"`

	Posture Posture `json:"posture"`

	AttentionZones []AttentionZone `json:"attention_zones"`

	Priorities []Priority `json:"priorities"`

	TrainingDays []TrainingDay `json:"training_days"`

	Nutrition []string `json:"nutrition"`

	Recovery []string `json:"recovery"`

	Progress Progress `json:"progress"`
}

type BodyData struct {
	Height string `json:"height"`

	Weight string `json:"weight"`

	MuscleMass string `json:"muscle_mass"`

	Fat string `json:"fat"`
}

type ProfileData struct {
	Composition int `json:"composition"`

	MuscleDevelopment int `json:"muscle_development"`

	Balance int `json:"balance"`

	Potential int `json:"potential"`
}

type Zone struct {
	Name string `json:"name"`

	Score int `json:"score"`

	Status string `json:"status"`

	Description string `json:"description"`
}

type Muscle struct {
	Name string `json:"name"`

	Level string `json:"level"`

	Assessment string `json:"assessment"`

	Symmetry string `json:"symmetry"`

	Recommendation string `json:"recommendation"`
}

type Posture struct {
	Type string `json:"type"`

	Head string `json:"head"`

	Shoulders string `json:"shoulders"`

	Pelvis string `json:"pelvis"`

	Description string `json:"description"`
}

type AttentionZone struct {
	Name string `json:"name"`

	Problem string `json:"problem"`

	Solution string `json:"solution"`
}

type Priority struct {
	Title string `json:"title"`

	Description string `json:"description"`
}

type TrainingDay struct {
	Day string `json:"day"`

	Exercises []Exercise `json:"exercises"`
}

type Exercise struct {
	Name string `json:"name"`

	Sets string `json:"sets"`

	Reps string `json:"reps"`
}

type Progress struct {
	Recheck string `json:"recheck"`

	Targets []string `json:"targets"`
}
