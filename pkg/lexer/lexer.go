package lexer

type TokenType string

const (
	TokenEOF     TokenType = "EOF"
	TokenIllegal TokenType = "ILLEGAL"

	// Keywords
	TokenFunc   TokenType = "FUNC"
	TokenPrint  TokenType = "PRINT"
	TokenLet    TokenType = "LET"
	TokenReturn TokenType = "RETURN"
	TokenIf     TokenType = "IF"
	TokenElse   TokenType = "ELSE"
	TokenTypeKw TokenType = "TYPE"
	TokenStruct  TokenType = "STRUCT"
	TokenDelete  TokenType = "DELETE"
	TokenHasKey  TokenType = "HASKEY"
	TokenImport  TokenType = "IMPORT"
	TokenVar     TokenType = "VAR"
	TokenMatrix  TokenType = "MATRIX"
	TokenPrintln TokenType = "PRINTLN"

	// Literals & Identifiers
	TokenIdent  TokenType = "IDENT"
	TokenInt    TokenType = "INT"
	TokenString TokenType = "STRING"

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
	TokenColon    TokenType = ":"
	TokenDot      TokenType = "."
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
}

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
	line         int
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	var tok Token
	switch l.ch {
	case '(':
		tok = Token{Type: TokenLParen, Literal: "(", Line: l.line}
	case ')':
		tok = Token{Type: TokenRParen, Literal: ")", Line: l.line}
	case '{':
		tok = Token{Type: TokenLBrace, Literal: "{", Line: l.line}
	case '}':
		tok = Token{Type: TokenRBrace, Literal: "}", Line: l.line}
	case '[':
		tok = Token{Type: TokenLBracket, Literal: "[", Line: l.line}
	case ']':
		tok = Token{Type: TokenRBracket, Literal: "]", Line: l.line}
	case ',':
		tok = Token{Type: TokenComma, Literal: ",", Line: l.line}
	case ':':
		tok = Token{Type: TokenColon, Literal: ":", Line: l.line}
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: TokenEqual, Literal: string(ch) + string(l.ch), Line: l.line}
		} else {
			tok = Token{Type: TokenAssign, Literal: "=", Line: l.line}
		}
	case '<':
		tok = Token{Type: TokenLessThan, Literal: "<", Line: l.line}
	case '>':
		tok = Token{Type: TokenGreaterThan, Literal: ">", Line: l.line}
	case '+':
		tok = Token{Type: TokenPlus, Literal: "+", Line: l.line}
	case '-':
		tok = Token{Type: TokenMinus, Literal: "-", Line: l.line}
	case '*':
		tok = Token{Type: TokenStar, Literal: "*", Line: l.line}
	case '/':
		tok = Token{Type: TokenSlash, Literal: "/", Line: l.line}
	case '.':
		tok = Token{Type: TokenDot, Literal: ".", Line: l.line}
	case '"':
		tok.Type = TokenString
		tok.Literal = l.readString()
		tok.Line = l.line
		return tok
	case 0:
		tok = Token{Type: TokenEOF, Literal: "", Line: l.line}
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			return Token{Type: lookupIdent(literal), Literal: literal, Line: l.line}
		} else if isDigit(l.ch) {
			return Token{Type: TokenInt, Literal: l.readNumber(), Line: l.line}
		} else {
			tok = Token{Type: TokenIllegal, Literal: string(l.ch), Line: l.line}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for {
		if l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
			if l.ch == '\n' {
				l.line++
			}
			l.readChar()
		} else if l.ch == '/' && l.peekChar() == '/' {
			// Skip comment until end of line
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
		} else {
			break
		}
	}
}


func (l *Lexer) readIdentifier() string {
	pos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[pos:l.position]
}

func (l *Lexer) readNumber() string {
	pos := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[pos:l.position]
}

func (l *Lexer) readString() string {
	pos := l.position + 1
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '\\' {
			l.readChar() // skip escaped character
			continue
		}
		if l.ch == '"' {
			break
		}
	}
	str := l.input[pos:l.position]
	l.readChar()
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
	case "let":
		return TokenLet
	case "return":
		return TokenReturn
	case "if":
		return TokenIf
	case "else":
		return TokenElse
	case "type":
		return TokenTypeKw
	case "struct":
		return TokenStruct
	case "delete":
		return TokenDelete
	case "hasKey":
		return TokenHasKey
	case "import":
		return TokenImport
	case "var":
		return TokenVar
	case "matrix":
		return TokenMatrix
	case "println":
		return TokenPrintln
	default:
		return TokenIdent
	}
}
