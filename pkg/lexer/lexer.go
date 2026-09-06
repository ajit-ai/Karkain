package lexer

import (
	"strings"
)

type TokenType string

const (
	TokenEOF     TokenType = "EOF"
	TokenIllegal TokenType = "ILLEGAL"

	// Keywords
	TokenFunc   TokenType = "FUNC"
	TokenPrint  TokenType = "PRINT"
	TokenLet    TokenType = "LET"
	TokenVar    TokenType = "VAR"
	TokenReturn TokenType = "RETURN"
	TokenIf     TokenType = "IF"
	TokenElse   TokenType = "ELSE"
	TokenImport TokenType = "IMPORT"
	TokenMatrix TokenType = "MATRIX"
	TokenAlloc  TokenType = "ALLOC"
	TokenFree   TokenType = "FREE"
	TokenAddr   TokenType = "ADDR"

	// Quantum Keywords
	TokenQReg    TokenType = "QREG"
	TokenGate    TokenType = "GATE"
	TokenMeasure TokenType = "MEASURE"

	// Actor Keywords
	TokenActor   TokenType = "ACTOR"
	TokenSpawn   TokenType = "SPAWN"
	TokenReceive TokenType = "RECEIVE"
	TokenChannel TokenType = "CHANNEL"
	TokenSend    TokenType = "SEND"

	// Metaprogramming Keywords (Phase 17)
	TokenMacro    TokenType = "MACRO"
	TokenQuote    TokenType = "QUOTE"
	TokenUnquote  TokenType = "UNQUOTE"
	TokenComptime TokenType = "COMPTIME"

	// Loop Keywords
	TokenWhile    TokenType = "WHILE"
	TokenFor      TokenType = "FOR"
	TokenIn       TokenType = "IN"       // Phase 47: for-in loops
	TokenBreak    TokenType = "BREAK"    // Phase 48: break
	TokenContinue TokenType = "CONTINUE" // Phase 48: continue

	// Type Keywords
	TokenStruct  TokenType = "STRUCT"
	TokenTypeDef TokenType = "TYPE"
	TokenBool    TokenType = "BOOL"
	TokenTrue    TokenType = "TRUE"
	TokenFalse   TokenType = "FALSE"

	// Literals & Identifiers
	TokenIdent   TokenType = "IDENT"
	TokenInt     TokenType = "INT"
	TokenFloat64 TokenType = "FLOAT64"
	TokenBigInt  TokenType = "BIGINT"
	TokenBigFloat TokenType = "BIGFLOAT"
	TokenString  TokenType = "STRING"

	// Operators & Delimiters
	TokenAssign       TokenType = "="
	TokenEqual        TokenType = "=="
	TokenNotEqual     TokenType = "!="
	TokenLessThan     TokenType = "<"
	TokenLessEqual    TokenType = "<="
	TokenGreaterThan  TokenType = ">"
	TokenGreaterEqual TokenType = ">="
	TokenPlus         TokenType = "+"
	TokenMinus        TokenType = "-"
	TokenStar         TokenType = "*"
	TokenSlash        TokenType = "/"
	TokenPercent      TokenType = "%"
	TokenAnd          TokenType = "&&"
	TokenOr           TokenType = "||"
	TokenNot          TokenType = "!"

	TokenLParen    TokenType = "("
	TokenRParen    TokenType = ")"
	TokenLBrace    TokenType = "{"
	TokenRBrace    TokenType = "}"
	TokenLBracket  TokenType = "["
	TokenRBracket  TokenType = "]"
	TokenComma     TokenType = ","
	TokenDot       TokenType = "."
	TokenAt        TokenType = "@"
	TokenSemicolon TokenType = ";"
	TokenAmp       TokenType = "&"  // Phase 41: reference/borrow operator
	TokenFatArrow  TokenType = "=>" // Phase 42: match arm separator
	TokenQuestion  TokenType = "?"  // Phase 44: error propagation operator

	TOKEN_DERIVE TokenType = "DERIVE"
	TOKEN_TAG    TokenType = "TAG"

	TokenKernel   TokenType = "KERNEL"
	TokenDevice   TokenType = "DEVICE"
	TokenGlobalID TokenType = "GLOBAL_ID"
	TokenBarrier  TokenType = "BARRIER"
	TokenColon    TokenType = "COLON"

	// Phase 41: Hybrid Memory Model
	TokenMut TokenType = "MUT"
	TokenRaw TokenType = "RAW"
	TokenMove TokenType = "MOVE"

	// Phase 42: Option<T>, Result<T,E>, match
	TokenSome   TokenType = "SOME"
	TokenNone   TokenType = "NONE"
	TokenOk     TokenType = "OK"
	TokenErr    TokenType = "ERR"
	TokenMatch  TokenType = "MATCH"
	TokenLinear TokenType = "LINEAR"
	TokenPacked TokenType = "PACKED"
	TokenSIMD   TokenType = "SIMD"
	TokenEnum   TokenType = "ENUM"
	TokenFn     TokenType = "FN" // Phase 48: lambda/function pointers
	TokenPub    TokenType = "PUB" // Phase 80: visibility modifiers
)

