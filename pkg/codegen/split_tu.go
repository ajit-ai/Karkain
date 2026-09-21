package codegen

// Phase 134: per-module translation-unit split for incremental builds.
//
// The Go front end historically emits one monolithic C unit per project. To
// rebuild only changed modules, v2 splits emission into one translation
// unit (TU) per source module plus a shared runtime TU, all linked together:
//
//   karkain_runtime.h   — the full preamble text with internal linkage:
//                         top-level function definitions gain `static`
//                         (identical code per TU, no cross-TU references),
//                         shared mutable state becomes `extern` decls.
//   karkain_runtime.c   — `#include` of the header plus the single
//                         definitions of the shared mutable globals.
//   <module>.c          — `#include` of the header, all struct/enum/forward
//                         declarations, then that module's function bodies.
//   <module>.o          — `gcc -c` output, content-addressed in the cache.
//   runtime.o           — compiled once per compiler+runtime key.
//
// Design notes (deliberate):
//   - The transform is line-based over the already-emitted preamble text, so
//     new runtime helpers are handled automatically (a missed shape fails
//     loudly at C compile/link time, never silently). The only hand-written
//     list is the shared-mutable-global set below.
//   - Program-level constructs (concurrency core/wrappers/glue, profiling
//     tables, AVX kernels, C-import blocks, kernels, main) stay in the ROOT
//     module TU verbatim. Cross-module use of those surfaces fails loudly
//     at C compile time (undefined identifier); every pinned multi-file
//     corpus program is single-surface-per-file, so all gates pass.
//   - `static inline` helpers, `static const` tables, typedefs, enums and
//     macros duplicate harmlessly per TU (identical text, internal linkage).
//   - The per-compilation-host precompiled header (karkain_runtime.h.gch)
//     is opportunistic: gcc uses it automatically when present beside the
//     header and falls back to plain inclusion otherwise.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RuntimeHeaderName is the shared header all TUs include.
const RuntimeHeaderName = "karkain_runtime.h"

// RuntimeSourceName is the tiny TU defining the shared mutable globals.
const RuntimeSourceName = "karkain_runtime.c"

// RuntimeObjectName is the cached compiled runtime TU.
const RuntimeObjectName = "runtime.o"

// PCHName is the optional precompiled header beside the runtime header.
const PCHName = RuntimeHeaderName + ".gch"

// sharedGlobal describes one file-scope mutable runtime variable that must
// exist exactly once program-wide: extern declaration (header) + single
// definition (runtime TU).
type sharedGlobal struct {
	// match is a distinctive substring of the definition line.
	match string
	// decl is the header declaration, def the runtime TU definition.
	decl string
	def  string
}

// sharedRuntimeGlobals is exhaustive: every file-scope mutable (non-const)
// variable in the preamble. const tables and function-local statics are
// intentionally absent (safe per-TU duplication).
var sharedRuntimeGlobals = []sharedGlobal{
	{"_karkain_gargc", "extern int _karkain_gargc;", "int _karkain_gargc = 0;"},
	{"_karkain_gargv", "extern char** _karkain_gargv;", "char** _karkain_gargv = NULL;"},
	{"_karkain_files[", "extern FILE* _karkain_files[256];", "FILE* _karkain_files[256];"},
	{"_karkain_file_count", "extern int _karkain_file_count;", "int _karkain_file_count = 0;"},
	{"_karkain_net_started", "extern int _karkain_net_started;", "int _karkain_net_started = 0;"},
	{"_karkain_net_error[", "extern char _karkain_net_error[512];", "char _karkain_net_error[512] = \"\";"},
	{"karkain_frames[", "extern karkain_frame karkain_frames[KARKAIN_MAX_FRAMES];", "karkain_frame karkain_frames[KARKAIN_MAX_FRAMES];"},
	{"karkain_frame_depth", "extern int karkain_frame_depth;", "int karkain_frame_depth = 0;"},
	{"karkain_current_line", "extern long long karkain_current_line;", "long long karkain_current_line = 0;"},
}

