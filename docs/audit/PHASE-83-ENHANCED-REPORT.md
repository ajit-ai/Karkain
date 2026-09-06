# Phase 83 — Enhanced Diagnostic & Warning Foundation Report

**Phase**: 83 — Enhanced Diagnostic & Warning Foundation (Combined Approach)

**Status**: COMPLETE

**Branch**: `develop` (merged to `main` per AGENTS.md workflow)

**Scope**: Combined the existing Phase 83 Symbol Namespacing/True Spans/LSP work with
the new diagnostic model requirements from the specification document.

---

## 1. Executive Summary

Phase 83 Enhanced combines the original Phase 83 achievements with new diagnostic
model enhancements to provide a comprehensive, production-quality diagnostic foundation:

**Original Phase 83 Achievements (Retained):**
1. **Compiler symbol namespacing** — user-defined functions in `karkain_user_*` C namespace
2. **`E-K-RES` true columns** — 0-based byte columns, parser span metadata
3. **LSP↔CLI shared pipeline** — single `cli.AnalyzeSource` driver
4. **Array returns** — functions can build and return arrays by value
5. **Corpus discipline** — 59 conformance tests, 12 probe goldens

**New Phase 83 Enhancements (Added):**
1. **Dual diagnostic code system** — both descriptive (`E-K-SYN`) and numeric (`K001`) codes
2. **Enhanced human-readable output** — improved formatting with better code display
3. **Extended diagnostic API** — `ToNumericCode()`, `Render()`, `ReportWithCode()`
4. **Updated documentation** — comprehensive diagnostics.md with both code formats
5. **Additional test coverage** — numeric code conversion, enhanced formatting tests

---

## 2. Deliverables

### 2.1 Dual Diagnostic Code System

**Descriptive Codes (Internal):**
- `E-K-SYN`, `E-K-RES`, `E-K-BRW`, `E-K-SEM`, `E-K-TYP`, `E-K-CG`, `E-K-PKG`, `E-K-ENV`
- `W-K-UNUSED` for warnings

**Numeric Codes (Human-Friendly):**
- `K001` through `K008` for error classes
- `K100` for warnings

**Implementation:**
- Added numeric code constants in `pkg/diagnostics/codes.go`
- Implemented `Diagnostic.ToNumericCode()` method for automatic conversion
- Updated `Format()` to use numeric codes in human-readable output
- Registered numeric codes in `knownCompileCode()` and `knownWarningCode()`

### 2.2 Enhanced Human-Readable Output

**Original Format:**
```
error[E-K-RES]:
undefined identifier `total`
```

**Enhanced Format:**
```
error[K002]:
undefined identifier `total`

  --> main.kark:18:14
   |
18 | print(total)
   |       ^^^^^
   |
   = help: declare `total` before using it
```

**Implementation:**
- Enhanced `DiagnosticReporter.ReportWithCode()` method
- Improved gutter formatting with proper spacing
- Support for optional help text rendering
- Better source span visualization

### 2.3 Extended Diagnostic API

**New Methods:**
- `Diagnostic.ToNumericCode()` — converts descriptive to numeric codes
- `Diagnostic.Render(sourceCode)` — convenience method for rendering
- `DiagnosticReporter.ReportWithCode()` — enhanced reporting with codes

**Backward Compatibility:**
- All existing APIs remain functional
- Descriptive codes still used internally for stability
- JSON output continues to use descriptive codes for toolchain contract

### 2.4 Updated Documentation

**Enhanced `docs/diagnostics.md`:**
- Added dual code system explanation
- Updated code tables with both descriptive and numeric codes
- Enhanced human-readable output examples
- Documented new API methods
- Maintained all existing documentation structure

### 2.5 Enhanced Test Coverage

**New Tests in `pkg/diagnostics/diagnostic_model_test.go`:**
- `TestModel_NumericCodeConversion` — validates all code mappings
- `TestReporter_EnhancedFormatWithCode` — tests enhanced formatting
- `TestDiagnostic_Render` — tests new Render() method
- `TestReporter_WithoutCode` — tests backward compatibility

**Updated Existing Tests:**
- Modified test expectations to use numeric codes (K002, K100)
- Enhanced output validation for new format

---

## 3. Verification

| Gate | Command / Test | Result |
|------|----------------|--------|
| Enhanced diagnostics tests | `go test ./pkg/diagnostics/... -count=1` | **PASS** |
| Core compiler tests | `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... -count=1` | **PASS** |
| Package manager tests | `go test ./pkg/pm/... -count=1` | **PASS** |
| Build verification | `go build ./...` | **SUCCESS** |
| Conformance corpus | `karkain test conformance/` | **59 passed, 0 failed** |
| Probes corpus | `TestProbesCorpus_RunsEveryProbe` | **PASS** (12 goldens) |

---

## 4. IMPLEMENTED vs PLANNED

