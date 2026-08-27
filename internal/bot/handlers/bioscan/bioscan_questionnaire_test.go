package bioscan

import (
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// TestBioscanQuestionnaireOrder проверяет, что опросник Bioscan PRO состоит из
// вопросов, идущих ПОСЛЕ цели и ДО загрузки фото (уровень тренированности и
// стаж тренировок), начинается с уровня тренированности и заканчивается
// переходом к загрузке фото, а каждый вопрос ведёт ровно в следующий (или в Photo1).
func TestBioscanQuestionnaireOrder(t *testing.T) {
	if len(bioscanQuestionnaire) == 0 {
		t.Fatalf("bioscan questionnaire must not be empty, got %d", len(bioscanQuestionnaire))
	}
	if bioscanQuestionnaire[0].state != states.StateWaitingBioscanTrainingLevel {
		t.Fatalf("first question should be training level")
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
// вместе с базовыми полями (имя/возраст/рост/вес/цель). Сокращённый
// опросник Bioscan PRO содержит уровень тренированности и стаж тренировок.
func TestBuildBioscanText(t *testing.T) {
	data := map[string]string{
		"bioscan_name":           "Иван",
		"bioscan_age":            "29",
		"bioscan_height":         "180",
		"bioscan_weight":         "78",
		"bioscan_goal":           "рельеф",
		"bioscan_training_level": "новичок",
		"bioscan_training_exp":   "3 года",
	}
	out := BuildBioscanText(data)
	for _, want := range []string{
		"Имя: Иван",
		"Возраст: 29 лет",
		"Рост: 180 см",
		"Вес: 78 кг",
		"Цель: рельеф",
		"Стаж тренировок: 3 года",
		"новичок",
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
		"bioscan_name":         "Иван",
		"bioscan_training_exp": "3 года",
	}
	out := BuildBioscanText(data)
	if !strings.Contains(out, "Стаж тренировок: 3 года") {
		t.Fatalf("non-empty answer should be present, got:\n%s", out)
	}
	// Пустой ответ не должен просочиться в виде пустой строки.
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "•" {
			t.Fatalf("empty answer leaked into output:\n%s", out)
		}
	}
}
