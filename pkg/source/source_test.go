package source

import "testing"

func TestLineIndexPositionAt(t *testing.T) {
	li := NewLineIndex("let a\nlet b")
	if li.LineCount() != 2 {
		t.Fatalf("LineCount = %d, want 2", li.LineCount())
	}
	cases := []struct {
		offset int
		line   int
		col    int
	}{
		{0, 1, 0},
		{3, 1, 3},
		{5, 1, 5},
		{6, 2, 0},
		{10, 2, 4},
	}
	for _, c := range cases {
		got := li.PositionAt(c.offset)
		if got.Line != c.line || got.Col != c.col {
			t.Errorf("PositionAt(%d) = line %d col %d, want line %d col %d", c.offset, got.Line, got.Col, c.line, c.col)
		}
	}
	// Clamping
	if p := li.PositionAt(999); p.Line != 2 || p.Col != 5 {
		t.Errorf("clamped offset = line %d col %d, want line 2 col 5", p.Line, p.Col)
	}
	if p := li.PositionAt(-1); p.Line != 1 || p.Col != 0 {
		t.Errorf("negative offset = line %d col %d, want line 1 col 0", p.Line, p.Col)
	}
}

func TestLineIndexRoundTrip(t *testing.T) {
	li := NewLineIndex("a\r\nb\r\nc")
	if li.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3", li.LineCount())
	}
	if li.LineText(2) != "b" {
		t.Errorf("LineText(2) = %q, want b", li.LineText(2))
	}
	for line := 1; line <= li.LineCount(); line++ {
		for col := 0; col <= len(li.LineText(line)); col++ {
			off := li.Offset(line, col)
			got := li.PositionAt(off)
			if got.Line != line || got.Col != col {
				t.Errorf("round trip line %d col %d -> offset %d -> line %d col %d", line, col, off, got.Line, got.Col)
			}
		}
	}
}

func TestLineTextAndLineEnd(t *testing.T) {
	li := NewLineIndex("ab\ncd\r\nef\r")
	if li.LineText(1) != "ab" {
		t.Errorf("LineText(1) = %q", li.LineText(1))
	}
	if li.LineText(2) != "cd" {
		t.Errorf("LineText(2) = %q", li.LineText(2))
	}
	if li.LineText(3) != "ef" {
		t.Errorf("LineText(3) = %q", li.LineText(3))
	}
	// Point just after "c" (col 1) is at offset 4 (after \n of line 1).
	if off := li.Offset(2, 1); off != 4 {
		t.Errorf("Offset(2,1) = %d, want 4", off)
	}
}

func TestLineIndexUTF16Col(t *testing.T) {
	text := "héllo"
	li := NewLineIndex(text)
	// 'h' 0-1, 'é' 1-3 (two bytes), 'l' 3-4 ...
	if got := li.UTF16Col(0); got != 0 {
		t.Errorf("UTF16Col(0) = %d, want 0", got)
	}
	li2 := NewLineIndex("𐐀x") // U+10400, one rune but two UTF-16 units
	if got := li2.UTF16Col(0); got != 0 {
		t.Errorf("UTF16Col(0) = %d, want 0", got)
	}
	// After the full astral rune (4 bytes) the UTF-16 column must be 2.
	if got := li2.UTF16Col(4); got != 2 {
		t.Errorf("UTF16Col(4) = %d, want 2", got)
	}
	if got := li2.UTF16Col(5); got != 3 {
		t.Errorf("UTF16Col(5) = %d, want 3", got)
	}
	if UTF16Width("𐐀x") != 3 {
		t.Errorf("UTF16Width = %d, want 3", UTF16Width("𐐀x"))
	}
	if UTF16Width("héllo") != 5 {
		t.Errorf("UTF16Width(héllo) = %d, want 5", UTF16Width("héllo"))
	}
}

func TestExcerpt(t *testing.T) {
	text := "func add(a, b) {\n    return a + b\n}\n"
	if got := Excerpt(text, 2, 40); got != "return a + b" {
		t.Errorf("Excerpt(line 2) = %q, want %q", got, "return a + b")
	}
	if got := Excerpt(text, 2, 8); got != "return a..." {
		t.Errorf("Excerpt(truncated) = %q, want %q", got, "return a...")
	}
	if got := Excerpt(text, 99, 40); got != "" {
		t.Errorf("Excerpt(out of range) = %q, want empty", got)
	}
	if got := Excerpt("", 1, 40); got != "" {
		t.Errorf("Excerpt(empty) = %q, want empty", got)
	}
}
