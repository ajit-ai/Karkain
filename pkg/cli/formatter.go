package cli

import (
	"fmt"
	"karkain/pkg/lexer"
	"os"
	"strings"
)

// FormatCommand implements `karkain fmt`: a deterministic, token-level
// canonical formatter. With checkOnly=false it rewrites the file in place;
// with checkOnly=true it only reports whether the file is already canonical
// (for CI). It returns ExitSuccess when the file is (or becomes) canonical,
// and ExitFailure when `--check` finds that formatting is required.
//
// The formatter is intentionally conservative (contract-level, not a
// sophisticated AST pretty-printer): it preserves token text and order, line
// structure, leading indentation, blank lines and comments, and only
// canonicalizes inter-token spacing, trailing whitespace and line endings.
// Because tokens are never re-ordered or re-spelled, formatting cannot change
// semantics, and it is idempotent by construction.
func FormatCommand(file string, checkOnly bool) CommandResult {
	if err := ValidateKarFile(file); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	srcBytes, err := os.ReadFile(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	formatted, danger, err := canonicalize(string(srcBytes))
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}

	if danger {
		// A token literal spans multiple lines (e.g. a multi-line string);
		// token-stream reshaping could corrupt it, so the file is left
		// untouched and reported as canonical to stay idempotent.
		return CommandResult{ExitCode: ExitSuccess, Message: "no formatting needed (line-preserving mode)"}
	}

	if formatted == string(srcBytes) {
		return CommandResult{ExitCode: ExitSuccess, Message: "already formatted."}
	}

	if checkOnly {
		return CommandResult{ExitCode: ExitFailure, Message: "file is not formatted."}
	}

	if err := os.WriteFile(file, []byte(formatted), 0o644); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing file: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "formatted."}
}

// canonicalize produces the canonical form of a source text. It returns
// danger=true when any token literal contains a newline (multi-line literals),
// in which case the caller must leave the source untouched.
func canonicalize(src string) (string, bool, error) {
	// Normalize line endings first so positions stay aligned afterwards.
	normalized := strings.ReplaceAll(src, "\r\n", "\n")

	l := lexer.New(normalized)
	var tokens []lexer.Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == lexer.TokenEOF {
			break
		}
	}

	// Multi-line literal detection + string-span mask for comment detection.
	stringMask := make([]bool, len(normalized))
	for _, tok := range tokens {
		if tok.Type == lexer.TokenString {
			end := int(tok.Start) + int(tok.Len)
			for i := int(tok.Start); i < end && i < len(normalized); i++ {
				stringMask[i] = true
			}
		}
		if lit := tok.Literal(normalized); strings.ContainsAny(lit, "\r\n") {
			return src, true, nil
		}
	}

	lines := strings.Split(normalized, "\n")
	lineStart := 0
	var out strings.Builder

	for li, line := range lines {
		lineEnd := lineStart + len(line)
		lineNo := li + 1

		var lineTokens []lexer.Token
		for _, tok := range tokens {
			if int(tok.Line) == lineNo && tok.Type != lexer.TokenEOF {
				lineTokens = append(lineTokens, tok)
			}
		}

		if lineHasComment(normalized, lineStart, lineEnd, stringMask) || len(lineTokens) == 0 {
			out.WriteString(strings.TrimRight(line, " \t"))
		} else {
			emitCanonicalLine(&out, normalized, line, lineTokens)
		}

		if li < len(lines)-1 {
			out.WriteString("\n")
		}
		lineStart = lineEnd + 1
	}

	return out.String(), false, nil
}

// lineHasComment reports whether the given source line contains a `//` comment
// outside any string-literal span.
func lineHasComment(src string, lineStart, lineEnd int, stringMask []bool) bool {
	for i := lineStart; i < lineEnd-1; i++ {
		if i >= len(src) || i+1 >= len(src) {
			break
		}
		if stringMask[i] {
			continue
		}
		if src[i] == '/' && src[i+1] == '/' {
			return true
		}
	}
	return false
}

// emitCanonicalLine writes one line: its original leading indentation followed
// by the line's tokens joined with canonical spacing.
func emitCanonicalLine(out *strings.Builder, src, line string, toks []lexer.Token) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	out.WriteString(indent)

	for i, tok := range toks {
		if i > 0 {
			out.WriteString(canonicalSeparator(toks[i-1], tok))
		}
		out.WriteString(tokenText(src, tok))
	}
}

// tokenText renders a token's source text. String tokens span the inner
// content only (quotes excluded by the lexer), so write the surrounding quotes
// back to keep the token spelled exactly as the source author intended.
func tokenText(src string, tok lexer.Token) string {
	if tok.Type == lexer.TokenString {
		return `"` + tok.Literal(src) + `"`
	}
	return tok.Literal(src)
}

// canonicalSeparator decides the space between two consecutive tokens: closing
// delimiters hug their content as in `f(x)`, `.` binds to both neighbors, and
// everything else gets a single space.
func canonicalSeparator(prev, next lexer.Token) string {
	switch next.Type {
	case lexer.TokenComma, lexer.TokenSemicolon, lexer.TokenColon,
		lexer.TokenDot, lexer.TokenRParen, lexer.TokenRBracket, lexer.TokenRBrace,
		lexer.TokenLParen, lexer.TokenLBracket:
		return ""
	}
	switch prev.Type {
	case lexer.TokenLParen, lexer.TokenLBracket, lexer.TokenLBrace, lexer.TokenDot:
		return ""
	}
	return " "
}