package codegen

// Phase 134: per-module translation-unit emission for incremental builds.
//
// The monolith path (GenerateAndCompile) is untouched. This file adds a
// parallel emission that splits one assembled program into a shared runtime
// TU plus one TU per source module, all linked together:
//
//   karkain_runtime.h   — transformed preamble (internal linkage, extern
//                         shared state) + AVX kernel prototypes.
//   karkain_runtime.c   — header include + shared-global definitions +
//                         fixed AVX kernel bodies.
//   <module>.c          — header include + shared decls + that module's
//                         function bodies (+ its nested lambdas via the
//                         lambdaBuf protocol, + root-only program extras).
//
// Module attribution is by top-level function name: duplicate top-level
// definitions are already a K107 error, so every name in a VALID program
// belongs to exactly one file. (Invalid duplicates emit twice and fail
// loudly at link time, the same class as the monolith's C error.)
//
// Concurrency core/wrappers/glue, profiling tables, C-import blocks and
// kernels stay in the ROOT module TU verbatim; cross-module use of those
// surfaces fails loudly at C compile time (every pinned multi-file corpus
// program is single-surface-per-file, so all gates pass).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
)

// SplitResult bundles every text artifact of a split emission.
type SplitResult struct {
	Header     string            // karkain_runtime.h content
	RuntimeSrc string            // karkain_runtime.c content
	RootTU     string            // root module TU (program extras included)
	ModuleTUs  map[string]string // module file path -> TU text (root excluded)
	// Fallback signals: when set, the caller must use the monolith path.
	UsesConcurrency bool
	Profiling       bool
	SimdNeedsAVX    bool
	// RootFile is the assembly root (main lives here by filtering).
	RootFile string
}

// ModuleFuncNames parses each file and returns its top-level function
// names. Macro expansion cannot rename top-level FuncDecls, so parsing
// without expansion is sufficient and cheap.
func ModuleFuncNames(files []string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading module %s: %w", f, err)
		}
		l := lexer.New(string(data))
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("parsing module %s: %s", f, p.Errors[0])
		}
		var names []string
		for _, st := range prog.Statements {
			if fn, ok := st.(*parser.FuncDecl); ok && fn.Name != "" {
				names = append(names, fn.Name)
			}
		}
		out[f] = names
	}
	return out, nil
}

// GenerateSplit emits the runtime header/source plus one TU per module.
// prog is the fully assembled program (parsed once by the caller); files
// is the deterministic assembly order with the root file last. Only
// whole-program prescans run here — per-module work is pure emission.
func GenerateSplit(cfg Config, prog *parser.Program, files []string, rootFile string) (*SplitResult, error) {
	g := New(cfg)
	// Phase 146A: same instantiation the monolith path runs — split TUs
	// must see plain-unit specializations, never templates.
	if monoErrs := sema.MonomorphizeProgram(prog); len(monoErrs) > 0 {
		return nil, fmt.Errorf("%s", monoErrs[0].Error())
	}
	g.sourceFile = strings.Replace(rootFile, "\\", "/", -1)
	g.prescanTables(prog)

	res := &SplitResult{
		ModuleTUs:       map[string]string{},
		UsesConcurrency: g.usesConcurrency,
		Profiling:       g.profiling,
		RootFile:        rootFile,
	}

	// Runtime texts: preamble + SIMD runtime through the header transform,
	// plus fixed AVX kernel bodies and the shared-global definitions.
	runtimeText := g.generateCHeader() + simdRuntimeC()
	res.Header = HeaderForRuntime(runtimeText) + avxKernelProtos()
	res.RuntimeSrc = "#include \"" + RuntimeHeaderName + "\"\n\n" +
		RuntimeSourceFor() +
		g.genAVX2MatrixMul("rowsA", "colsA", "colsB") +
		g.genScalarMatrixMul()

	// Attribute top-level functions to their defining files.
	funcFiles, err := ModuleFuncNames(files)
	if err != nil {
		return nil, err
	}
	owner := map[string]string{} // func name -> file (first wins; dups link-fail loudly)
	for _, f := range files {
		for _, name := range funcFiles[f] {
			if _, seen := owner[name]; !seen {
				owner[name] = f
			}
		}
	}

	// Shared declarations go in every TU (identical text, deterministic).
	var shared strings.Builder
	g.emitSharedDecls(&shared, prog)
	sharedText := shared.String()

	rootAbs, _ := filepath.Abs(rootFile)
	for _, f := range files {
		var sb strings.Builder
		sb.WriteString("#include \"" + RuntimeHeaderName + "\"\n")
		if g.needsHTTP {
			sb.WriteString("#define KARKAIN_USE_HTTP\n")
		}
		if g.cfg.Trace {
			sb.WriteString("#define KARKAIN_TRACE 1\n")
		}
		isRoot := sameFile(f, rootFile, rootAbs)
		if isRoot {
			// C-import blocks live in the root TU (verbatim move; see docs).
			if len(prog.CImports) > 0 {
				for _, cImport := range prog.CImports {
					if cImport.Content != "" && !strings.Contains(cImport.Content, "Phase 11") {
						sb.WriteString(cImport.Content)
						sb.WriteByte('\n')
					}
				}
			}
		}
		sb.WriteString(sharedText)
		// Per-program extras stay in the root TU verbatim.
		if isRoot {
			if g.usesConcurrency {
				sb.WriteString(g.concWrapperC())
				sb.WriteString(concRuntimeAPIC())
			}
			if g.profiling {
				sb.WriteString(profNameTableC(g.profNames))
				sb.WriteString(profRuntimeC())
			}
			for _, stmt := range prog.Statements {
				if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
					sb.WriteString(g.genKernelDecl(kernel))
				}
			}
		}
		// Bodies owned by this module.
		own := map[string]bool{}
		for _, name := range funcFiles[f] {
			if owner[name] == f {
				own[name] = true
			}
		}
		g.emitFuncBodies(&sb, prog, own)
		if isRoot {
			res.RootTU = sb.String()
		} else {
			res.ModuleTUs[f] = sb.String()
		}
	}
	// simdNeedsAVX is discovered during body emission (256-bit lane use),
	// so it is read here, after all TUs are emitted.
	res.SimdNeedsAVX = g.simdNeedsAVX
	return res, nil
}

// sameFile reports whether f and root name the same file.
func sameFile(f, root, rootAbs string) bool {
	if f == root || filepath.Clean(f) == filepath.Clean(root) {
		return true
	}
	absp, err := filepath.Abs(f)
	if err != nil {
		return false
	}
	return absp == rootAbs
}

// avxKernelProtos declares the fixed AVX kernel bodies living in runtime.c
// so any module TU may call them.
func avxKernelProtos() string {
	return "void matrix_mul_avx2(double* A, double* B, double* C, int64_t rowsA, int64_t colsA, int64_t colsB);\n" +
		"void matrix_mul_scalar(double* A, double* B, double* C, int64_t rowsA, int64_t colsA, int64_t colsB);\n"
}
