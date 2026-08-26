package models

// HealthAssessment - структура отчёта «Общая оценка здоровья», который
// строится ИСКЛЮЧИТЕЛЬНО по ответам пользователя на опросник об образе
// жизни (без загрузки медицинских файлов). Формируется ИИ из текста
// опросника и возвращается в виде JSON.
type HealthAssessment struct {
	// HealthIndex - общий индекс здоровья (0-100).
	HealthIndex int `json:"health_index"`
	// Summary - общий развёрнутый разбор образа жизни.
	Summary string `json:"summary"`
	// Lifestyle - разбор по сферам образа жизни (сон, питание, движение,
	// стресс, энергия). Каждый элемент содержит оценку 0-100 и комментарий.
	Lifestyle map[string]LifestyleDim `json:"lifestyle"`
	// RiskZones - зоны риска (красные флаги / что требует внимания).
	RiskZones []RiskZone `json:"risk_zones"`
	// Plan - персональный план улучшения на 3 месяца по 4 сферам.
	Plan HealthPlan `json:"plan"`
}

// LifestyleDim - оценка одной сферы образа жизни.
type LifestyleDim struct {
	Score   int    `json:"score"`
	Comment string `json:"comment"`
}

// RiskZone - одна зона риска.
type RiskZone struct {
	Name        string `json:"name"`
	Level       string `json:"level"`
	Description string `json:"description"`
}

// HealthPlan - персональный план улучшения на 3 месяца.
type HealthPlan struct {
	Sleep     string `json:"sleep"`
	Nutrition string `json:"nutrition"`
	Movement  string `json:"movement"`
	Stress    string `json:"stress"`
}