// HeaderForRuntime transforms full preamble text into a self-contained
// header: top-level function definitions gain internal linkage (with their
// forward prototypes static-ified to match — a static definition following
// a non-static prototype is a hard C error), shared mutable globals become
// extern declarations, everything else verbatim.
func HeaderForRuntime(preamble string) string {
	lines := strings.Split(preamble, "\n")
	staticFns := map[string]bool{}
	for _, line := range lines {
		if name, ok := topLevelFuncName(line); ok {
			staticFns[name] = true
		}
	}
	var sb strings.Builder
	sb.WriteString("#ifndef KARKAIN_RUNTIME_H\n#define KARKAIN_RUNTIME_H\n\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if sharedDecl, ok := matchSharedGlobal(line); ok {
			sb.WriteString(sharedDecl)
			sb.WriteString("\n")
			continue
		}
		if _, ok := topLevelFuncName(line); ok {
			sb.WriteString("static " + line + "\n")
			continue
		}
		if name, ok := funcPrototypeName(line, trimmed); ok && staticFns[name] {
			sb.WriteString("static " + line + "\n")
			continue
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n#endif // KARKAIN_RUNTIME_H\n")
	return sb.String()
}

// topLevelFuncName extracts the function name from a column-zero definition
// line (`<type> <name>(...) {`), or "" when the line is not one.
func topLevelFuncName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !isTopLevelFuncDef(line, trimmed) {
		return "", false
	}
	open := strings.Index(trimmed, "(")
	if open < 0 {
		return "", false
	}
	head := strings.TrimSpace(trimmed[:open])
	fields := strings.Fields(head)
	if len(fields) == 0 {
		return "", false
	}
	name := fields[len(fields)-1]
	// Strip pointer stars attached to the name (`char* foo(` or `char *foo(`).
	name = strings.Trim(name, "*")
	if name == "" || strings.ContainsAny(name, " \t") {
		return "", false
	}
	return name, true
}

// funcPrototypeName extracts the function name from a column-zero prototype
// line (`<type> <name>(...);`), or "" when the line is not one.
func funcPrototypeName(line, trimmed string) (string, bool) {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return "", false
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "typedef") ||
		strings.HasPrefix(trimmed, "struct") || strings.HasPrefix(trimmed, "enum") ||
		strings.HasPrefix(trimmed, "union") || strings.HasPrefix(trimmed, "}") ||
		strings.HasPrefix(trimmed, "static ") || strings.HasPrefix(trimmed, "extern ") {
		return "", false
	}
	if !strings.HasSuffix(trimmed, ";") || !strings.Contains(trimmed, "(") {
		return "", false
	}
	open := strings.Index(trimmed, "(")
	head := strings.TrimSpace(trimmed[:open])
	fields := strings.Fields(head)
	if len(fields) == 0 {
		return "", false
	}
	name := strings.Trim(fields[len(fields)-1], "*")
	if name == "" || strings.ContainsAny(name, " \t();,") {
		return "", false
	}
	return name, true
}

// matchSharedGlobal reports the extern declaration when line defines one of
// the shared mutable globals.
func matchSharedGlobal(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "static ") {
		return "", false
	}
	if strings.Contains(trimmed, "(") {
		return "", false // functions handled by the definition rule
	}
	for _, g := range sharedRuntimeGlobals {
		if strings.Contains(trimmed, g.match) {
			return g.decl, true
		}
	}
	return "", false
}

// isTopLevelFuncDef reports whether a preamble line is a top-level function
// definition needing internal linkage for multi-TU inclusion: starts at
// column zero, ends with an opening brace, and is not a directive, comment,
// typedef/struct/enum/union declaration, or already static/extern.
func isTopLevelFuncDef(line, trimmed string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return false
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "typedef") ||
		strings.HasPrefix(trimmed, "struct") || strings.HasPrefix(trimmed, "enum") ||
		strings.HasPrefix(trimmed, "union") || strings.HasPrefix(trimmed, "}") ||
		strings.HasPrefix(trimmed, "static ") || strings.HasPrefix(trimmed, "extern ") {
		return false
	}
	if !strings.HasSuffix(trimmed, "{") {
		return false
	}
	return strings.Contains(trimmed, "(")
}

