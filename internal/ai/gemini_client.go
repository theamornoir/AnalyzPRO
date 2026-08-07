package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================
// MOCK DATA ДЛЯ ТЕСТИРОВАНИЯ
// ============================================

func getMockAnalysis(input string) string {
	return `📊 РЕЗУЛЬТАТЫ АНАЛИЗОВ

📅 Дата: 06.08.2026

🩸 Общий анализ крови

✅ Гемоглобин: 145 г/л (норма: 120-160)
• Транспортный белок, доставляющий кислород к тканям. Уровень в норме.

✅ Эритроциты: 4.8 ×10^12/л (норма: 4.0-5.5)
• Красные кровяные клетки, участвующие в газообмене. Уровень в норме.

✅ Лейкоциты: 6.5 ×10^9/л (норма: 4.0-9.0)
• Белые кровяные клетки, защищающие организм от инфекций. Уровень в норме.

✅ Тромбоциты: 250 ×10^9/л (норма: 180-320)
• Клетки, отвечающие за свертываемость крови. Уровень в норме.

🧬 Биохимический анализ

✅ Глюкоза: 5.2 ммоль/л (норма: 3.3-5.5)
• Основной источник энергии для клеток. Уровень в норме.

✅ Холестерин общий: 4.8 ммоль/л (норма: <5.2)
• Липидный показатель, отражающий риск атеросклероза. Уровень в норме.

📋 ЗАКЛЮЧЕНИЕ

Все показатели находятся в пределах референсных значений. Состояние оценивается как удовлетворительное.

📊 Общая оценка:
• Всего показателей: 6
• В норме: 6

💡 Рекомендации:
• Продолжайте поддерживать здоровый образ жизни
• Регулярно проходите профилактические осмотры
• Соблюдайте сбалансированное питание

⚠️ ВНИМАНИЕ: Данный анализ носит информационный характер и не заменяет консультацию врача. Для постановки диагноза обратитесь к специалисту.`
}

func getMockAnalysisFromFile() string {
	return `📊 РЕЗУЛЬТАТЫ АНАЛИЗОВ

📅 Дата: 06.08.2026

🩸 Общий анализ крови

✅ Гемоглобин: 142 г/л (норма: 120-160)
• Транспортный белок. Уровень в норме.

✅ Лейкоциты: 6.2 ×10^9/л (норма: 4.0-9.0)
• Клетки иммунитета. Уровень в норме.

✅ Эритроциты: 4.9 ×10^12/л (норма: 4.0-5.5)
• Красные кровяные клетки. Уровень в норме.

✅ Тромбоциты: 230 ×10^9/л (норма: 180-320)
• Клетки свертываемости. Уровень в норме.

🧬 Биохимический анализ

✅ Глюкоза: 5.0 ммоль/л (норма: 3.3-5.5)
• Основной источник энергии. Уровень в норме.

❌ Холестерин: 5.1 ммоль/л (норма: <5.2)
• Липидный показатель на верхней границе нормы.

📋 ЗАКЛЮЧЕНИЕ

Большинство показателей в норме. Холестерин на верхней границе - рекомендуется коррекция питания.

📊 Общая оценка:
• Всего показателей: 6
• В норме: 5
• Требует внимания: 1

💡 Рекомендации:
• Уменьшить потребление жирной пищи
• Увеличить физическую активность
• Повторить анализ через 3 месяца

⚠️ ВНИМАНИЕ: Данный анализ носит информационный характер и не заменяет консультацию врача. Для постановки диагноза обратитесь к специалисту.`
}

// ============================================
// НОВЫЙ МОК ДЛЯ ОБЫЧНОГО АНАЛИЗА
// ВСЕГДА ПОКАЗЫВАЕТ 10 ПОКАЗАТЕЛЕЙ С РАЗНЫМИ СТАТУСАМИ
// ============================================

func getMockAnalysisFromData(input string) string {
	var result strings.Builder

	result.WriteString("📊 РЕЗУЛЬТАТЫ АНАЛИЗОВ\n\n")
	result.WriteString(fmt.Sprintf("📅 Дата: %s\n\n", time.Now().Format("02.01.2006")))

	result.WriteString("🩸 ОБЩИЙ АНАЛИЗ КРОВИ\n\n")

	indicators := getIndicatorsWithMixedStatus()

	for _, ind := range indicators {
		result.WriteString(ind)
		result.WriteString("\n\n")
	}

	// ============================
	// ЗАКЛЮЧЕНИЕ
	// ============================

	result.WriteString("📋 ЗАКЛЮЧЕНИЕ\n\n")

	result.WriteString(
		"По результатам анализа большинство показателей находятся в допустимых пределах. " +
			"При этом выявлены значения, которые требуют дополнительного внимания и контроля.\n\n",
	)

	// ============================
	// СТАТИСТИКА
	// ============================

	total := len(indicators)

	normal := 0
	warnings := 0
	critical := 0

	var attention []string

	for _, ind := range indicators {

		lines := strings.Split(ind, "\n")

		name := ""

		if len(lines) > 0 {

			name = strings.TrimSpace(lines[0])

			name = strings.ReplaceAll(name, "✅", "")
			name = strings.ReplaceAll(name, "⚠️", "")
			name = strings.ReplaceAll(name, "❌", "")

			if idx := strings.Index(name, ":"); idx > 0 {
				name = strings.TrimSpace(name[:idx])
			}
		}

		switch {
		case strings.Contains(ind, "❌"):
			critical++
			attention = append(attention, name)

		case strings.Contains(ind, "⚠️"):
			warnings++
			attention = append(attention, name)

		default:
			normal++
		}
	}

	result.WriteString("📊 Общая оценка\n\n")

	result.WriteString(fmt.Sprintf("Всего показателей: %d\n", total))
	result.WriteString(fmt.Sprintf("В норме: %d\n", normal))

	if warnings > 0 {
		result.WriteString(fmt.Sprintf("Требуют внимания: %d\n", warnings))
	}

	if critical > 0 {
		result.WriteString(fmt.Sprintf("Значимые отклонения: %d\n", critical))
	}

	// ============================
	// ЧТО ПОПРАВИТЬ
	// ============================

	if len(attention) > 0 {

		result.WriteString("\n\nЧто стоит проверить:\n\n")

		for _, item := range attention {

			result.WriteString("• ")
			result.WriteString(item)
			result.WriteString("\n")
		}
	}

	// ============================
	// РЕКОМЕНДАЦИИ
	// ============================

	result.WriteString("\n💡 Рекомендации\n\n")

	if critical > 0 {

		result.WriteString(
			"• Обсудить выявленные изменения с врачом\n" +
				"• При необходимости пройти дополнительное обследование\n" +
				"• Контролировать показатели в динамике\n",
		)

	} else if warnings > 0 {

		result.WriteString(
			"• Обратить внимание на выявленные показатели\n" +
				"• Поддерживать здоровый образ жизни\n" +
				"• Повторить контрольные анализы при необходимости\n",
		)

	} else {

		result.WriteString(
			"• Продолжать поддерживать здоровый образ жизни\n" +
				"• Регулярно проходить профилактические обследования\n",
		)
	}

	result.WriteString(
		"\n⚠️ ВНИМАНИЕ: Данный анализ носит информационный характер и не заменяет консультацию врача. " +
			"Для постановки диагноза обратитесь к специалисту.",
	)

	return result.String()
}

