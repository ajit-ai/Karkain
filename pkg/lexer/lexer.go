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

	// Literals & Identifiers
	TokenIdent   TokenType = "IDENT"
	TokenInt     TokenType = "INT"
	TokenFloat64 TokenType = "FLOAT64"
	TokenString  TokenType = "STRING"

	// Operators & Delimiters
	TokenAssign      TokenType = "="
	TokenEqual       TokenType = "=="
	TokenLessThan    TokenType = "<"
	TokenGreaterThan TokenType = ">"
	TokenPlus        TokenType = "+"
	TokenMinus       TokenType = "-"
	TokenStar        TokenType = "*"
	TokenSlash       TokenType = "/"

	TokenLParen   TokenType = "("
	TokenRParen   TokenType = ")"
	TokenLBrace   TokenType = "{"
	TokenRBrace   TokenType = "}"
	TokenLBracket TokenType = "["
	TokenRBracket TokenType = "]"
	TokenComma    TokenType = ","
	TokenDot      TokenType = "."
	TokenAt       TokenType = "@"

		// Add new token constants
	TOKEN_MACRO    = "MACRO"
	TOKEN_QUOTE    = "QUOTE"
	TOKEN_UNQUOTE  = "UNQUOTE"
	TOKEN_COMPTIME = "COMPTIME"
	TOKEN_DERIVE   = "DERIVE"
	TOKEN_TAG      = "TAG"

)

	// Register keywords in lexer map
    var keywords = map[string]TokenType{
    "macro":    TOKEN_MACRO,
    "quote":    TOKEN_QUOTE,
    "unquote":  TOKEN_UNQUOTE,
    "comptime": TOKEN_COMPTIME,
}





type Token struct {
	Type    TokenType
	Literal string
	Line    int
}

type Lexer struct {
	Input        string
	Position     int
	ReadPosition int
	Ch           byte
	Line         int
}

// GetInput returns the lexer's input string (needed for C import parsing)
func (l *Lexer) GetInput() string {
	return l.Input
}

func New(input string) *Lexer {
	l := &Lexer{Input: input, Line: 1}
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
		tok = Token{Type: TokenLParen, Literal: "(", Line: l.Line}
	case ')':
		tok = Token{Type: TokenRParen, Literal: ")", Line: l.Line}
	case '{':
		tok = Token{Type: TokenLBrace, Literal: "{", Line: l.Line}
	case '}':
		tok = Token{Type: TokenRBrace, Literal: "}", Line: l.Line}
	case '[':
		tok = Token{Type: TokenLBracket, Literal: "[", Line: l.Line}
	case ']':
		tok = Token{Type: TokenRBracket, Literal: "]", Line: l.Line}
	case ',':
		tok = Token{Type: TokenComma, Literal: ",", Line: l.Line}
	case '.':
		tok = Token{Type: TokenDot, Literal: ".", Line: l.Line}
	case '@':
		tok = Token{Type: TokenAt, Literal: "@", Line: l.Line}
	case '=':
		if l.peekChar() == '=' {
			ch := l.Ch
			l.readChar()
			tok = Token{Type: TokenEqual, Literal: string(ch) + string(l.Ch), Line: l.Line}
		} else {
			tok = Token{Type: TokenAssign, Literal: "=", Line: l.Line}
		}
	case '<':
		if l.peekChar() == '-' {
			ch := l.Ch
			l.readChar()
			tok = Token{Type: TokenSend, Literal: string(ch) + string(l.Ch), Line: l.Line}
		} else {
			tok = Token{Type: TokenLessThan, Literal: "<", Line: l.Line}
		}
	case '>':
		tok = Token{Type: TokenGreaterThan, Literal: ">", Line: l.Line}
	case '+':
		tok = Token{Type: TokenPlus, Literal: "+", Line: l.Line}
	case '-':
		tok = Token{Type: TokenMinus, Literal: "-", Line: l.Line}
	case '*':
		tok = Token{Type: TokenStar, Literal: "*", Line: l.Line}
	case '/':
		tok = Token{Type: TokenSlash, Literal: "/", Line: l.Line}
	case '"':
		tok.Type = TokenString
		tok.Literal = l.readString()
		tok.Line = l.Line
		return tok
	case 0:
		tok = Token{Type: TokenEOF, Literal: "", Line: l.Line}
	default:
		if isLetter(l.Ch) {
			literal := l.readIdentifier()
			return Token{Type: lookupIdent(literal), Literal: literal, Line: l.Line}
		} else if isDigit(l.Ch) {
			literal := l.readNumber()
			// Check if it's a float
			if strings.Contains(literal, ".") {
				return Token{Type: TokenFloat64, Literal: literal, Line: l.Line}
			}
			return Token{Type: TokenInt, Literal: literal, Line: l.Line}
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.Ch), Line: l.Line}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.Ch == ' ' || l.Ch == '\t' || l.Ch == '\n' || l.Ch == '\r' {
		if l.Ch == '\n' {
			l.Line++
		}
		l.readChar()
	}

	// Skip comments
	if l.Ch == '/' && l.peekChar() == '/' {
		for l.Ch != '\n' && l.Ch != 0 {
			l.readChar()
		}
		l.skipWhitespace() // Skip whitespace after comment
	}
}

func (l *Lexer) readIdentifier() string {
	pos := l.Position
	for isLetter(l.Ch) || isDigit(l.Ch) {
		l.readChar()
	}
	return l.Input[pos:l.Position]
}

func (l *Lexer) readNumber() string {
	pos := l.Position
	for isDigit(l.Ch) {
		l.readChar()
	}
	// Handle float64 literals with decimal point
	if l.Ch == '.' {
		l.readChar()
		for isDigit(l.Ch) {
			l.readChar()
		}
	}
	return l.Input[pos:l.Position]
}

func (l *Lexer) readString() string {
	pos := l.Position + 1
	for {
		l.readChar()
		if l.Ch == 0 {
			break
		}
		if l.Ch == '\\' {
			l.readChar() // skip escaped character
			continue
		}
		if l.Ch == '"' {
			break
		}
	}
	str := l.Input[pos:l.Position]
	l.readChar()
	return str
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
	str := l.Input[pos:l.Position]
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
	default:
		return TokenIdent
	}
}
