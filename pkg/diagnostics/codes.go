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
	CodeSyntax   Code = "E-K-SYN" // lexer/parser: malformed source
	CodeResolve  Code = "E-K-RES" // name resolution: undefined/duplicate/private access
	CodeBorrow   Code = "E-K-BRW" // borrow checker: lifetime/ownership
	CodeSema     Code = "E-K-SEM" // semantic analysis: kernels/actors/coroutines/quantum
	CodeType     Code = "E-K-TYP" // type checking
	CodeCodegen  Code = "E-K-CG"  // code generation / backend emission
	CodePackage  Code = "E-K-PKG" // package/dependency integration
	CodeEnv      Code = "E-K-ENV" // infrastructure: compiler toolchain, filesystem
)

// knownCompileCode reports whether c is a registered compiler error code.
func knownCompileCode(c Code) bool {
	switch c {
	case CodeSyntax, CodeResolve, CodeBorrow, CodeSema, CodeType, CodeCodegen, CodePackage, CodeEnv:
		return true
	}
	return false
}

// Known reports whether the given uppercase code string is registered (either
// a compiler code or a package-manager code E-PKG-*).
func Known(code string) bool {
	return knownCompileCode(Code(code))
}