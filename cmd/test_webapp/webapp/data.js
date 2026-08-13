// ============================================
// AnalyzPRO Web App — Интерактивный дашборд
// ============================================
// Моковые данные + интерактивные графики
// ============================================

const MOCK = {
    // Пользователь
    user: {
        name: "Алексей",
        age: 32,
        since: "2025-01-15",
        premium: true,
        premiumUntil: "2026-02-15"
    },

    // Индексы здоровья (0-100)
    health: {
        overall: 78,
        energy: 82,
        stress: 45,
        sleep: 71,
        hydration: 65
    },

    // Кровь
    blood: {
        hemoglobin: { value: 145, unit: "г/л", normal: "130-170", status: "normal", score: 92 },
        leukocytes: { value: 6.2, unit: "×10⁹/л", normal: "4.0-9.0", status: "normal", score: 88 },
        platelets: { value: 250, unit: "×10⁹/л", normal: "150-400", status: "normal", score: 85 },
        glucose: { value: 5.8, unit: "ммоль/л", normal: "3.9-5.5", status: "warning", score: 62 },
        cholesterol: { value: 5.4, unit: "ммоль/л", normal: "3.0-5.2", status: "warning", score: 58 },
        crreatinine: { value: 88, unit: "мкмоль/л", normal: "62-115", status: "normal", score: 90 }
    },

    // Питание
    nutrition: {
        protein: { current: 85, target: 100, unit: "г" },
        fats: { current: 65, target: 70, unit: "г" },
        carbs: { current: 280, target: 300, unit: "г" },
        calories: { current: 2100, target: 2400, unit: "ккал" },
        water: { current: 2.1, target: 2.5, unit: "л" },
        fiber: { current: 18, target: 30, unit: "г" }
    },

    // Активность
    activity: {
        steps: { current: 8500, target: 10000 },
        calories: { current: 2200, target: 2500 },
        workoutMin: { current: 45, target: 60 },
        sleepHours: { current: 7.2, target: 8 }
    },

    // Динамика за 6 месяцев
    trend: {
        labels: ["Янв", "Фев", "Мар", "Апр", "Май", "Июн"],
        health: [65, 68, 72, 70, 75, 78],
        energy: [70, 72, 75, 78, 80, 82],
        stress: [55, 52, 50, 48, 47, 45],
        sleep: [60, 63, 65, 68, 70, 71]
    },

    // Риски
    risks: [
        { name: "Глюкоза", level: "warning", desc: "Пограничное значение. Рекомендуется контролировать сахар в рационе." },
        { name: "Холестерин", level: "warning", desc: "Незначительно повышен. Рекомендована диета с ограничением насыщенных жиров." },
        { name: "Стресс", level: "critical", desc: "Высокий уровень стресса. Рекомендованы техники релаксации и корректировка режима." }
    ],

    // Рекомендации
    recommendations: [
        "🥗 Увеличить потребление белка до 1.5г/кг массы тела",
        "💧 Пить минимум 2.5л воды в день",
        "🏃 Добавить 30 минут кардио 3 раза в неделю",
        "😴 Нормализовать режим сна (7-8 часов)",
        "🧘 Практиковать медитацию 10 минут ежедневно",
        "📊 Контролировать уровень глюкозы раз в месяц"
    ],

    // Мышцы (для биоскана)
    muscles: [
        { name: "Грудь", score: 85, status: "Отлично развиты" },
        { name: "Плечи", score: 82, status: "Отлично развиты" },
        { name: "Спина", score: 78, status: "Хорошо развиты" },
        { name: "Ноги", score: 92, status: "Отлично развиты" },
        { name: "Бицепс", score: 70, status: "Хорошо развиты" },
        { name: "Трицепс", score: 68, status: "Средне развиты" },
        { name: "Пресс", score: 55, status: "Требует внимания" },
        { name: "Трапеция", score: 60, status: "Средне развиты" }
    ]
};
