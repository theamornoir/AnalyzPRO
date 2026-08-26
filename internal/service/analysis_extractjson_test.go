package service

import "testing"

func TestExtractJSONStripsFencesAndPreamble(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "cyrillic preamble + json",
			in:   "Вот ваш отчёт:\n{\"score\":82,\"level\":\"Хорошая форма\"}",
			want: "{\"score\":82,\"level\":\"Хорошая форма\"}",
		},
		{
			name: "markdown fence json",
			in:   "```json\n{\"a\":1}\n```",
			want: "{\"a\":1}",
		},
		{
			name: "plain fence + trailing text",
			in:   "```\n{\"a\":1}\n```\nНадеюсь, помогло",
			want: "{\"a\":1}",
		},
		{
			name: "BOM prefix",
			in:   "\uFEFF{\"a\":1}",
			want: "{\"a\":1}",
		},
		{
			name: "clean json",
			in:   "{\"a\":1}",
			want: "{\"a\":1}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractJSON(c.in)
			if got != c.want {
				t.Fatalf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