// ============================================
// 10 ПОКАЗАТЕЛЕЙ С РАЗНЫМИ СТАТУСАМИ
// ============================================

func getIndicatorsWithMixedStatus() []string {
	return []string{
		// ✅ В норме
		`✅ Гемоглобин: 145 г/л (норма: 120-160)
• Белок эритроцитов, отвечает за транспорт кислорода к тканям и органам. Является основным показателем для диагностики анемии.`,

		`✅ Эритроциты: 4.8 ×10^12/л (норма: 4.0-5.5)
• Красные кровяные клетки, обеспечивают газообмен в организме. Их количество отражает способность крови переносить кислород.`,

		`✅ Тромбоциты: 250 ×10^9/л (норма: 180-320)
• Клетки крови, отвечающие за свёртываемость. Предотвращают кровопотерю при повреждении сосудов.`,

		// ⚠️ Требует внимания
		`⚠️ Лейкоциты: 11.5 ×10^9/л (норма: 4.0-9.0)
• Белые кровяные клетки, главные защитники организма от инфекций. Повышение может указывать на воспалительный процесс или инфекцию.`,

		`⚠️ Глюкоза: 6.8 ммоль/л (норма: 3.3-5.5)
• Основной источник энергии для клеток организма. Повышенный уровень может свидетельствовать о нарушении углеводного обмена.`,

		`⚠️ Холестерин общий: 6.2 ммоль/л (норма: <5.2)
• Жироподобное вещество, необходимое для синтеза гормонов и клеточных мембран. Повышение увеличивает риск атеросклероза.`,

		// Критические отклонения
		`❌ Билирубин общий: 42 мкмоль/л (норма: 3.4-17.1)
• Значительное повышение может указывать на заболевания печени, желчевыводящих путей или усиленный распад эритроцитов.`,

		`❌ Креатинин: 180 мкмоль/л (норма: 44-104)
• Существенное повышение может свидетельствовать о выраженном нарушении функции почек.`,

		`❌ АЛТ (аланинаминотрансфераза): 85 Ед/л (норма: до 40)
• Фермент печени. Значительное повышение характерно для повреждения клеток печени.`,

		`❌ АСТ (аспартатаминотрансфераза): 95 Ед/л (норма: до 40)
• Фермент печени и мышечной ткани. Значительное повышение требует дополнительного обследования.`,
	}
}

// ============================================
// СТАРАЯ ФУНКЦИЯ ДЛЯ РАСШИРЕННОГО АНАЛИЗА (НЕ МЕНЯЕТСЯ)
// ============================================

func getMockAnalysisWithContext(context string) string {
	return `📊 Результаты анализов (с контекстом)
👤 Пациент: Тестовый пользователь
📅 Дата: 06.08.2026

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🩸 Общий анализ крови

Все показатели в норме.

📋 Информация о пациенте:
` + context + `

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ️ Важно: Это тестовый ответ. Для реального анализа подключите Gemini API.`
}

