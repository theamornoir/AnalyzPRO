package report

type Report struct {
	Score int `json:"score"`

	Level string `json:"level"`

	Summary string `json:"summary"`

	Body BodyInfo `json:"body"`

	Composition string `json:"composition"`

	Profile ProfileInfo `json:"profile"`

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

type BodyInfo struct {
	Height string `json:"height"`

	Weight string `json:"weight"`

	MuscleMass string `json:"muscle_mass"`

	Fat string `json:"fat"`
}

type ProfileInfo struct {
	Composition int `json:"composition"`

	MuscleDevelopment int `json:"muscle_development"`

	Balance int `json:"balance"`

	Potential int `json:"potential"`

	CompositionAngle int `json:"composition_angle"`

	MuscleAngle int `json:"muscle_angle"`

	BalanceAngle int `json:"balance_angle"`

	PotentialAngle int `json:"potential_angle"`
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
