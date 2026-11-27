// v1.2.26 S4b-2: kcc trait/impl parity via the Go-side staging bridge.
//
// The self-hosted parser has no TraitDecl/ImplDecl nodes. Rather than adding a
// second parser shape to kcc, the Go assembler lowers trait syntax to what kcc
// already understands: trait declarations become blank lines (they carry no
// runtime code), and impl bodies are lifted so their `func` methods become
// ordinary top-level declarations. Line numbers are preserved by replacing
// removed lines with empty lines, so kcc diagnostics still point at the
// original source lines.
//
// The Go engine keeps its native trait/impl parsing and validation; this
// bridge only affects the source text handed to kcc. Trait completeness
// validation therefore remains Go-side (documented boundary).
package cli

import (
	"os"
	"regexp"
	"strings"
)

// applyKCCBridge lowers trait syntax in an already-staged source file. It is
// applied after staging because flat projects are mirrored verbatim and never
// pass through kccAssembleSource.
func applyKCCBridge(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lowered := stripTraitImplForKCC(string(data))
	// v1.2.31: lower async/await for the self-hosted parser as well. Both
	// forms are sugar over the existing spawn/join runtime; only identifier
	// await operands are lowered in this first slice.
	lowered = reAsyncCall.ReplaceAllString(lowered, `spawn($1, `)
	lowered = reAwaitIdent.ReplaceAllString(lowered, `join($1)`)
	if lowered == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(lowered), 0o644)
}

var (
	reAsyncCall  = regexp.MustCompile(`\basync[ \t]+([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(`)
	reAwaitIdent = regexp.MustCompile(`\bawait[ \t]+([A-Za-z_][A-Za-z0-9_]*)\b`)
)

var (
	traitDeclStart = regexp.MustCompile(`^[ \t]*trait[ \t]+[A-Za-z_][A-Za-z0-9_]*[ \t]*\{`)
	implDeclStart  = regexp.MustCompile(`^[ \t]*impl[ \t]+[A-Za-z_][A-Za-z0-9_]*[ \t]+for[ \t]+[A-Za-z_][A-Za-z0-9_]*[ \t]*\{`)
)

// liftImplLine extracts the body of a single-line `impl ... { ... }` so the
// contained method declarations can stay on the same source line.
func liftImplLine(line string) (string, bool) {
	start := strings.Index(line, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(line); i++ {
		switch line[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(line[start+1 : i]), true
			}
		}
	}
	return "", false
}

// stripTraitImplForKCC lowers trait/impl declarations for the kcc parser.
// Only lines that begin a declaration are recognized; an unmatched brace
// leaves the remainder untouched rather than deleting user code.
func stripTraitImplForKCC(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, len(lines))
	copy(out, lines)
	i := 0
	for i < len(lines) {
		line := lines[i]
		if traitDeclStart.MatchString(line) {
			depth := strings.Count(line, "{") - strings.Count(line, "}")
			out[i] = ""
			i++
			for i < len(lines) && depth > 0 {
				out[i] = ""
				depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
				i++
			}
			continue
		}
		if implDeclStart.MatchString(line) {
			// Compact one-line impl: lift the inner method text in place so
			// the line count and the method both survive.
			if inner, ok := liftImplLine(line); ok {
				out[i] = inner
				i++
				continue
			}
			depth := strings.Count(line, "{") - strings.Count(line, "}")
			out[i] = ""
			i++
			for i < len(lines) && depth > 0 {
				// Method lines are kept as ordinary top-level functions; only
				// the impl's own closing brace is blanked.
				if depth == 1 && strings.TrimSpace(lines[i]) == "}" {
					out[i] = ""
					depth = 0
					i++
					break
				}
				depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
				i++
			}
			continue
		}
		i++
	}
	return strings.Join(out, "\n")
}
