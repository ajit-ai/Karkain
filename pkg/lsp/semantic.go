package lsp

// Phase 136, Slice A — semantic tokens.
//
// Classification is driven by the Karkain lexer (never re-lexed by hand):
// every emitted token maps to one legend entry, and comments — which the
// lexer skips — are recovered from the gaps between consecutive tokens
// (a gap holding non-whitespace can only be comment text, since the lexer
// consumes all code into tokens). Positions convert the lexer's 1-based
// lines and 0-based byte columns into the LSP 0-based lines and UTF-16
// units, then delta-encode per the protocol.

import (
	"strings"

	"karkain/pkg/lexer"
)

// Legend indices. Order is the contract: SemanticTokenTypeLegend[i] names
// the class encoded as i. Keep in sync with SemanticTokenTypeLegend.
const (
	SemTokNamespace uint32 = iota
	SemTokType
	SemTokFunction
	SemTokVariable
	SemTokKeyword
	SemTokString
	SemTokNumber
	SemTokComment
	SemTokOperator
	SemTokParameter
	SemTokProperty
)

// SemanticTokenTypeLegend is the advertised token vocabulary.
func SemanticTokenTypeLegend() []string {
	return []string{
		"namespace",
		"type",
		"function",
		"variable",
		"keyword",
		"string",
		"number",
		"comment",
		"operator",
		"parameter",
		"property",
	}
}

// SemanticLegend builds the protocol legend for capability advertisement.
func SemanticLegend() SemanticTokensLegend {
	return SemanticTokensLegend{
		TokenTypes:     SemanticTokenTypeLegend(),
		TokenModifiers: []string{},
	}
}

// classifyToken maps a lexer token to its legend index. ok=false means the
// token carries no classification (EOF, illegal input) and is skipped —
// never mislabeled.
func classifyToken(t lexer.TokenType) (typ uint32, ok bool) {
	switch t {
	case lexer.TokenIdent:
		return SemTokVariable, true
	case lexer.TokenInt, lexer.TokenFloat64, lexer.TokenBigInt, lexer.TokenBigFloat:
		return SemTokNumber, true
	case lexer.TokenString:
		return SemTokString, true
	case lexer.TokenFunc, lexer.TokenPrint, lexer.TokenLet, lexer.TokenVar,
		lexer.TokenReturn, lexer.TokenIf, lexer.TokenElse, lexer.TokenImport,
		lexer.TokenMatrix, lexer.TokenAlloc, lexer.TokenFree, lexer.TokenAddr,
		lexer.TokenQReg, lexer.TokenGate, lexer.TokenMeasure, lexer.TokenActor,
		lexer.TokenSpawn, lexer.TokenReceive, lexer.TokenChannel, lexer.TokenSend,
		lexer.TokenMacro, lexer.TokenQuote, lexer.TokenUnquote, lexer.TokenComptime,
		lexer.TokenWhile, lexer.TokenFor, lexer.TokenIn, lexer.TokenBreak,
		lexer.TokenContinue, lexer.TokenConst, lexer.TokenStruct, lexer.TokenTypeDef,
		lexer.TokenBool, lexer.TokenTrue, lexer.TokenFalse,
		lexer.TokenKernel, lexer.TokenDevice, lexer.TokenGlobalID, lexer.TokenBarrier,
		lexer.TokenMut, lexer.TokenRaw, lexer.TokenMove,
		lexer.TokenSome, lexer.TokenNone, lexer.TokenOk, lexer.TokenErr,
		lexer.TokenMatch, lexer.TokenLinear, lexer.TokenPacked, lexer.TokenSIMD,
		lexer.TokenEnum, lexer.TokenFn, lexer.TokenPub:
		return SemTokKeyword, true
	case lexer.TokenAssign, lexer.TokenEqual, lexer.TokenNotEqual,
		lexer.TokenLessThan, lexer.TokenLessEqual,
		lexer.TokenGreaterThan, lexer.TokenGreaterEqual,
		lexer.TokenPlus, lexer.TokenMinus, lexer.TokenStar, lexer.TokenSlash,
		lexer.TokenPercent, lexer.TokenAnd, lexer.TokenOr, lexer.TokenNot,
		lexer.TokenLParen, lexer.TokenRParen, lexer.TokenLBrace, lexer.TokenRBrace,
		lexer.TokenLBracket, lexer.TokenRBracket, lexer.TokenComma, lexer.TokenDot,
		lexer.TokenAt, lexer.TokenSemicolon, lexer.TokenAmp, lexer.TokenFatArrow,
		lexer.TokenQuestion, lexer.TokenColon:
		return SemTokOperator, true
	default:
		return 0, false
	}
}

