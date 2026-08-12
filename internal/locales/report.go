package locales

// Report rendering: status icons and texts
const (
	StatusIconNormal   = "✅"
	StatusIconWarning  = "⚠️"
	StatusIconCritical = "❌"
	StatusIconDefault  = "ℹ️"

	StatusTextNormal   = "В норме"
	StatusTextWarning  = "Требует внимания"
	StatusTextCritical = "Отклонение"

	CategoryIconDefault = "📋"
)

// CategoryIcons - map of category names to emoji icons.
var CategoryIcons = map[string]string{
	"Общий анализ крови":   "🩸",
	"Биохимический анализ": "🧬",
	"Гормоны":              "💉",
	"Липидный профиль":     "📊",
	"Коагулограмма":        "🧫",
	"Иммунология":          "🛡️",
	"Микроэлементы":        "🔬",
	"Витамины":             "💊",
	"Онкомаркеры":          "🎯",
	"Маркеры воспаления":   "🔥",
}