func getMockBioscanJSON(contextInfo string) string {
	return `{
  "score": 85,
  "level": "Хорошая форма",
  "summary": "Телосложение гармоничное, заметен развитый мышечный корсет. Есть потенциал для дальнейшего роста мышц верхней части тела.",
  "body": {
    "height": "175",
    "weight": "72",
    "muscle_mass": "34.5",
    "fat": "13.5"
  },
  "composition": "Мышечный тип с низким процентом жира. Хорошая плотность мышц.",
  "profile": {
    "composition": 82,
    "muscle_development": 78,
    "balance": 85,
    "potential": 90
  },
  "zones": [
    {
      "name": "Грудные мышцы",
      "score": 80,
      "status": "Хорошо развиты",
      "description": "Грудные мышцы имеют хорошую форму и объём. Верхняя часть груди требует дополнительной проработки.",
      "recommendation": "Жим штанги лёжа 4x8-10, разводка гантелей 3x12, жим на наклонной скамье 3x10"
    },
    {
      "name": "Плечи",
      "score": 75,
      "status": "Среднее развитие",
      "description": "Дельтовидные мышцы развиты средне. Средний пучок требует внимания.",
      "recommendation": "Жим гантелей сидя 4x10-12, боковые подъёмы 3x15, подъёмы перед собой 3x12"
    },
    {
      "name": "Спина",
      "score": 85,
      "status": "Хорошо развита",
      "description": "Широчайшие мышцы спины хорошо выражены. Трапеции требуют дополнительной проработки.",
      "recommendation": "Тяга штанги в наклоне 4x8-10, подтягивания 3x10, шраги 3x15"
    },
    {
      "name": "Бицепс",
      "score": 70,
      "status": "Среднее развитие",
      "description": "Бицепс развит средне, есть потенциал для роста.",
      "recommendation": "Подъём штанги на бицепс 3x10, молотковые сгибания 3x12, концентрированные подъёмы 3x10"
    },
    {
      "name": "Трицепс",
      "score": 72,
      "status": "Среднее развитие",
      "description": "Трицепс требует дополнительной проработки для объёма рук.",
      "recommendation": "Французский жим 3x12, разгибание рук на блоке 3x15, жим узким хватом 3x10"
    },
    {
      "name": "Пресс",
      "score": 90,
      "status": "Отлично развит",
      "description": "Мышцы пресса хорошо видны, кубики просматриваются. Нижняя часть пресса отстаёт.",
      "recommendation": "Скручивания 3x20, подъём ног в висе 3x15, косые скручивания 3x15"
    },
    {
      "name": "Квадрицепсы",
      "score": 88,
      "status": "Хорошо развиты",
      "description": "Квадрицепсы сильные, с хорошим объёмом.",
      "recommendation": "Приседания 4x8-10, жим ногами 3x12, разгибания ног 3x15"
    },
    {
      "name": "Бицепс бедра",
      "score": 82,
      "status": "Хорошо развиты",
      "description": "Бицепс бедра хорошо развит, но требует изолирующей работы.",
      "recommendation": "Румынская тяга 3x12, сгибания ног лёжа 3x15"
    },
    {
      "name": "Ягодицы",
      "score": 82,
      "status": "Хорошо развиты",
      "description": "Ягодичные мышцы имеют хорошую форму.",
      "recommendation": "Ягодичный мост 3x15, выпады 3x12, приседания сумо 3x12"
    },
    {
      "name": "Икры",
      "score": 75,
      "status": "Среднее развитие",
      "description": "Икроножные мышцы требуют дополнительной проработки.",
      "recommendation": "Подъёмы на носки стоя 4x15-20, подъёмы на носки сидя 3x15-20"
    }
  ],
  "posture": {
    "type": "Нормальная",
    "head": "Нейтральное положение, без наклона вперёд",
    "shoulders": "Нейтральное положение, без сутулости",
    "pelvis": "Нейтральное положение, без перекоса",
    "description": "Осанка оценивается как хорошая. Положение головы, плечевого пояса и таза находится в нейтральной позиции."
  },
  "attention_zones": [
    {
      "name": "Верхняя часть тела (грудь, плечи, руки)",
      "problem": "Недостаточный объём мышечной массы в верхней части тела по сравнению с нижней",
      "solution": "Увеличить частоту тренировок верхней части тела до 2-3 раз в неделю. Сделать акцент на базовых упражнениях."
    },
    {
      "name": "Мышечный баланс между верхом и низом",
      "problem": "Выраженный дисбаланс: нижняя часть тела значительно превосходит верхнюю по объёму",
      "solution": "Пересмотреть тренировочную программу: уменьшить объём тренировок ног до 1 раза в неделю, увеличить объём для верха."
    },
    {
      "name": "Икроножные мышцы",
      "problem": "Недостаточная проработка, отставание в объёме",
      "solution": "Добавить акцент на икроножные мышцы в каждую тренировку ног. Использовать разные углы и положения."
    }
  ],
  "priorities": [
    {
      "title": "1. Увеличение объёма верхней части тела",
      "description": "Основное направление улучшения связано с развитием плечевого пояса, груди и спины. Это позволит сделать силуэт более выраженным."
    },
    {
      "title": "2. Повышение качества мышечной структуры",
      "description": "Следующий этап развития должен быть направлен не только на увеличение массы, но и на улучшение плотности и формы мышц."
    },
    {
      "title": "3. Сохранение текущего уровня композиции тела",
      "description": "Важно сохранить благоприятное соотношение мышечной ткани и жировой массы во время дальнейшего прогресса."
    }
  ],
  "training_days": [
    {
      "day": "День 1 — Верх тела (силовой)",
      "exercises": [
        {"name": "Жим штанги лёжа", "sets": "4", "reps": "8-10"},
        {"name": "Тяга штанги в наклоне", "sets": "4", "reps": "8-10"},
        {"name": "Жим гантелей сидя", "sets": "4", "reps": "10-12"},
        {"name": "Тяга верхнего блока", "sets": "3", "reps": "10-12"},
        {"name": "Разводка гантелей лёжа", "sets": "3", "reps": "12-15"},
        {"name": "Подъём штанги на бицепс", "sets": "3", "reps": "10-12"}
      ]
    },
    {
      "day": "День 2 — Ноги (силовой)",
      "exercises": [
        {"name": "Приседания со штангой", "sets": "4", "reps": "8-10"},
        {"name": "Румынская тяга", "sets": "4", "reps": "10-12"},
        {"name": "Выпады с гантелями", "sets": "3", "reps": "10-12"},
        {"name": "Сгибания ног лёжа", "sets": "3", "reps": "12-15"},
        {"name": "Подъёмы на носки стоя", "sets": "4", "reps": "15-20"},
        {"name": "Ягодичный мост", "sets": "3", "reps": "12-15"}
      ]
    },
    {
      "day": "День 3 — Верх тела (объёмный)",
      "exercises": [
        {"name": "Жим гантелей лёжа", "sets": "4", "reps": "10-12"},
        {"name": "Тяга гантели в наклоне", "sets": "4", "reps": "10-12"},
        {"name": "Жим гантелей сидя", "sets": "4", "reps": "12-15"},
        {"name": "Тяга нижнего блока", "sets": "3", "reps": "12-15"},
        {"name": "Французский жим", "sets": "3", "reps": "12-15"},
        {"name": "Молотковые сгибания", "sets": "3", "reps": "12-15"}
      ]
    },
    {
      "day": "День 4 — Коррекция и функционал",
      "exercises": [
        {"name": "Планка", "sets": "3", "reps": "60-90 сек"},
        {"name": "Скручивания на пресс", "sets": "3", "reps": "20-25"},
        {"name": "Косые скручивания", "sets": "3", "reps": "15-20"},
        {"name": "Гиперэкстензия", "sets": "3", "reps": "12-15"},
        {"name": "Кардио (интервальное)", "sets": "15", "reps": "мин"},
        {"name": "Стретчинг", "sets": "10", "reps": "мин"}
      ]
    }
  ],
  "nutrition": [
    "Цель: умеренный профицит калорий (+200-300 ккал/день) для набора мышечной массы",
    "Белок: 1.8-2.2 г/кг веса (≈ 140-170 г/день) — куриная грудка, рыба, яйца, творог, протеин",
    "Жиры: 0.8-1 г/кг веса (≈ 60-80 г/день) — оливковое масло, орехи, авокадо, жирная рыба",
    "Углеводы: 4-5 г/кг веса (≈ 310-390 г/день) — рис, гречка, овсянка, картофель, фрукты",
    "Питание: 4-5 приёмов пищи в день, распределение белка равномерно",
    "Водный режим: 2.5-3 литра воды в день",
    "Ограничить: быстрые углеводы, трансжиры, алкоголь",
    "Добавить: BCAA во время тренировки, креатин, омега-3"
  ],
  "recovery": [
    "Сон: 7-8 часов в сутки, ложиться и вставать в одно и то же время",
    "Дни отдыха: 2 полных дня отдыха в неделю для восстановления ЦНС",
    "Стретчинг: 10-15 минут после каждой тренировки",
    "Массаж: 1-2 раза в неделю для улучшения кровообращения и расслабления мышц",
    "Баня/сауна: 1 раз в неделю для ускорения восстановления",
    "Контрастный душ: после тренировки для улучшения тонуса сосудов",
    "Прогулки на свежем воздухе: ежедневно 20-30 минут для снижения стресса"
  ],
  "progress": {
    "recheck": "Через 4-6 недель",
    "targets": [
      "Увеличение мышечной массы на 1-2 кг (чистая масса)",
      "Сохранение процента жира на текущем уровне (13-14%)",
      "Увеличение силовых показателей на 10-15% во всех базовых упражнениях",
      "Улучшение пропорций тела: увеличение объёма грудной клетки на 2-3 см",
      "Улучшение качества мышц: появление более чёткого рельефа",
      "Контроль качества сна и восстановления"
    ]
  }
}`
}

