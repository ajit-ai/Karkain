package target

import "strings"

// KIRClass classifies one KIR v1 text line (src/compiler/kir.kark) according
// to the operation it describes. Classification is purely textual and mirrors
// the statement forms the self-hosted emitter actually produces — it is a
// query vocabulary over KIR v1 text, not a re-implementation of the IR.
type KIRClass string

const (
	KIRImport       KIRClass = "import"
	KIRCImport      KIRClass = "cimport"
	KIRFunc         KIRClass = "func"
	KIRKernel       KIRClass = "kernel"
	KIRStruct       KIRClass = "struct"
	KIREnum         KIRClass = "enum"
	KIRVar          KIRClass = "var"
	KIRLet          KIRClass = "let"
	KIRConst        KIRClass = "const"
	KIRReturn       KIRClass = "return"
	KIRPrint        KIRClass = "print"
	KIRExprStmt     KIRClass = "expr_stmt"
	KIRBlock        KIRClass = "block"
	KIRIf           KIRClass = "if"
	KIRElse         KIRClass = "else"
	KIRWhile        KIRClass = "while"
	KIRFor          KIRClass = "for"
	KIRForIn        KIRClass = "forin"
	KIRBreak        KIRClass = "break"
	KIRContinue     KIRClass = "continue"
	KIRMatch        KIRClass = "match"
	KIRCase         KIRClass = "case"
	KIRAlloc        KIRClass = "alloc"
	KIRFree         KIRClass = "free"
	KIRGlobalID     KIRClass = "global_id"
	KIRMeasure      KIRClass = "measure"
	KIRUnknown      KIRClass = "unknown"
)

// ClassifyKIRLine maps one KIR v1 text line (trimmed of leading indentation)
// to its operation class. Structural markers ("block", "stmt <T>") and
// unknown forms fall back to KIRUnknown; a program under the KIR lowering
// boundary therefore reports deterministic diagnostics for anything the target
// is not annotated to accept.
func ClassifyKIRLine(line string) KIRClass {
	t := strings.TrimSpace(line)
	switch {
	case t == "KIR v1" || t == "block" || strings.HasPrefix(t, "stmt <") ||
		strings.HasPrefix(t, "source: "):
		return KIRUnknown
	case t == "":
		return KIRUnknown
	case strings.HasPrefix(t, "import "):
		return KIRImport
	case strings.HasPrefix(t, "cimport "):
		return KIRCImport
	case strings.HasPrefix(t, "func "):
		return KIRFunc
	case strings.HasPrefix(t, "kernel "):
		return KIRKernel
	case strings.HasPrefix(t, "struct "):
		return KIRStruct
	case strings.HasPrefix(t, "enum "):
		return KIREnum
	case strings.HasPrefix(t, "var "):
		return KIRVar
	case strings.HasPrefix(t, "let "):
		return KIRLet
	case strings.HasPrefix(t, "const "):
		return KIRConst
	case strings.HasPrefix(t, "return"):
		return KIRReturn
	case strings.HasPrefix(t, "print "):
		return KIRPrint
	case strings.HasPrefix(t, "else"):
		return KIRElse
	case strings.HasPrefix(t, "if "):
		return KIRIf
	case strings.HasPrefix(t, "while "):
		return KIRWhile
	case strings.HasPrefix(t, "for "):
		return KIRFor
	case strings.HasPrefix(t, "forin "):
		return KIRForIn
	case strings.HasPrefix(t, "break"):
		return KIRBreak
	case strings.HasPrefix(t, "continue"):
		return KIRContinue
	case strings.HasPrefix(t, "match "):
		return KIRMatch
	case strings.HasPrefix(t, "case "):
		return KIRCase
	case strings.HasPrefix(t, "alloc "):
		return KIRAlloc
	case strings.HasPrefix(t, "free "):
		return KIRFree
	case strings.HasPrefix(t, "global_id "):
		return KIRGlobalID
	case strings.Contains(t, "qmeasure") || strings.Contains(t, " measure"):
		return KIRMeasure
	default:
		// A statement that is none of the above. ExprStmt lines begin with the
		// rendered expression ("(call ...)", "x", "(index ...)", ...) and carry
		// no leading keyword, so surviving lines are expression statements.
		return KIRExprStmt
	}
}