// RuntimeSourceFor returns runtime.c: the header include plus the single
// definitions of the shared mutable globals.
func RuntimeSourceFor() string {
	var sb strings.Builder
	sb.WriteString("#include \"" + RuntimeHeaderName + "\"\n\n")
	sb.WriteString("// Phase 134: single definitions of program-wide mutable runtime state.\n")
	for _, g := range sharedRuntimeGlobals {
		sb.WriteString(g.def + "\n")
	}
	return sb.String()
}

// gccTimeout bounds every spawned C toolchain invocation.
const gccTimeout = 60 * time.Second

// HostToolchain detects the C toolchain for cfg, reporting whether it
// speaks gcc-style flags (`-c`/`-o`/`-I`, suitable for per-TU flows).
// MSVC cl.exe (and detection failures, surfaced by the caller as the
// canonical toolchain error) select the monolith fallback instead.
//
// On success it also returns the derived per-TU compile flags, link
// libraries, and the -mconsole link need, so callers never parse raw
// flag lists themselves.
func HostToolchain(cfg Config) (cc string, gccStyle bool, baseFlags []string, err error) {
	g := New(cfg)
	cc, flags, err := g.detectCompilerForTarget("KARKAIN_TU_SRC", "KARKAIN_TU_OUT")
	if err != nil {
		return "", false, nil, err
	}
	base := filepath.Base(cc)
	if base == "cl" || base == "cl.exe" {
		return cc, false, flags, nil
	}
	return cc, true, flags, nil
}

// SplitToolchain combines HostToolchain with SplitFlags: one call gives
// the driver everything for per-TU compiles, PCH builds, and links.
func SplitToolchain(cfg Config) (cc string, compile []string, libs []string, console bool, gccStyle bool, err error) {
	cc, gccStyle, baseFlags, err := HostToolchain(cfg)
	if err != nil || !gccStyle {
		return cc, nil, nil, false, gccStyle, err
	}
	compile, libs, console = SplitFlags(baseFlags)
	return cc, compile, libs, console, true, nil
}

// SplitFlags derives per-TU compile flags and link libraries from a
// monolith-style flag list (with KARKAIN_TU_SRC/OUT placeholders): drops
// the file placeholders and linker inputs from the compile set, keeps
// defines/standards/arch flags (including PCH-compatible -mavx), and
// collects -l libraries plus -mconsole for the link set.
func SplitFlags(monolith []string) (compile []string, linkLibs []string, consoleLink bool) {
	skipNext := false
	for _, f := range monolith {
		if skipNext {
			skipNext = false
			continue
		}
		if f == "KARKAIN_TU_SRC" || f == "KARKAIN_TU_OUT" {
			continue
		}
		if f == "-o" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(f, "-l") {
			linkLibs = append(linkLibs, f)
			continue
		}
		if f == "-mconsole" {
			consoleLink = true
			continue
		}
		compile = append(compile, f)
	}
	return compile, linkLibs, consoleLink
}

// CompileTU compiles one C file to an object file. All paths must be
// absolute (the child inherits the caller's working directory).
func CompileTU(cc string, baseFlags []string, includeDir, src, obj string) error {
	args := append(append([]string{}, baseFlags...), "-I", includeDir, "-c", src, "-o", obj)
	return runGCC(cc, args, gccTimeout)
}

// LinkObjects links objects (plus the runtime object first) into exePath.
func LinkObjects(cc string, linkLibs []string, consoleLink bool, objs []string, exePath string) error {
	args := append(append([]string{}, objs...), "-o", exePath)
	if consoleLink {
		args = append(args, "-mconsole")
	}
	args = append(args, linkLibs...)
	return runGCC(cc, args, gccTimeout)
}

// BuildPCH precompiles the runtime header beside itself. Best-effort: a
// failure only forfeits the speedup (gcc falls back to plain inclusion).
func BuildPCH(cc string, baseFlags []string, headerPath string) {
	args := append(append([]string{}, baseFlags...), "-x", "c-header", headerPath, "-o", headerPath+".gch")
	_ = runGCC(cc, args, gccTimeout)
}

// runGCC runs one C toolchain invocation with stdout/stderr passthrough,
// mirroring the Windows cmd.exe handling of the monolith path. All paths
// must be absolute (the child inherits the caller's working directory).
func runGCC(compiler string, args []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, compiler, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", append([]string{"/C", compiler}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}