var keywords = map[string]TokenType{
	"macro":    TokenMacro,
	"quote":    TokenQuote,
	"unquote":  TokenUnquote,
	"comptime": TokenComptime,
	"while":    TokenWhile,
	"fn":       TokenFn,
}

// Token stores source offsets instead of copying token strings.
// This eliminates heap allocations during tokenization — the token
// text is derived on-demand via the Literal() method which returns
// a zero-copy slice of the original source buffer.
type Token struct {
	Type TokenType
	Start uint32
	Len   uint16
	Line  uint16
	Col   uint16
}

// Literal returns the token's source text by slicing into the lexer's
// input buffer. This is a zero-allocation view at the point of token
// creation; the string is materialized only when this method is called.
func (t Token) Literal(src string) string {
	if t.Len == 0 {
		return ""
	}
	return src[t.Start : uint32(t.Start)+uint32(t.Len)]
}

// LiteralBytes returns the token's source text as a byte slice, avoiding
// any string allocation. Useful for comparisons.
func (t Token) LiteralBytes(src []byte) []byte {
	if t.Len == 0 {
		return nil
	}
	return src[t.Start : uint32(t.Start)+uint32(t.Len)]
}

type Lexer struct {
	Input        []byte
	Position     int
	ReadPosition int
	Ch           byte
	Line         int
	Col          int
}

// GetInput returns the lexer's input string (needed for C import parsing)
func (l *Lexer) GetInput() string {
	return string(l.Input)
}

// GetInputBytes returns the raw input bytes for zero-copy token literal access
func (l *Lexer) GetInputBytes() []byte {
	return l.Input
}

func New(input string) *Lexer {
	// Strip a leading UTF-8 byte-order mark (EF BB BF) if present. A BOM is
	// not a token; without skipping it, the first token of a file would be
	// corrupted (e.g. a `func` becoming an unknown identifier), silently
	// dropping the first declaration when sibling .kark files are concatenated.
	b := []byte(input)
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		b = b[3:]
	}
	l := &Lexer{Input: b, Line: 1, Col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.ReadPosition >= len(l.Input) {
		l.Ch = 0
	} else {
		l.Ch = l.Input[l.ReadPosition]
	}
	l.Position = l.ReadPosition
	l.ReadPosition++
}

