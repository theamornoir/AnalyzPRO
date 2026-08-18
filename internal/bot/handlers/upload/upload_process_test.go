package upload

import "testing"

func TestExtractIndicatorGroupsStripsMarkdown(t *testing.T) {
	wrapped := "```json\n{\"title\":\"Анализ\",\"sections\":[{\"title\":\"Кровь\",\"indicators\":[{\"name\":\"Гемоглобин\",\"value\":\"145\",\"unit\":\"г/л\",\"status\":\"normal\"}]}]}\n```"

	groups := extractIndicatorGroups(wrapped)
	if groups == nil {
		t.Fatalf("ожидались извлечённые группы, получен nil (markdown не снят)")
	}
	sections, ok := groups["sections"]
	if !ok || sections == nil {
		t.Fatalf("ключ sections отсутствует в результате: %#v", groups)
	}

	plain := "{\"sections\":[{\"title\":\"Кровь\",\"indicators\":[{\"name\":\"Гемоглобин\",\"value\":\"145\"}]}]}"
	if g := extractIndicatorGroups(plain); g == nil {
		t.Fatalf("чистый JSON не извлёкся")
	}

	if g := extractIndicatorGroups(""); g != nil {
		t.Fatalf("пустая строка должна давать nil, получен %#v", g)
	}

	prose := "Вот результат:\n{\"sections\":[{\"title\":\"Кровь\",\"indicators\":[{\"name\":\"IgG\",\"value\":\"12.5\"}]}]}\nСпасибо."
	if g := extractIndicatorGroups(prose); g == nil {
		t.Fatalf("JSON с текстом вокруг не извлёкся")
	}
}