// ============================================
// ОСНОВНАЯ СТРУКТУРА И МЕТОДЫ
// ============================================

type GeminiClient struct {
	model  string
	apiKey string
	client *http.Client
}

func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-3.6-flash"
	}

	model = strings.TrimPrefix(model, "models/")

	log.Printf("🔑 Gemini Client initialized with model: %s", model)

	return &GeminiClient{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *GeminiClient) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	log.Printf("📤 GenerateAnalysisSummary called with input length: %d", len(userInput))

	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock response for analysis")
		return getMockAnalysisFromData(userInput), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		log.Printf("❌ API key is empty or only whitespace")
		return noKeyFallback(), nil
	}

	var prompt string
	if strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") &&
		strings.Contains(userInput, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") {
		courseInfo := extractCourseInfo(userInput)
		if courseInfo != "" {
			prompt = c.buildPromptForAthlete(userInput, courseInfo)
			log.Printf("🏋️ Athlete mode: on course")
		} else {
			prompt = c.buildPromptForRegular(userInput)
			log.Printf("👤 Regular mode: no course info")
		}
	} else if strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		if strings.Contains(userInput, "Вид спорта") || strings.Contains(userInput, "Стаж тренировок") {
			courseInfo := extractCourseInfo(userInput)
			if courseInfo != "" {
				prompt = c.buildPromptForAthlete(userInput, courseInfo)
				log.Printf("🏋️ Athlete mode: sportsman detected")
			} else {
				prompt = c.buildPromptForRegular(userInput)
				log.Printf("👤 Regular mode")
			}
		} else {
			prompt = c.buildPromptForRegular(userInput)
			log.Printf("👤 Regular mode")
		}
	} else {
		prompt = c.buildPromptForRegular(userInput)
		log.Printf("👤 Regular mode")
	}

	log.Printf("📝 Built prompt length: %d characters", len(prompt))

	return c.generate(ctx, []geminiPart{{Text: prompt}})
}

func (c *GeminiClient) buildPromptForRegular(text string) string {
	return fmt.Sprintf(`Ты опытный врач-диагност. Проанализируй ТОЛЬКО те показатели, которые присутствуют в предоставленных анализах.

ВАЖНЫЕ ПРАВИЛА:

1. НЕ придумывай показатели, которых нет.
2. НЕ добавляй категории, которых нет в анализах.
3. НЕ используй символы *, **, --- или ━━━.
4. НЕ используй Markdown.
5. Для каждого показателя ставь статус:
   ✅ — показатель в пределах нормы.
   ⚠️ — умеренное отклонение.
   ❌ — выраженное отклонение.

ФОРМАТ ОТВЕТА СТРОГО СЛЕДУЮЩИЙ:

📊 РЕЗУЛЬТАТЫ АНАЛИЗОВ

📅 Дата: [дата исследования]

[Название раздела]

✅ Гемоглобин: 145 г/л (норма: 120–160)
• Краткое описание показателя.

⚠️ Лейкоциты: 11.2 ×10⁹/л (норма: 4.0–9.0)
• Краткое описание показателя.

❌ Глюкоза: 7.2 ммоль/л (норма: 3.3–5.5)
• Краткое описание показателя.

📋 ЗАКЛЮЧЕНИЕ

Кратко опиши общую картину анализов.

Критические отклонения

• Глюкоза
• Креатинин

Показатели, требующие внимания

• Лейкоциты
• Холестерин

Общая оценка

• Всего показателей: N
• В норме: N
• Требуют внимания: N
• Критических отклонений: N

Рекомендации

• Первая рекомендация.
• Вторая рекомендация.
• Третья рекомендация.

ВНИМАНИЕ

Данный анализ носит исключительно информационный характер и не заменяет консультацию врача. Для постановки диагноза и назначения лечения обратитесь к квалифицированному специалисту.

ДОПОЛНИТЕЛЬНЫЕ ПРАВИЛА:

- Заголовки "Критические отклонения", "Показатели, требующие внимания", "Общая оценка", "Рекомендации" и "ВНИМАНИЕ" выводи БЕЗ эмодзи.
- После каждого заголовка обязательно оставляй пустую строку.
- Если критических отклонений нет — НЕ выводи раздел "Критические отклонения".
- Если показателей с предупреждением нет — НЕ выводи раздел "Показатели, требующие внимания".
- Не используй слова "🔴", "🟡", "❌ Критические отклонения", "⚠️ Показатели..." в заголовках.
- Используй эмодзи только возле конкретных показателей.
- Ответ должен выглядеть как медицинское заключение, аккуратно оформленное для чтения в Telegram.

Данные для анализа:

%s`, text)
}