func (l *Lexer) peekChar() byte {
	if l.ReadPosition >= len(l.Input) {
		return 0
	}
	return l.Input[l.ReadPosition]
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	var tok Token
	switch l.Ch {
	case '(':
		tok = Token{Type: TokenLParen, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case ')':
		tok = Token{Type: TokenRParen, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '{':
		tok = Token{Type: TokenLBrace, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '}':
		tok = Token{Type: TokenRBrace, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '[':
		tok = Token{Type: TokenLBracket, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case ']':
		tok = Token{Type: TokenRBracket, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case ':':
		tok = Token{Type: TokenColon, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case ',':
		tok = Token{Type: TokenComma, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case ';':
		tok = Token{Type: TokenSemicolon, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '.':
		tok = Token{Type: TokenDot, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '@':
		tok = Token{Type: TokenAt, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '=':
		if l.peekChar() == '=' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenEqual, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else if l.peekChar() == '>' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenFatArrow, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenAssign, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '!':
		if l.peekChar() == '=' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenNotEqual, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenNot, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '&':
		if l.peekChar() == '&' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenAnd, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenAmp, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '|':
		if l.peekChar() == '|' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenOr, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenIllegal, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '<':
		if l.peekChar() == '-' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenSend, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else if l.peekChar() == '=' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenLessEqual, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenLessThan, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '>':
		if l.peekChar() == '=' {
			start := l.Position
			l.readChar()
			l.Col++
			tok = Token{Type: TokenGreaterEqual, Start: uint32(start), Len: 2, Line: uint16(l.Line), Col: uint16(l.Col - 1)}
		} else {
			tok = Token{Type: TokenGreaterThan, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	case '+':
		tok = Token{Type: TokenPlus, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '-':
		tok = Token{Type: TokenMinus, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '*':
		tok = Token{Type: TokenStar, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '/':
		tok = Token{Type: TokenSlash, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '%':
		tok = Token{Type: TokenPercent, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '?':
		tok = Token{Type: TokenQuestion, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
	case '"':
		return l.scanString()
	case 0:
		tok = Token{Type: TokenEOF, Start: uint32(l.Position), Len: 0, Line: uint16(l.Line), Col: uint16(l.Col)}
	default:
		if isLetter(l.Ch) {
			return l.scanIdentifier()
		} else if isDigit(l.Ch) {
			return l.scanNumber()
		} else {
			tok = Token{Type: TokenIllegal, Start: uint32(l.Position), Len: 1, Line: uint16(l.Line), Col: uint16(l.Col)}
		}
	}

	l.readChar()
	l.Col++
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.Ch == ' ' || l.Ch == '\t' || l.Ch == '\n' || l.Ch == '\r' {
		if l.Ch == '\n' {
			l.Line++
			l.Col = 0
			l.readChar()
			continue
		}
		l.readChar()
		l.Col++
	}

	// Skip comments
	if l.Ch == '/' && l.peekChar() == '/' {
		for l.Ch != '\n' && l.Ch != 0 {
			l.readChar()
			l.Col++
		}
		l.skipWhitespace() // Skip whitespace after comment
	}
}

func (l *Lexer) scanIdentifier() Token {
	start := l.Position
	startLine := l.Line
	startCol := l.Col
	for isLetter(l.Ch) || isDigit(l.Ch) {
		l.readChar()
		l.Col++
	}
	tok := Token{Type: lookupIdent(string(l.Input[start:l.Position])), Start: uint32(start), Len: uint16(l.Position - start), Line: uint16(startLine), Col: uint16(startCol)}
	return tok
}

func (l *Lexer) scanNumber() Token {
	start := l.Position
	startLine := l.Line
	startCol := l.Col
	for isDigit(l.Ch) {
		l.readChar()
		l.Col++
	}
	// Handle float64 literals with decimal point
	if l.Ch == '.' {
		l.readChar()
		l.Col++
		for isDigit(l.Ch) {
			l.readChar()
			l.Col++
		}
	}
	tt := TokenInt
	text := string(l.Input[start:l.Position])
	if strings.Contains(text, ".") {
		// Check for bigfloat suffix 'b' after decimal
		if l.Ch == 'b' || l.Ch == 'B' {
			l.readChar()
			l.Col++
			return Token{Type: TokenBigFloat, Start: uint32(start), Len: uint16(l.Position - start), Line: uint16(startLine), Col: uint16(startCol)}
		}
		tt = TokenFloat64
	} else {
		// Check for bigint suffix 'n' after integer
		if l.Ch == 'n' || l.Ch == 'N' {
			l.readChar()
			l.Col++
			return Token{Type: TokenBigInt, Start: uint32(start), Len: uint16(l.Position - start), Line: uint16(startLine), Col: uint16(startCol)}
		}
	}
	return Token{Type: tt, Start: uint32(start), Len: uint16(l.Position - start), Line: uint16(startLine), Col: uint16(startCol)}
}

func (l *Lexer) scanString() Token {
	startLine := l.Line
	startCol := l.Col
	startPos := l.Position
	l.readChar() // skip opening quote
	l.Col++

	for {
		if l.Ch == 0 {
			break
		}
		if l.Ch == '\\' {
			// Consume both the backslash and the escaped character as string
			// content. The escaped character must be included in the token span
			// even when it is a quote ("\"" -> \" ) or another backslash.
			l.readChar() // advance past the backslash
			l.Col++
			if l.Ch != 0 {
				l.readChar() // consume the escaped character itself
				l.Col++
			}
			continue
		}
		if l.Ch == '"' {
			break
		}
		l.readChar()
		l.Col++
	}
	// Token spans from the opening quote to the closing quote (exclusive of closing quote for the inner content)
	tokStart := startPos + 1 // skip opening quote in the literal
	tokLen := l.Position - tokStart
	l.readChar() // consume closing quote
	l.Col++

	return Token{Type: TokenString, Start: uint32(tokStart), Len: uint16(tokLen), Line: uint16(startLine), Col: uint16(startCol)}
}

// Read C import block content between { }
func (l *Lexer) readCBlock() string {
	pos := l.Position + 1 // skip {
	braceDepth := 1
	for {
		l.readChar()
		if l.Ch == 0 {
			break
		}
		if l.Ch == '{' {
			braceDepth++
		}
		if l.Ch == '}' {
			braceDepth--
			if braceDepth == 0 {
				break
			}
		}
	}
	str := string(l.Input[pos:l.Position])
	l.readChar() // consume closing }
	return str
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func lookupIdent(ident string) TokenType {
	switch ident {
	case "func":
		return TokenFunc
	case "print":
		return TokenPrint
	case "println":
		return TokenPrint
	case "let":
		return TokenLet
	case "var":
		return TokenVar
	case "return":
		return TokenReturn
	case "if":
		return TokenIf
	case "else":
		return TokenElse
	case "import":
		return TokenImport
	case "matrix":
		return TokenMatrix
	case "alloc":
		return TokenAlloc
	case "free":
		return TokenFree
	case "addr":
		return TokenAddr
	case "float64":
		return TokenFloat64
	case "qreg":
		return TokenQReg
	case "gate":
		return TokenGate
	case "measure":
		return TokenMeasure
	case "actor":
		return TokenActor
	case "spawn":
		return TokenSpawn
	case "receive":
		return TokenReceive
	case "channel":
		return TokenChannel
	case "send":
		return TokenSend
	case "macro":
		return TokenMacro
	case "quote":
		return TokenQuote
	case "unquote":
		return TokenUnquote
	case "comptime":
		return TokenComptime
	case "while":
		return TokenWhile
	case "for":
		return TokenFor
	case "type":
		return TokenTypeDef
	case "struct":
		return TokenStruct
	case "bool":
		return TokenBool
	case "bigint":
		return TokenBigInt
	case "bigfloat":
		return TokenBigFloat
	case "true":
		return TokenTrue
	case "false":
		return TokenFalse
	case "kernel":
		return TokenKernel
	case "device":
		return TokenDevice
	case "global_id":
		return TokenGlobalID
	case "barrier":
		return TokenBarrier
	case "mut":
		return TokenMut
	case "raw":
		return TokenRaw
	case "move":
		return TokenMove
	case "Some":
		return TokenSome
	case "None":
		return TokenNone
	case "Ok":
		return TokenOk
	case "Err":
		return TokenErr
	case "match":
		return TokenMatch
	case "linear":
		return TokenLinear
	case "packed":
		return TokenPacked
	case "enum":
		return TokenEnum
	case "fn":
		return TokenFn
	case "public":
		return TokenPub
	case "in":
		return TokenIn
	case "break":
		return TokenBreak
	case "continue":
		return TokenContinue
	default:
		return TokenIdent
	}
}