// semEntry is one classified span before delta encoding.
type semEntry struct {
	line   int // 0-based
	start  int // UTF-16 units
	length int // UTF-16 units
	typ    uint32
}

// utf16Units counts UTF-16 code units in s (LSP character semantics: BMP
// runes count 1, astral runes count 2).
func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		if r < 0x10000 {
			n++
		} else {
			n += 2
		}
	}
	return n
}

// byteColToUTF16 converts a byte column within lineBytes to UTF-16 units,
// clamping defensively (never negative, never past end of line).
func byteColToUTF16(line string, col int) int {
	if col < 0 {
		return 0
	}
	if col > len(line) {
		col = len(line)
	}
	return utf16Units(line[:col])
}

// EncodeSemanticTokens classifies a document into protocol delta data.
// It is total: any input (including empty or lexically broken text) yields
// a well-formed stream sorted by (line, start).
func EncodeSemanticTokens(text string) []uint32 {
	lines := strings.Split(text, "\n")

	l := lexer.New(text)
	var toks []lexer.Token
	for {
		t := l.NextToken()
		if t.Type == lexer.TokenEOF {
			break
		}
		toks = append(toks, t)
	}

	var out []semEntry
	emit := func(byteOff int, lit string, typ uint32) {
		// Derive (line, byte column) from the byte offset rather than the
		// token's Line/Col fields: STRING tokens report the opening quote's
		// column while spanning only the inner content (lexer.go), so the
		// offset is the only uniform source of truth.
		line := strings.Count(text[:byteOff], "\n")
		lineStart := strings.LastIndex(text[:byteOff], "\n") + 1
		byteCol := byteOff - lineStart
		lb := ""
		if line >= 0 && line < len(lines) {
			lb = lines[line]
		}
		for i, seg := range strings.Split(lit, "\n") {
			seg = strings.TrimSuffix(seg, "\r")
			if seg == "" {
				line++
				continue
			}
			start := byteColToUTF16(lb, byteCol)
			if i > 0 {
				start = 0
				if line >= 0 && line < len(lines) {
					lb = lines[line]
				}
			}
			out = append(out, semEntry{line: line, start: start, length: utf16Units(seg), typ: typ})
			line++
		}
	}

	prevEnd := 0 // byte offset of the previous token end in text
	emitGap := func(from, to int) {
		if to <= from {
			return
		}
		gap := text[from:to]
		if strings.TrimSpace(gap) == "" {
			return
		}
		// Gap start as (line, byte column) from the byte offset.
		line := strings.Count(text[:from], "\n")
		lineStart := strings.LastIndex(text[:from], "\n") + 1
		col := from - lineStart
		rest := gap
		for len(rest) > 0 {
			var part string
			if nl := strings.IndexByte(rest, '\n'); nl < 0 {
				part, rest = rest, ""
			} else {
				part, rest = rest[:nl], rest[nl+1:]
			}
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				lead := len(part) - len(strings.TrimLeft(part, " \t\r"))
				lb := ""
				if line >= 0 && line < len(lines) {
					lb = lines[line]
				}
				start := byteColToUTF16(lb, col+lead)
				out = append(out, semEntry{line: line, start: start, length: utf16Units(trimmed), typ: SemTokComment})
			}
			line++
			col = 0
		}
	}

	for _, t := range toks {
		start := int(t.Start)
		spanStart, spanEnd := start, start+int(t.Len)
		if t.Type == lexer.TokenString {
			// The lexer spans only the inner content (opening quote
			// excluded from Start, closing quote excluded from Len):
			// widen to the quotes so gap recovery never sees them.
			if spanStart > 0 && text[spanStart-1] == '"' {
				spanStart--
			}
			if spanEnd < len(text) && text[spanEnd] == '"' {
				spanEnd++
			}
		}
		emitGap(prevEnd, spanStart)
		if typ, ok := classifyToken(t.Type); ok {
			emit(spanStart, text[spanStart:spanEnd], typ)
		}
		prevEnd = spanEnd
	}
	emitGap(prevEnd, len(text))

	// Lexer order with inline gaps is already sorted, but multiline splits
	// and defensive clamps could disturb it: insertion-sort by (line, start)
	// to guarantee the protocol contract.
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && (out[j-1].line > out[j].line ||
			(out[j-1].line == out[j].line && out[j-1].start > out[j].start)) {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}

	data := make([]uint32, 0, len(out)*5)
	prevLine, prevStart := 0, 0
	for i, e := range out {
		deltaLine := e.line - prevLine
		deltaStart := e.start
		if deltaLine == 0 && i > 0 {
			deltaStart = e.start - prevStart
		}
		data = append(data, uint32(deltaLine), uint32(deltaStart), uint32(e.length), e.typ, 0)
		prevLine, prevStart = e.line, e.start
	}
	return data
}
