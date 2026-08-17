package locales

// Константы, извлечённые из internal/bot/handlers/helpers/*.go
// (файлы helpers_analysis.go, helpers.go, helpers_download.go).
// Содержат user-facing строки, формируемые в BuildAnalysisText.

const (
	// MsgAnalysisPatient - заголовок блока пациента. Параметр: имя.
	MsgAnalysisPatient = "👤 Пациент: %s"

	// MsgAnalysisImportantInfo - заголовок блока важной информации для анализа.
	MsgAnalysisImportantInfo = "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:"

	// MsgAnalysisGender - пол пациента. Параметр: значение.
	MsgAnalysisGender = "• Пол: %s"

	// MsgAnalysisAge - возраст пациента. Параметр: значение.
	MsgAnalysisAge = "• Возраст: %s лет"

	// MsgAnalysisHeight - рост пациента. Параметр: значение.
	MsgAnalysisHeight = "• Рост: %s см"

	// MsgAnalysisWeight - вес пациента. Параметр: значение.
	MsgAnalysisWeight = "• Вес: %s кг"

	// MsgAnalysisBMI - индекс массы тела (ИМТ). Параметр: значение (float).
	MsgAnalysisBMI = "• ИМТ: %.1f"

	// MsgAnalysisChronic - хронические заболевания. Параметр: значение.
	MsgAnalysisChronic = "• Хронические заболевания: %s"

	// MsgAnalysisAllergies - аллергии. Параметр: значение.
	MsgAnalysisAllergies = "• Аллергии: %s"

	// MsgAnalysisMedications - принимаемые лекарства. Параметр: значение.
	MsgAnalysisMedications = "• Принимаемые лекарства: %s"

	// MsgAnalysisSmoking - курение. Параметр: значение.
	MsgAnalysisSmoking = "• Курение: %s"

	// MsgAnalysisAlcohol - употребление алкоголя. Параметр: значение.
	MsgAnalysisAlcohol = "• Алкоголь: %s"

	// MsgAnalysisSportType - вид спорта. Параметр: значение.
	MsgAnalysisSportType = "• Вид спорта: %s"

	// MsgAnalysisTrainingExp - стаж тренировок. Параметр: значение.
	MsgAnalysisTrainingExp = "• Стаж тренировок: %s лет"

	// MsgAnalysisGoal - цель. Параметр: значение.
	MsgAnalysisGoal = "• Цель: %s"

	// MsgAnalysisSleep - сон. Параметр: значение.
	MsgAnalysisSleep = "• Сон: %s"

	// MsgAnalysisStress - уровень стресса. Параметр: значение.
	MsgAnalysisStress = "• Уровень стресса: %s"

	// MsgAnalysisNutritionVeg - овощи/фрукты. Параметр: значение.
	MsgAnalysisNutritionVeg = "• Овощи/фрукты: %s"

	// MsgAnalysisNutritionProcessed - ультраобработанные продукты. Параметр: значение.
	MsgAnalysisNutritionProcessed = "• Ультраобработанные продукты: %s"

	// MsgAnalysisWater - питьевой режим. Параметр: значение.
	MsgAnalysisWater = "• Питьевой режим: %s"

	// MsgAnalysisActivity - физическая активность. Параметр: значение.
	MsgAnalysisActivity = "• Физическая активность: %s"

	// MsgAnalysisFamilyHistory - семейный анамнез. Параметр: значение.
	MsgAnalysisFamilyHistory = "• Семейный анамнез: %s"

	// MsgAnalysisDigestion - ЖКТ / пищеварение. Параметр: значение.
	MsgAnalysisDigestion = "• ЖКТ / пищеварение: %s"

	// MsgAnalysisCourseInfo - используемые препараты (инфо указано). Параметр: значение.
	MsgAnalysisCourseInfo = "• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ: %s"

	// MsgAnalysisCourseNoInfo - использует препараты, но информация не указана.
	MsgAnalysisCourseNoInfo = "• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ (информация не указана)"

	// MsgAnalysisCourseInterpret - требуется интерпретация с учётом приёма препаратов.
	MsgAnalysisCourseInterpret = "• Требуется интерпретация с учетом приема препаратов"

	// MsgAnalysisCourseHormonal - оценить влияние на гормональный фон и показатели.
	MsgAnalysisCourseHormonal = "• Оценить влияние на гормональный фон и показатели"

	// MsgAnalysisNoCourse - без препаратов (естественный фон).
	MsgAnalysisNoCourse = "• Без препаратов (естественный фон)"
)
