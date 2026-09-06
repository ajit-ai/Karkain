package diagnostics

// Code is a stable, machine-readable compiler error category. Unlike free-form
// messages, codes are part of the public toolchain contract: scripts may match
// on them, and `karkain explain <code>` prints documentation for each one.
// Package-manager errors (E-PKG-*) are owned by pkg/pm; this set covers the
// compiler/language classes.
type Code string

// Compiler error-code classes (Karkain-owned, "K" namespace). Every diagnostic
// emitted by the toolchain front end falls into exactly one of these classes.
const (
	CodeSyntax  Code = "E-K-SYN" // lexer/parser: malformed source
	CodeResolve Code = "E-K-RES" // name resolution: undefined/duplicate/private access
	CodeBorrow  Code = "E-K-BRW" // borrow checker: lifetime/ownership
	CodeSema    Code = "E-K-SEM" // semantic analysis: kernels/actors/coroutines/quantum
	CodeType    Code = "E-K-TYP" // type checking
	CodeCodegen Code = "E-K-CG"  // code generation / backend emission
	CodePackage Code = "E-K-PKG" // package/dependency integration
	CodeEnv     Code = "E-K-ENV" // infrastructure: compiler toolchain, filesystem
)

// Compiler warning-code classes (W-K-*). Warnings share the diagnostic model
// but never terminate compilation; code classes are still stable so tooling
// and `karkain explain` can address them.
const (
	CodeWarnUnused Code = "W-K-UNUSED" // name declared but never read
)

// Numeric-style diagnostic codes (K001, K002, etc.) as alternatives to E-K-*
// These provide a compact, human-friendly format while maintaining the same
// categorization as the E-K-* codes.
const (
	CodeK001 Code = "K001" // equivalent to E-K-SYN (syntax errors)
	CodeK002 Code = "K002" // equivalent to E-K-RES (name resolution)
	CodeK003 Code = "K003" // equivalent to E-K-BRW (borrow checker)
	CodeK004 Code = "K004" // equivalent to E-K-SEM (semantic analysis)
	CodeK005 Code = "K005" // equivalent to E-K-TYP (type checking)
	CodeK006 Code = "K006" // equivalent to E-K-CG (code generation)
	CodeK007 Code = "K007" // equivalent to E-K-PKG (package/dependency)
	CodeK008 Code = "K008" // equivalent to E-K-ENV (infrastructure)
	CodeK100 Code = "K100" // equivalent to W-K-UNUSED (unused variable)
)

// knownCompileCode reports whether c is a registered compiler error code.
func knownCompileCode(c Code) bool {
	switch c {
	case CodeSyntax, CodeResolve, CodeBorrow, CodeSema, CodeType, CodeCodegen, CodePackage, CodeEnv,
		CodeK001, CodeK002, CodeK003, CodeK004, CodeK005, CodeK006, CodeK007, CodeK008:
		return true
	}
	return false
}

// knownWarningCode reports whether c is a registered warning code.
func knownWarningCode(c Code) bool {
	switch c {
	case CodeWarnUnused, CodeK100:
		return true
	}
	return false
}

// Known reports whether the given uppercase code string is registered (either
// a compiler code, a warning code, or a package-manager code E-PKG-*).
func Known(code string) bool {
	return knownCompileCode(Code(code)) || knownWarningCode(Code(code))
}
