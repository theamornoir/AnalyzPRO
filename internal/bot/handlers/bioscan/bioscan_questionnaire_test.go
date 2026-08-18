package bioscan

import (
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// TestBioscanQuestionnaireOrder проверяет, что опросник Bioscan PRO состоит из
// 18 вопросов, начинается со стажа тренировок и заканчивается переходом к
// загрузке фото, а каждый вопрос ведёт ровно в следующий (или в Photo1).
func TestBioscanQuestionnaireOrder(t *testing.T) {
	if len(bioscanQuestionnaire) != 18 {
		t.Fatalf("expected 18 bioscan questionnaire questions, got %d", len(bioscanQuestionnaire))
	}
	if bioscanQuestionnaire[0].state != states.StateWaitingBioscanTrainingExp {
		t.Fatalf("first question should be training experience")
	}
	if bioscanQuestionnaire[len(bioscanQuestionnaire)-1].next != states.StateWaitingBioscanPhoto1 {
		t.Fatalf("last question should transition to photo1")
	}
	for i, q := range bioscanQuestionnaire {
		var expected states.State
		if i+1 < len(bioscanQuestionnaire) {
			expected = bioscanQuestionnaire[i+1].state
		} else {
			expected = states.StateWaitingBioscanPhoto1
		}
		if q.next != expected {
			t.Fatalf("question %d (state=%s) next=%s, expected %s", i, q.state, q.next, expected)
		}
	}
}

// TestBuildBioscanText проверяет, что опросник попадает в контекст отчёта
// вместе с базовыми полями (имя/возраст/рост/вес/цель).
func TestBuildBioscanText(t *testing.T) {
	data := map[string]string{
		"bioscan_name":           "Иван",
		"bioscan_age":            "29",
		"bioscan_height":         "180",
		"bioscan_weight":         "78",
		"bioscan_goal":           "рельеф",
		"bioscan_training_exp":   "3 года",
		"bioscan_training_freq":  "4 р/нед",
		"bioscan_injuries":       "боль в колене",
		"bioscan_posture_issues": "сутулость",
	}
	out := BuildBioscanText(data)
	for _, want := range []string{
		"Имя: Иван",
		"Возраст: 29 лет",
		"Рост: 180 см",
		"Вес: 78 кг",
		"Цель: рельеф",
		"Стаж тренировок: 3 года",
		"Травмы и боли: боль в колене",
		"Проблемы с осанкой: сутулость",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("BuildBioscanText missing %q\nGot:\n%s", want, out)
		}
	}
}

// TestBuildBioscanTextSkipsEmpty проверяет, что пустые ответы опросника не
// попадают в контекст (не плодят пустые строки).
func TestBuildBioscanTextSkipsEmpty(t *testing.T) {
	data := map[string]string{
		"bioscan_name":              "Иван",
		"bioscan_smoking":           "",
		"bioscan_diet_restrictions": "Нет",
	}
	out := BuildBioscanText(data)
	if strings.Contains(out, "Курение:") {
		t.Fatalf("empty smoking answer should be skipped, got:\n%s", out)
	}
	if !strings.Contains(out, "Ограничения в питании: Нет") {
		t.Fatalf("non-empty answer should be present, got:\n%s", out)
	}
}