func (c *GeminiClient) buildPromptForAthlete(text string, courseInfo string) string {
	return fmt.Sprintf(`Ты опытный спортивный врач и фармаколог. Отвечай строго по шаблону.

📌 ПРАВИЛА ФОРМАТИРОВАНИЯ:
1. Используй разделители для каждой категории
2. Каждый показатель на новой строке
3. Учитывай влияние препаратов
4. Профессиональный, но понятный язык

📋 ШАБЛОН ОТВЕТА:

📊 Спортивный анализ
👤 Спортсмен: [имя]
🎯 Цель: [цель]
💪 Стаж: [стаж]
🏋️ Вид спорта: [вид спорта]
💊 Препараты: [курс]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🩸 Общий анализ крови

[показатель 1] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
📈 Влияние на спорт: [описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

[показатель 2] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
📈 Влияние на спорт: [описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🧬 Биохимический анализ

[аналогичный формат с учетом препаратов]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💉 Гормоны (с учетом препаратов)

[аналогичный формат с учетом препаратов]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🏋️ Рекомендации по курсу

1. [рекомендация по препарату 1]
2. [рекомендация по препарату 2]
3. [рекомендация по поддержке]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Общее заключение

[1-2 предложения итога с учетом курса]

⚠️ На что обратить внимание:
• [пункт 1]
• [пункт 2]

✅ Рекомендации:
1. [рекомендация 1]
2. [рекомендация 2]
3. [рекомендация 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ️ Важно: Анализ выполнен с учетом приема препаратов. Не является диагнозом. Для коррекции курса проконсультируйтесь с врачом.

📌 Инструкция:
- Если показатели в норме → ставь ✅
- Если есть отклонения → ставь ⚠️ или ❌
- Учитывай влияние препаратов на показатели
- 📈 - положительное влияние на спорт
- 📉 - негативное влияние на спорт
- Если данных нет → пиши "Данные не предоставлены"

Информация о пациенте и препаратах: %s

Данные анализов пользователя:
%s`, courseInfo, text)
}

func extractCourseInfo(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
			var courseInfo strings.Builder
			for j := i + 1; j < len(lines) && j < i+20; j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if strings.Contains(lines[j], "•") {
					courseInfo.WriteString(strings.TrimSpace(lines[j]) + "\n")
				}
			}
			return courseInfo.String()
		}
	}
	return ""
}

func rateLimitFallback() string {
	log.Printf("⚠️ Returning rate-limit fallback response")
	return `⏳ Сервис временно перегружен

📊 В данный момент я не могу обработать ваш запрос, так как достигнут лимит запросов к сервису искусственного интеллекта.

🔄 Что делать:
• Подождите 1-2 минуты
• Отправьте анализ повторно

💡 Бесплатный тариф имеет ограничение на количество запросов в минуту.

⏰ Обычно лимит восстанавливается через 1-2 минуты. Попробуйте снова через несколько минут.`
}

func locationErrorFallback() string {
	log.Printf("⚠️ Returning location error fallback response")
	return `🌍 Сервис временно недоступен в вашем регионе

Google Gemini API может быть недоступен в некоторых странах.

🔄 Что делать:
• Используйте VPN для доступа к сервису
• Попробуйте позже
• Обратитесь к администратору бота

⏰ В ближайшее время мы работаем над решением этой проблемы.`
}

func noKeyFallback() string {
	log.Printf("⚠️ Returning no-key fallback response")
	return `❌ AI не настроен

🔑 В текущем окружении не настроен ключ для доступа к искусственному интеллекту.

🛠️ Что делать:
• Обратитесь к администратору бота
• Убедитесь, что в настройках указан GOOGLE_GEMINI_API_KEY

⏳ Как только ключ будет добавлен, бот снова сможет обрабатывать анализы.`
}

func serviceUnavailableFallback() string {
	log.Printf("⚠️ Returning service-unavailable fallback response")
	return `🔧 Сервис временно недоступен

🌐 В данный момент сервис искусственного интеллекта не отвечает.

🔄 Что делать:
• Проверьте интернет-соединение
• Подождите несколько минут
• Попробуйте отправить анализ повторно

⏰ Если проблема сохраняется, попробуйте позже.`
}

func (c *GeminiClient) GenerateBioscanJSON(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock bioscan JSON response")
		return getMockBioscanJSON(contextInfo), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("empty gemini api key")
	}

	prompt := `
Ты анализируешь фото тела для фитнес отчёта.

Верни ТОЛЬКО JSON.
Без markdown.
Без комментариев.

Формат:

{
"score":85,
"level":"Хорошая форма",
"summary":"описание",

"body":{
"height":"",
"weight":"",
"muscle_mass":"",
"fat":""
},

"composition":"",

"profile":{
"composition":80,
"muscle_development":75,
"balance":85,
"potential":90
},

"zones":[
{
"name":"",
"score":80,
"status":"",
"description":""
}
],

"muscles":[
{
"name":"",
"level":"",
"assessment":"",
"symmetry":"",
"recommendation":""
}
],

"posture":{
"type":"",
"head":"",
"shoulders":"",
"pelvis":"",
"description":""
},

"attention_zones":[
{
"name":"",
"problem":"",
"solution":""
}
],

"priorities":[
{
"title":"",
"description":""
}
],

"training_days":[
{
"day":"",
"exercises":[
{
"name":"",
"sets":"",
"reps":""
}
]
}
],

"nutrition":[],
"recovery":[],

"progress":{
"recheck":"",
"targets":[]
}

}

Данные пользователя:

` + contextInfo

	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{
			Text: prompt,
		},
	}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

