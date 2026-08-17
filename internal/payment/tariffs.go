package payment

import "time"

// Tariff - тарифная модель.
type Tariff struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       int           `json:"price"` // в копейках (₽ * 100)
	Duration    time.Duration `json:"duration"`
	Features    []string      `json:"features"`
}

// AvailableTariffs - доступные тарифы.
var AvailableTariffs = []Tariff{
	{
		ID:          "premium_monthly",
		Name:        "Premium (1 месяц)",
		Description: "Полный доступ ко всем функциям бота на 30 дней",
		Price:       49900, // 499 ₽
		Duration:    30 * 24 * time.Hour,
		Features: []string{
			"📊 Адаптивные HTML-отчёты",
			"📊 Интерактивный дашборд",
			"🔬 Расширенный анализ",
			"📸 Bioscan без ограничений",
			"🤖 Быстрая консультация (безлимит)",
			"🏥 Приоритетная поддержка",
		},
	},
	{
		ID:          "premium_quarterly",
		Name:        "Premium (3 месяца)",
		Description: "Полный доступ на 90 дней со скидкой 15%",
		Price:       127500, // 1275 ₽ (~425 ₽/мес)
		Duration:    90 * 24 * time.Hour,
		Features: []string{
			"Всё из месячного тарифа",
			"🤖 Быстрая консультация (безлимит)",
			"🎁 Скидка 15%",
			"📊 Расширенная аналитика",
		},
	},
	{
		ID:          "premium_yearly",
		Name:        "Premium (1 год)",
		Description: "Полный доступ на 365 дней со скидкой 30%",
		Price:       179900, // 1799 ₽ (~150 ₽/мес)
		Duration:    365 * 24 * time.Hour,
		Features: []string{
			"Всё из квартального тарифа",
			"🤖 Быстрая консультация (безлимит)",
			"🎁 Скидка 30%",
			"🏆 Ранний доступ к новым функциям",
		},
	},
}

// GetTariffByID - найти тариф по ID.
func GetTariffByID(id string) *Tariff {
	for i := range AvailableTariffs {
		if AvailableTariffs[i].ID == id {
			return &AvailableTariffs[i]
		}
	}
	return nil
}
