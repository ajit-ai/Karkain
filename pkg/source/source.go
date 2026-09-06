// Package source provides first-class, byte-precise source positions for the
// Karkain front end: spans, a lazy line index (byte offset <-> 1-based line /
// 0-based byte column), UTF-16 column conversion for the LSP, and diagnostic
// excerpt helpers. It is consumed by the parser, the check pipeline, and the
// language server so that CLI and LSP diagnostics share one position model.
package source

// Span is a half-open byte range [Start, End) within one source text.
type Span struct {
	Start int
	End   int
}

// Position is a 1-based line with a 0-based byte column within that line.
// Columns are byte offsets (not rune or UTF-16 units); use LineIndex.UTF16Col
// when a UTF-16 column (LSP) is needed.
type Position struct {
	Line int
	Col  int
}

// LineIndex maps byte offsets to line/column coordinates for one source text.
// It supports LF, CRLF, and lone CR line endings and never rescans the whole
// text per query: line-start offsets are computed once.
type LineIndex struct {
	text      string
	lineStart []int // byte offset of the first byte of each line (line i is lineStart[i])
}

// NewLineIndex builds a line index over text. line numbering is 1-based in the
// index: lineStart[0] is line 1.
func NewLineIndex(text string) *LineIndex {
	li := &LineIndex{text: text, lineStart: []int{0}}
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\n':
			li.lineStart = append(li.lineStart, i+1)
		case '\r':
			// Fold \r\n into a single newline; a lone \r is also a newline.
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			li.lineStart = append(li.lineStart, i+1)
		}
	}
	return li
}

// Text returns the indexed source text.
func (li *LineIndex) Text() string { return li.text }

// LineCount returns the number of lines (>= 1).
func (li *LineIndex) LineCount() int { return len(li.lineStart) }

// LineStart returns the byte offset of the first byte of the given 1-based line.
func (li *LineIndex) LineStart(line int) int {
	if line < 1 {
		line = 1
	}
	if line > len(li.lineStart) {
		line = len(li.lineStart)
	}
	return li.lineStart[line-1]
}

// LineEnd returns the byte offset just past the last content byte of the given
// 1-based line, excluding the terminating newline (bytes up to and including
// the final \n are excluded; a \r\n counts as two excluded bytes).
func (li *LineIndex) LineEnd(line int) int {
	if line < 1 {
		line = 1
	}
	if line >= len(li.lineStart) {
		return len(li.text)
	}
	end := li.lineStart[line]
	for end > li.lineStart[line-1] {
		if li.text[end-1] == '\n' || li.text[end-1] == '\r' {
			end--
			continue
		}
		break
	}
	return end
}

// LineText returns the content of the given 1-based line without its newline.
func (li *LineIndex) LineText(line int) string {
	return li.text[li.LineStart(line):li.LineEnd(line)]
}

// PositionAt converts a byte offset into a 1-based line and a 0-based byte
// column within that line. The offset is clamped into [0, len(text)].
func (li *LineIndex) PositionAt(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(li.text) {
		offset = len(li.text)
	}
	lo, hi := 0, len(li.lineStart)
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if li.lineStart[mid] <= offset {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Position{Line: lo + 1, Col: offset - li.lineStart[lo]}
}

// Offset converts a 1-based line and a 0-based byte column into a byte offset.
// Out-of-range coordinates clamp to the nearest valid offset.
func (li *LineIndex) Offset(line, col int) int {
	if line < 1 {
		line = 1
	}
	start := li.LineStart(line)
	if line >= len(li.lineStart) {
		end := len(li.text)
		if col < 0 {
			col = 0
		}
		if start+col > end {
			return end
		}
		return start + col
	}
	end := li.lineStart[line]
	if col < 0 {
		col = 0
	}
	if start+col > end {
		return end
	}
	return start + col
}

// UTF16Col returns the UTF-16 code-unit column (LSP convention) of a byte
// offset within its line: the number of UTF-16 code units on the line up to the
// offset. Offsets in the middle of a multi-byte UTF-8 sequence count up to the
// last complete code point before offset.
func (li *LineIndex) UTF16Col(offset int) int {
	pos := li.PositionAt(offset)
	if pos.Col == 0 {
		return 0
	}
	line := li.LineText(pos.Line)
	unit := 0
	for i := 0; i < len(line) && i < pos.Col; {
		r, size := decodeRune(line[i:])
		i += size
		if r >= 0x10000 {
			unit += 2
		} else {
			unit++
		}
	}
	return unit
}

// decodeRune decodes the first UTF-8 rune of s and returns it with its width.
// Invalid bytes decode as the byte value with width 1.
func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	if b < 0x80 {
		return rune(b), 1
	}
	need := 0
	switch {
	case b&0xE0 == 0xC0:
		need = 2
	case b&0xF0 == 0xE0:
		need = 3
	case b&0xF8 == 0xF0:
		need = 4
	default:
		return rune(b), 1
	}
	if len(s) < need {
		return rune(b), 1
	}
	r := rune(b)
	mask := byte(0xFF >> (need + 1))
	r = rune(b&mask) << (6 * (need - 1))
	for i := 1; i < need; i++ {
		if s[i]&0xC0 != 0x80 {
			return rune(b), 1
		}
		r |= rune(s[i]&0x3F) << (6 * (need - 1 - i))
	}
	return r, need
}

// UTF16Width returns the UTF-16 code-unit count of a string (LSP column width).
func UTF16Width(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x10000 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// Excerpt returns a single-line excerpt of text around line (1-based) for
// diagnostics: the line's content, trimmed of leading/trailing whitespace and
// limited to max runes. It returns "" for empty or out-of-range lines.
func Excerpt(text string, line, max int) string {
	if max <= 0 {
		return ""
	}
	li := NewLineIndex(text)
	if line < 1 || line > li.LineCount() {
		return ""
	}
	s := trimSpace(li.LineText(line))
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max]) + "..."
	}
	return string(runes)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\v' || b == '\f'
}