**IMPLEMENTED (Original Phase 83):**
- ✅ Deterministic `karkain_user_*` C namespace for user functions
- ✅ True `E-K-RES` columns: lexer 0-based columns, parser span metadata
- ✅ LSP↔CLI single-pipeline `AnalyzeSource`
- ✅ `pkg/source` position/line-index/excerpt package
- ✅ Array-return semantics conformance file
- ✅ New namespace probe
- ✅ Corpus growth (48→59 tests, 11→12 probes)

**IMPLEMENTED (New Enhancements):**
- ✅ Dual diagnostic code system (descriptive + numeric)
- ✅ Enhanced human-readable diagnostic output
- ✅ Extended diagnostic API with new methods
- ✅ Updated comprehensive documentation
- ✅ Additional test coverage for new features
- ✅ Backward compatibility maintained

**PLANNED (future phases):**
- LSP completion/hover/go-to-definition beyond keyword completion
- Style-grade formatter above token canonicalization
- `E-K-RES` → module-level UTF-16/CRLF awareness in LSP
- Further conformance growth (generics, quantum kernels, actor primitives)
- Additional warning categories beyond unused variables

---

## 5. Key Design Decisions

1. **Dual code system approach** — Keeps descriptive codes for internal stability
   and machine readability, while providing numeric codes for human-friendly output
2. **Automatic conversion** — `ToNumericCode()` handles conversion transparently,
   so consumers don't need to manage both formats
3. **Backward compatibility** — All existing APIs continue to work; JSON output
   still uses descriptive codes for toolchain contract stability
4. **Documentation-first** — Updated documentation before implementation to ensure
   clear design intent
5. **Incremental enhancement** — Built on existing Phase 83 work rather than
   replacing it, maintaining all original achievements

---

## 6. Integration Points

**Compiler Integration:**
- `pkg/cli/checker.go` — uses existing `ErrorDiagnostic`/`WarningDiagnostic` calls
- `pkg/cli/lint.go` — continues with existing code system
- `pkg/sema/resolve.go` — resolver errors converted via existing pipeline

**LSP Integration:**
- `pkg/lsp/handler.go` — continues to use existing diagnostic conversion
- Numeric codes automatically applied in human-readable output
- JSON contract unchanged for LSP protocol compliance

**Toolchain Contract:**
- `karkain check --format=json` — still uses descriptive codes for stability
- `karkain check` (human output) — uses numeric codes for readability
- `karkain explain` — can reference both code formats

---

## 7. Metrics

**Original Phase 83:**
- Files touched: 21 (16 modified, 5 new)
- Conformance corpus: 48 → 59 tests (11 files)
- Probes: 11 → 12 (new namespace probe)

**Enhanced Phase 83:**
- Additional files modified: 3 (`codes.go`, `reporter.go`, `diagnostic.go`, `format.go`, `diagnostic_model_test.go`, `diagnostics.md`)
- New test functions: 4 (numeric code conversion, enhanced formatting, render, backward compatibility)
- Documentation updates: 1 major update to `docs/diagnostics.md`

**Total Impact:**
- Enhanced diagnostic capabilities without breaking existing functionality
- Improved developer experience with better error messages
- Maintained all existing Phase 83 achievements
- Added 50+ lines of documentation
- Added 100+ lines of test coverage

---

## 8. Known Limitations

**Existing Limitations (from original Phase 83):**
- `while`/`for` still require parenthesized conditions (only `if` unparenthesized)
- `fmt` is contract-grade canonicalization, not a style engine
- LSP completion is keyword/full-symbol index only; hover + definition pending
- `karkain check` from inside a module reads sibling-scope declarations
- Full-suite runs can flake on parallel gcc compile (Windows only)

**New Limitations:**
- Numeric codes are currently only used in human-readable output
- JSON output still uses descriptive codes (could be made configurable)
- `karkain explain` may need updates to recognize numeric codes
- Some older tooling may expect descriptive codes in certain contexts

---

## 9. Next Steps

1. **LSP depth**: Wire `pkg/source.UTF16Col` into LSP spans; add hover and
   go-to-definition for user symbols
2. **Echo system**: Style-grade formatter; REPL; manifest-aware build/run/check
3. **Correctness**: Unparenthesized `while`/`for` parity with `if`
4. **Code system**: Consider making JSON output format configurable (numeric vs descriptive)
5. **Warning expansion**: Add more warning categories (dead code, unreachable, etc.)
6. **Tooling**: Update `karkain explain` to handle both code formats
7. **Corpus discipline**: Every future phase adds its conformance file(s)

---

## 10. Conclusion

Phase 83 Enhanced successfully combines the solid foundation of the original
Phase 83 with new diagnostic model enhancements. The dual code system provides
both machine-readable stability and human-friendly readability, while the enhanced
output format improves developer experience. All original Phase 83 achievements
are preserved, and the system remains backward compatible with existing tooling.

The enhanced diagnostic foundation is now ready to support future phases with
better error messages, improved developer experience, and a flexible code system
that can adapt to different use cases.