type geminiRequest struct {
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Contents          []geminiContent         `json:"contents"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func extractGeminiText(resp *geminiResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}

	var parts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func normalizeAIResponse(text string) string {
	text = strings.TrimSpace(text)

	if !strings.Contains(text, "📊") {
		text = "📊 РЕЗУЛЬТАТЫ АНАЛИЗОВ\n\n" + text
	}

	if !strings.Contains(text, "📋 ЗАКЛЮЧЕНИЕ") {
		text += "\n\n📋 ЗАКЛЮЧЕНИЕ\n\nАнализ выполнен на основе предоставленных данных. Для получения детальной интерпретации обратитесь к врачу."
	}

	if !strings.Contains(text, "⚠️ ВНИМАНИЕ") {
		text += "\n\n⚠️ ВНИМАНИЕ: Данный анализ носит информационный характер и не заменяет консультацию врача. Для постановки диагноза обратитесь к специалисту."
	}

	text = strings.ReplaceAll(text, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━", "")
	text = strings.ReplaceAll(text, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "**", "")

	lines := strings.Split(text, "\n")
	var result []string
	var lastEmpty bool
	for _, line := range lines {
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty && lastEmpty {
			continue
		}
		result = append(result, line)
		lastEmpty = isEmpty
	}

	return strings.Join(result, "\n")
}

func (c *GeminiClient) generateRaw(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf("🔄 Starting Gemini RAW request...")

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: "Ты должен вернуть только JSON. Без markdown. Без комментариев.",
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 4000,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini error: %s", string(respBody))
	}

	var result geminiResponse
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return "", err
	}

	text := extractGeminiText(&result)
	log.Printf("📝 RAW JSON response length: %d", len(text))

	cleanedText := strings.TrimSpace(text)
	cleanedText = strings.TrimPrefix(cleanedText, "```json")
	cleanedText = strings.TrimPrefix(cleanedText, "```")
	cleanedText = strings.TrimSuffix(cleanedText, "```")
	cleanedText = strings.TrimSpace(cleanedText)

	return cleanedText, nil
}

func normalizeJSONResponse(text string) string {
	text = strings.TrimSpace(text)

	text = strings.ReplaceAll(text, "```json", "")
	text = strings.ReplaceAll(text, "```", "")

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	return strings.TrimSpace(text)
}

func (c *GeminiClient) GenerateAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error) {
	log.Printf("📤 GenerateAnalysisFromFile called with mimeType: %s, data size: %d bytes", mimeType, len(data))

	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock response for file analysis")
		return getMockAnalysisFromFileData(data, mimeType, "Содержимое загруженного документа с медицинскими анализами"), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		log.Printf("❌ API key is empty or only whitespace")
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: c.buildPromptForRegular("Содержимое загруженного документа с медицинскими анализами во вложении.")},
	}

	return c.generate(ctx, parts)
}

func (c *GeminiClient) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock response for file analysis with context")

		if strings.Contains(contextText, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") &&
			(strings.Contains(contextText, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") ||
				strings.Contains(contextText, "Вид спорта") ||
				strings.Contains(contextText, "Стаж тренировок")) {
			return getMockAnalysisWithContext(contextText), nil
		}

		return getMockAnalysisFromFileData(data, mimeType, contextText), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	var prompt string
	if strings.Contains(contextText, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		if strings.Contains(contextText, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") ||
			strings.Contains(contextText, "Вид спорта") ||
			strings.Contains(contextText, "Стаж тренировок") {
			courseInfo := extractCourseInfo(contextText)
			if courseInfo != "" {
				prompt = c.buildPromptForAthlete(contextText, courseInfo)
				log.Printf("🏋️ Athlete mode: on course")
			} else {
				prompt = c.buildPromptForRegular(contextText)
				log.Printf("👤 Regular mode")
			}
		} else {
			prompt = c.buildPromptForRegular(contextText)
			log.Printf("👤 Regular mode")
		}
	} else {
		prompt = c.buildPromptForRegular(contextText)
		log.Printf("👤 Regular mode")
	}

	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: prompt},
	}

	return c.generate(ctx, parts)
}

func getMockAnalysisFromFileData(data []byte, mimeType string, contextText string) string {
	if !strings.Contains(mimeType, "text") && len(data) > 0 {
		return getMockAnalysisFromData(contextText)
	}

	content := string(data)
	return getMockAnalysisFromData(content)
}

