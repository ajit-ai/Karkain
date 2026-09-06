# Diagnostics

Karkain compiler diagnostics — the shared model, severity levels, codes,
source locations, and the human-readable report format. One model drives the
CLI (`karkain check`, `karkain lint`), the JSON wire contract
(`check --format=json`), and the language server.

## 1. Diagnostic Model

Every compiler finding is a single structured `Diagnostic`:

| field       | meaning                                                        |
|-------------|----------------------------------------------------------------|
| `file`      | path of the source file the finding refers to                  |
| `line`      | 1-based source line                                            |
| `column`    | 1-based token-start column (optional span start)               |
| `endColumn` | optional 1-based column just past the token (span end)         |
| `excerpt`   | optional captured/trimmed source line for the report           |
| `severity`  | `error`, `warning`, `note`, or `help`                          |
| `code`      | stable machine-readable code (`E-K-*` errors, `W-K-*` warnings)|
| `message`   | human-readable text                                            |
| `help`      | optional remediation text                                      |
| `related`   | optional supplemental diagnostics (notes about a finding)      |

`line` + `column` + `endColumn` form the optional source span. When
`endColumn` is 0 the span is unknown and the report underlines a single
column.

The model lives in `pkg/diagnostics`. Errors and warnings share it; there is
no second, ad-hoc diagnostic system. Constructors:

- `ErrorDiagnostic(file, line, col, code, msg)`
- `WarningDiagnostic(file, line, col, code, msg)`
- `NoteDiagnostic(file, line, col, msg)`
- `d.WithHelp(text)` / `d.WithRelated(diag...)`

## 2. Severity Levels

| severity | meaning                                                  |
|----------|----------------------------------------------------------|
| `error`  | compilation stops; a non-zero exit code is returned      |
| `warning`| reported but compilation continues; exit code stays 0    |
| `note`   | supplemental context attached to a primary finding       |
| `help`   | supplemental remediation text attached to a finding      |

Warnings and errors are collected with the same infrastructure. The compiler
displays warnings without terminating, and continues compilation when it can
safely run later stages.

## 3. Diagnostic Codes

Codes are stable, machine-readable, and Karkain-owned (`K` namespace). They
are documented via `karkain explain <code>`.

Karkain supports two code formats:

1. **Descriptive codes** (`E-K-SYN`, `E-K-RES`, etc.) - detailed category names
2. **Numeric codes** (`K001`, `K002`, etc.) - compact, human-friendly alternatives

The diagnostic model uses descriptive codes internally, but the human-readable
output and `Format()` functions automatically convert to numeric codes for
better readability.

Compiler error classes:

| descriptive code | numeric code | class                                    |
|------------------|--------------|------------------------------------------|
| `E-K-SYN`        | `K001`       | lexer/parser: malformed source           |
| `E-K-RES`        | `K002`       | name resolution (undefined/duplicate/private) |
| `E-K-BRW`        | `K003`       | borrow checker (lifetime/ownership)      |
| `E-K-SEM`        | `K004`       | semantic analysis (kernels/actors/…)     |
| `E-K-TYP`        | `K005`       | type checking                            |
| `E-K-CG`         | `K006`       | code generation / backend emission       |
| `E-K-PKG`        | `K007`       | package/dependency integration           |
| `E-K-ENV`        | `K008`       | toolchain infrastructure                 |

Warning classes (`W-K-*`):

| descriptive code | numeric code | class                            |
|------------------|--------------|----------------------------------|
| `W-K-UNUSED`     | `K100`       | name declared but never read     |

The `Diagnostic.ToNumericCode()` method converts descriptive codes to their
numeric equivalents. Unknown codes pass through unchanged.

## 4. Errors vs Warnings

- **Errors** short-circuit the pipeline: `check` stops at the first failing
  stage (syntax → resolution → semantics) and reports that stage's findings.
  Exit code is non-zero (`ExitCompile`).
- **Warnings** are computed only for programs that resolved successfully, so
  an undefined name cannot cascade into bogus warnings. They never change the
  exit code. `karkain check` renders them and still succeeds.
- The JSON mode follows the same rule: error arrays exit non-zero; warning
  arrays exit 0 (`[]` on a fully clean file).

The only Phase 83 warning producer is unused-variable detection
(`sema.UnusedVars`, per `let`/`var` declaration, span-accurate). It is
deliberately conservative: parameters and loop variables are exempt, and a
function containing a construct the walker does not understand emits nothing
for that function. Future warning categories plug into the same model and are
expected to keep this contract.

## 5. Source Locations

- Lines are 1-based everywhere (JSON, human report, LSP has no lines — it
  consumes 0-based ranges).
- Columns are emitted 1-based on the wire; the lexer/parser compute true
  0-based byte-column spans internally, so `column`/`endColumn` point at the
  real token, not a heuristic.
- The LSP adapter converts `(line, column, endColumn)` to 0-based UTF-16
  ranges and maps severity to the LSP scale.

## 6. Human-Readable Output

`karkain check` renders diagnostics to stderr in a fixed report shape using
numeric codes for better readability:

```
error[K002]:
undefined identifier `total`

  --> main.kark:18:14
   |
18 | print(total)
   |       ^^^^^
   |
   = help: declare `total` before using it

warning[K100]:
unused variable `count`

  --> main.kark:7:5
   |
7  | let count = 10
   |     ^^^^^
```

- Header carries severity and numeric code: `error[K002]:`, `warning[K100]:`.
- `--> file:line:column` is the true location.
- The gutter shows the source line; the caret underline spans the token
  (`column..endColumn`, at least one caret).
- `= help:` and `= note:` fringe lines carry remediation and related context.

The enhanced `DiagnosticReporter.ReportWithCode()` method supports both formats:
- With code: `error[K002]: message`
- Without code: `error: message`

The renderer is `diagnostics.Format` / `diagnostics.FormatDiagnostics`; it is
used by `karkain check` and is available to every consumer of the model.
