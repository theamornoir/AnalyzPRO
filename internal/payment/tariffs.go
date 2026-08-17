package payment

import (
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// Tariff - тарифная модель. Все параметры тарифа (цена, длительность, название,
// описание, список фич) централизованы в locales.TariffText по ID; здесь код
// только собирает структуру и задаёт порядок вывода в меню.
type Tariff struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       int           `json:"price"` // в копейках (₽ * 100)
	Duration    time.Duration `json:"duration"`
	Features    []string      `json:"features"`
}

// tariffOrder - порядок вывода тарифов в меню (только ID; тексты/цены в locales).
// Гарантирует детерминированный порядок, т.к. перебор map в Go случаен.
var tariffOrder = []string{
	"premium_monthly",
	"premium_quarterly",
	"premium_yearly",
}

// AvailableTariffs - доступные тарифы (все данные из locales.TariffText).
var AvailableTariffs []Tariff

func init() {
	for _, id := range tariffOrder {
		t, ok := locales.TariffText[id]
		if !ok {
			continue
		}
		AvailableTariffs = append(AvailableTariffs, Tariff{
			ID:          id,
			Name:        t.Name,
			Description: t.Description,
			Price:       t.PriceKopecks,
			Duration:    time.Duration(t.DurationDays) * 24 * time.Hour,
			Features:    t.Features,
		})
	}
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