func (c *GeminiClient) generate(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf("🔄 Starting Gemini API request...")

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: "Ты опытный врач-диагност. Отвечай строго по формату, без лишнего текста.",
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.2,
			MaxOutputTokens: 3000,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal payload: %v", err)
		return "", err
	}

	log.Printf("📦 Request body size: %d bytes", len(body))

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	if len(c.apiKey) > 10 {
		loggedURL := fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s...%s",
			c.model,
			c.apiKey[:10],
			c.apiKey[len(c.apiKey)-4:],
		)
		log.Printf("🌐 Request URL: %s", loggedURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("❌ Failed to create request: %v", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("⏳ Sending request to Gemini...")
	startTime := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP request failed: %v", err)
		return serviceUnavailableFallback(), nil
	}
	defer resp.Body.Close()

	elapsed := time.Since(startTime)
	log.Printf("⏱️ Request completed in %v", elapsed)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Failed to read response body: %v", err)
		return "", err
	}

	log.Printf("📥 Response status: %d", resp.StatusCode)
	if len(respBody) > 500 {
		log.Printf("📄 Response body (first 500 chars): %s...", string(respBody[:500]))
	} else {
		log.Printf("📄 Response body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Non-OK status: %d", resp.StatusCode)

		if resp.StatusCode == 429 {
			return rateLimitFallback(), nil
		}

		if resp.StatusCode == 400 {
			var errResp struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Status  string `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal(respBody, &errResp); err == nil {
				if strings.Contains(errResp.Error.Message, "location is not supported") {
					return locationErrorFallback(), nil
				}
			}
			return serviceUnavailableFallback(), nil
		}

		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}

		if err := json.Unmarshal(respBody, &errResp); err == nil {
			log.Printf("📋 Gemini error details: Code=%d, Message=%s, Status=%s",
				errResp.Error.Code,
				errResp.Error.Message,
				errResp.Error.Status)

			if errResp.Error.Code == 429 || errResp.Error.Code == 401 || errResp.Error.Code == 403 || errResp.Error.Code == 500 {
				return serviceUnavailableFallback(), nil
			}

			return "", fmt.Errorf("gemini error %d: %s", errResp.Error.Code, errResp.Error.Message)
		}

		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 500 {
			return serviceUnavailableFallback(), nil
		}
		return "", fmt.Errorf("gemini request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("❌ Failed to unmarshal response: %v", err)
		return "", err
	}

	if result.Error != nil {
		log.Printf("❌ Gemini returned error: Code=%d, Message=%s", result.Error.Code, result.Error.Message)
		if result.Error.Code == 429 || result.Error.Code == 401 || result.Error.Code == 403 || result.Error.Code == 500 {
			return serviceUnavailableFallback(), nil
		}
		return "", fmt.Errorf("gemini error: %s", result.Error.Message)
	}

	text := extractGeminiText(&result)
	log.Printf("📝 Extracted text length: %d characters", len(text))

	if text == "" {
		log.Printf("⚠️ Empty response from Gemini")
		log.Printf("🔍 Full response: %s", string(respBody))
		return getMockAnalysis(""), nil
	}

	log.Printf("✅ Successfully generated response from Gemini")
	return normalizeAIResponse(text), nil
}

// ============================================
// ГЕНЕРАЦИЯ JSON ДЛЯ АНАЛИЗОВ
// ============================================

func (c *GeminiClient) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock analysis JSON response")
		return getMockAnalysisJSON(userInput), nil
	}

	prompt := c.buildAnalysisJSONPrompt(userInput)
	parts := []geminiPart{{Text: prompt}}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

func (c *GeminiClient) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if strings.Contains(c.apiKey, "mock") || c.apiKey == "" {
		log.Printf("🧪 Using mock analysis JSON response from file")
		return getMockAnalysisJSON(contextText), nil
	}

	prompt := c.buildAnalysisJSONPrompt(contextText)
	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: prompt},
	}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

func (c *GeminiClient) buildAnalysisJSONPrompt(text string) string {
	return `Ты опытный врач-диагност. Проанализируй медицинские показатели и верни ТОЛЬКО JSON.

Верни ТОЛЬКО JSON.
Без markdown.
Без комментариев.
Без текста до или после JSON.

Формат JSON:
{
  "profile": {
    "name": "Пациент",
    "date": "2026-08-06",
    "age": 35,
    "gender": "мужской"
  },
  "categories": [
    {
      "name": "Общий анализ крови",
      "description": "Описание категории",
      "indicators": [
        {
          "name": "Гемоглобин",
          "value": "145",
          "unit": "г/л",
          "normal": "120-160",
          "status": "normal",
          "description": "Краткое описание статуса",
          "explanation": "Краткое клиническое объяснение показателя.",
          "risk": "low",
          "recommendation": "Персонализированная рекомендация",
          "role": "Белок-переносчик кислорода",
          "short_desc": "Основной белок эритроцитов",
          "full_desc": "Полное описание роли показателя в организме.",
          "function": "Транспорт кислорода"
        }
      ]
    }
  ],
  "summary": "Краткое общее заключение по всем анализам",
  "attention": ["Пункт внимания 1", "Пункт внимания 2"],
  "recommendations": ["Рекомендация 1", "Рекомендация 2"],
  "disclaimer": "Этот анализ носит информационный характер и не является диагнозом."
}

КРИТИЧЕСКИ ВАЖНЫЕ ПРАВИЛА И ОГРАНИЧЕНИЯ ДЛИНЫ:
1. Будь предельно лаконичен! Избегай длинных текстов, чтобы JSON не превысил лимит токенов.
2. Поле "explanation": НЕ БОЛЕЕ 1-2 кратких предложений (до 20 слов). Главная суть и значение.
3. Поле "full_desc": НЕ БОЛЕЕ 1-2 кратких предложений (до 20 слов).
4. Поле "recommendation": НЕ БОЛЕЕ 1 краткого предложения (до 15 слов).
5. Поле "role": кратко (1-3 слова).
6. Поле "short_desc": одно короткое предложение (до 10 слов).
7. status: строго одно из values ["normal", "warning", "critical"]
8. risk: строго одно из values ["low", "medium", "high"]
9. Если данных нет, ставь "Данные не предоставлены".
10. Не придумывай показатели, которых нет в тексте.

Данные анализов пользователя:
` + text
}

func getMockAnalysisJSON(input string) string {
	return `{
  "profile": {
    "name": "Тестовый пациент",
    "date": "2026-08-06",
    "age": 35,
    "gender": "мужской"
  },
  "categories": [
    {
      "name": "Общий анализ крови",
      "description": "Базовое исследование, оценивающее количество и качество клеток крови. Позволяет выявить анемию, воспалительные процессы и нарушения иммунитета.",
      "indicators": [
        {
          "name": "Гемоглобин",
          "value": "145",
          "unit": "г/л",
          "normal": "120-160",
          "status": "normal",
          "description": "Показатель в норме, соответствует референсным значениям.",
          "explanation": "Гемоглобин — это белок, который содержится в эритроцитах и отвечает за транспорт кислорода от лёгких к тканям и углекислого газа обратно. Ваш уровень 145 г/л находится в оптимальном диапазоне, что говорит о хорошей кислородно-транспортной функции крови. Это важно для энергии, выносливости и общего самочувствия. Нормальный уровень гемоглобина также свидетельствует о здоровом костном мозге и достаточном количестве железа в организме.",
          "risk": "low",
          "recommendation": "Поддерживайте уровень железа в организме: употребляйте красное мясо, печень, яйца, зелень. Продолжайте вести активный образ жизни.",
          "role": "Белок-переносчик кислорода",
          "short_desc": "Основной белок эритроцитов, обеспечивающий транспорт кислорода",
          "full_desc": "Гемоглобин — сложный железосодержащий белок, который составляет основную массу эритроцитов. Он обеспечивает связывание кислорода в легких и его доставку к тканям, а также выведение углекислого газа. Уровень гемоглобина является ключевым показателем для диагностики анемии и оценки общего состояния кроветворной системы.",
          "function": "Транспорт кислорода и углекислого газа"
        },
        {
          "name": "Эритроциты",
          "value": "4.8",
          "unit": "×10^12/л",
          "normal": "4.0-5.5",
          "status": "normal",
          "description": "Количество эритроцитов в норме.",
          "explanation": "Эритроциты — красные кровяные клетки, которые содержат гемоглобин. Их основная функция — перенос кислорода. Ваш показатель 4.8×10^12/л находится в пределах нормы, что обеспечивает адекватное снабжение тканей кислородом. Это также косвенно указывает на здоровый уровень эритропоэтина и нормальную работу костного мозга.",
          "risk": "low",
          "recommendation": "Для поддержания здоровья эритроцитов следите за уровнем железа и витамина B12 в питании.",
          "role": "Красные кровяные клетки",
          "short_desc": "Клетки крови, содержащие гемоглобин",
          "full_desc": "Эритроциты — наиболее многочисленные клетки крови, основная функция которых заключается в транспорте кислорода от легких к тканям организма. Они имеют форму двояковогнутого диска, что увеличивает их площадь поверхности для газообмена. Продолжительность жизни эритроцита составляет около 120 дней.",
          "function": "Транспорт кислорода и газообмен"
        },
        {
          "name": "Лейкоциты",
          "value": "11.5",
          "unit": "×10^9/л",
          "normal": "4.0-9.0",
          "status": "warning",
          "description": "Уровень лейкоцитов повышен, что может указывать на воспалительный процесс.",
          "explanation": "Лейкоциты — белые кровяные клетки, которые являются частью иммунной системы. Повышенный уровень (11.5×10^9/л) может свидетельствовать о наличии инфекции, воспаления или стрессовой реакции организма. Также это может быть реакцией на физическую нагрузку или прием некоторых лекарственных препаратов. Рекомендуется дополнительное обследование для выявления причины.",
          "risk": "medium",
          "recommendation": "Рекомендуется обратиться к терапевту для выяснения причины повышения лейкоцитов. Возможно потребуется дополнительный анализ крови с лейкоцитарной формулой.",
          "role": "Иммунные клетки",
          "short_desc": "Клетки иммунной системы, защищающие организм",
          "full_desc": "Лейкоциты — это гетерогенная группа клеток, выполняющих защитную функцию в организме. Они распознают и уничтожают чужеродные агенты, участвуют в воспалительных реакциях и иммунном ответе. Повышение уровня лейкоцитов (лейкоцитоз) обычно указывает на активный воспалительный или инфекционный процесс.",
          "function": "Иммунная защита организма"
        },
        {
          "name": "Тромбоциты",
          "value": "250",
          "unit": "×10^9/л",
          "normal": "180-320",
          "status": "normal",
          "description": "Количество тромбоцитов в норме.",
          "explanation": "Тромбоциты — клетки крови, отвечающие за свертываемость и остановку кровотечений. Ваш показатель 250×10^9/л находится в норме, что говорит о хорошей свёртывающей способности крови. Это важно для профилактики как кровотечений, так и тромбообразования.",
          "risk": "low",
          "recommendation": "Для здоровья тромбоцитов важно получать достаточно витамина K (зеленые овощи) и поддерживать водный баланс.",
          "role": "Клетки свертывания",
          "short_desc": "Клетки крови, отвечающие за свертываемость",
          "full_desc": "Тромбоциты — это безъядерные клетки, которые играют ключевую роль в гемостазе — процессе остановки кровотечения. Они активируются при повреждении сосуда, образуют тромбоцитарную пробку и выделяют факторы, активирующие систему свертывания крови. Нормальное количество тромбоцитов обеспечивает баланс между свертываемостью и текучестью крови.",
          "function": "Остановка кровотечений и свертывание"
        }
      ]
    },
    {
      "name": "Биохимический анализ крови",
      "description": "Исследование, оценивающее функциональное состояние внутренних органов и систем организма. Позволяет выявить нарушения обмена веществ, работы печени, почек и поджелудочной железы.",
      "indicators": [
        {
          "name": "Глюкоза",
          "value": "6.8",
          "unit": "ммоль/л",
          "normal": "3.3-5.5",
          "status": "critical",
          "description": "Уровень глюкозы значительно повышен, что может указывать на нарушение углеводного обмена.",
          "explanation": "Глюкоза — основной источник энергии для клеток организма. Повышенный уровень (6.8 ммоль/л) может свидетельствовать о развитии преддиабета или сахарного диабета. Также повышение глюкозы может быть вызвано стрессом, приемом некоторых лекарств или гормональными нарушениями. Требуется дополнительное обследование, включая тест на толерантность к глюкозе и анализ на гликированный гемоглобин (HbA1c).",
          "risk": "high",
          "recommendation": "НЕОБХОДИМО СРОЧНО ОБРАТИТЬСЯ К ВРАЧУ-ЭНДОКРИНОЛОГУ! До консультации исключите простые углеводы из рациона, увеличьте физическую активность.",
          "role": "Уровень сахара в крови",
          "short_desc": "Основной источник энергии для клеток",
          "full_desc": "Глюкоза — простой углевод, который является универсальным источником энергии для всех клеток организма. Уровень глюкозы в крови регулируется инсулином и глюкагоном. Хроническое повышение глюкозы ведет к повреждению сосудов, нервов и органов, развитию диабетических осложнений.",
          "function": "Энергетический обмен"
        }
      ]
    }
  ],
  "summary": "По результатам анализов выявлены значительные нарушения: повышенный уровень глюкозы (критично) указывает на высокий риск сахарного диабета; повышенные лейкоциты свидетельствуют о воспалительных процессах. Требуется срочная консультация врача.",
  "attention": [
    "🔴 Критическое повышение глюкозы (6.8 ммоль/л) — срочно обратитесь к эндокринологу!",
    "⚠️ Повышенный уровень лейкоцитов (11.5×10^9/л) — требуется дополнительное обследование"
  ],
  "recommendations": [
    "🔴 СРОЧНО обратиться к врачу-эндокринологу для диагностики и лечения",
    "Исключить из рациона: сахар, сладости, выпечку, белый хлеб",
    "Добавить в рацион: овощи, фрукты (кроме сладких), цельнозерновые продукты",
    "Физическая активность: ежедневные прогулки не менее 30 минут",
    "Повторный анализ крови через 2 недели для контроля динамики"
  ],
  "disclaimer": "Анализ выполнен на основе предоставленных данных и носит информационный характер. Выявлены критические отклонения, требующие неотложного медицинского вмешательства. Данный отчет НЕ ЗАМЕНЯЕТ консультацию врача."
}`
}
