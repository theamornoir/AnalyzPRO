package models

// HealthAssessment - структура отчёта «Общая оценка здоровья», который
// строится ИСКЛЮЧИТЕЛЬНО по ответам пользователя на опросник об образе
// жизни (без загрузки медицинских файлов). Формируется ИИ из текста
// опросника и возвращается в виде JSON.
type HealthAssessment struct {
	// Name - имя пользователя для шапки отчёта. Заполняется бэкендом из
	// профиля (user_profiles / собранные данные опросника). ИИ это поле НЕ
	// генерирует и НЕ использует: в comment/summary/plan пишется обезличенно
	// (например, «спит 8 часов», а не «Влад спит 8 часов»).
	Name string `json:"name"`
	// HealthIndex - общий индекс здоровья (0-100).
	HealthIndex int `json:"health_index"`
	// Summary - общий развёрнутый разбор образа жизни.
	Summary string `json:"summary"`
	// Lifestyle - разбор по 5 сферам образа жизни (сон, питание, физическая
	// активность, стресс, вредные привычки). Каждый элемент содержит оценку
	// 0-100 и комментарий.
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

// HealthPlan - персональный план улучшения на 3 месяца по 4 сферам.
type HealthPlan struct {
	Sleep     string `json:"sleep"`
	Nutrition string `json:"nutrition"`
	// Wellbeing - шаги по улучшению общего самочувствия (без спорта и
	// тренировок - они не входят в отчёт «Общая оценка здоровья»).
	Wellbeing string `json:"wellbeing"`
	Stress    string `json:"stress"`
}
