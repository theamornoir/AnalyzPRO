package helpers

import "testing"

func TestSplitLongMessageShort(t *testing.T) {
	got := SplitLongMessage("короткий текст", MaxMessageChunk)
	if len(got) != 1 {
		t.Fatalf("ожидался 1 кусок, получено %d", len(got))
	}
	if got[0] != "короткий текст" {
		t.Fatalf("неверное содержимое куска: %q", got[0])
	}
}

func TestSplitLongMessageEmpty(t *testing.T) {
	if got := SplitLongMessage("", MaxMessageChunk); got != nil {
		t.Fatalf("ожидался nil для пустой строки, получено %v", got)
	}
}

func TestSplitLongMessageChunks(t *testing.T) {
	// ~9001 кириллическая руна (по 2 байта) = 18002 байт.
	// При байтовом лимите MaxMessageChunk (3500) -> 6 кусков
	// (3500 + 3500 + 3500 + 3500 + 3500 + 502 байт).
	long := make([]rune, 0, 9001)
	for i := 0; i < 9001; i++ {
		long = append(long, 'А')
	}
	text := string(long)

	got := SplitLongMessage(text, MaxMessageChunk)
	if len(got) < 2 {
		t.Fatalf("ожидалось >=2 кусков для ~9001 рун, получено %d", len(got))
	}
	for i, c := range got {
		if n := len(c); n > MaxMessageChunk {
			t.Errorf("кусок %d превышает байтовый лимит: %d байт (макс %d)", i, n, MaxMessageChunk)
		}
	}

	// Склейка кусков должна давать исходный текст без потерь.
	joined := ""
	for _, c := range got {
		joined += c
	}
	if joined != text {
		t.Fatalf("склейка кусков не совпадает с исходным текстом")
	}
}
