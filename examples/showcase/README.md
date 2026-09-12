# Karkain Developer Showcase — `examples/showcase/`

Real, proven `.kark` programs written against the **current** Karkain toolchain
and validated with the real CLI (default `kcc` engine). These are working
examples first and marketing second: every `STATUS: WORKING TODAY` program
below was checked and run with `karkain check` / `karkain run`.

- Toolchain: `karkain.exe` v0.115.0 (Developer Preview Build, default engine `kcc`, self-hosted), MSYS2 gcc 14.2.0.
- Every working example is ASCII-clean and deterministic (pinnable output).
- Nothing here invents syntax, libraries, or features the language does not have.

## How to run any example

```
karkain check examples/showcase/<nn>_<cat>/<name>/main.kark
karkain run   examples/showcase/<nn>_<cat>/<name>/main.kark
```

`karkain run` compiles to C23 in a temporary sandbox and executes it, so file
I/O is self-contained inside the program.

## Capability status matrix (15 categories)

| # | Category | Status | Evidence |
|---|----------|--------|----------|
| 01 | Fundamentals | WORKING TODAY | `01-fundamentals/hello` — types, expressions, conditionals, loops, recursion, arrays |
| 02 | Algorithms | WORKING TODAY | `02-algorithms/queue` — FIFO queue on arrays |
| 03 | Systems | WORKING TODAY | `03-systems/file_processor` — writeFile/readFile/split/removeFile/getArgs |
| 04 | Networking | NOT CURRENTLY SUPPORTED | No socket/HTTP runtime; `http.get` is recognized as a builtin name but has no network implementation — do not build on it |
| 05 | Data | WORKING TODAY | `05-data/text_stats` — tokenizing, longest-word search, float stats (arrays, not maps: there is no map iteration API) |
| 06 | Database | NOT CURRENTLY SUPPORTED | No DB drivers, no query language, no persistence beyond flat file I/O (`readFile`/`writeFile`/`readLine`) |
| 07 | Web | NOT CURRENTLY SUPPORTED | No HTTP server or web framework; no JSON parsing (an `http.get` name exists but is unimplemented) |
| 08 | Concurrency | NOT CURRENTLY SUPPORTED | No goroutines/threads/actors in the runtime; queues in example 02 are a pure data structure |
| 09 | AI | WORKING TODAY | `09-ai/matrix` — dot product, matmul, cosine similarity, nearest-neighbor; `@target(cpu)` attribute |
| 10 | ML | WORKING TODAY | `10_ml/linear_regression` — closed-form least squares + prediction |
| 11 | Quantum | NOT CURRENTLY SUPPORTED | No quantum runtime/adapters; nothing quantum-real exists to showcase |
| 12 | Scientific | WORKING TODAY | `12_scientific/numerics` — Newton sqrt, trapezoid integration, statistics, deterministic Monte Carlo pi |
| 13 | Finance | WORKING TODAY | `13-finance/finance` — compound interest, moving average, volatility, EMI |
| 14 | Security | WORKING TODAY (non-cryptographic) | `14-security/checksum` — deterministic rolling-hash checksum + validation. NO crypto primitives exist (no hash/HMAC/cipher/secure RNG) |
| 15 | Developer Tools | WORKING TODAY | `15_devtools` — multi-file app, `test`, formatter |

## Example inventory

Working examples (10 program suites, validated):

| Dir | What it shows | Expected output (verified) |
|-----|---------------|----------------------------|
| `01-fundamentals/hello` | dynamic typing, `float` annotations, if/else-if/else, while, recursion (fib), arrays, len/push/index | see `01-fundamentals/hello/README.md` |
| `02-algorithms/queue` | FIFO queue built on an array; array pass-by-value semantics | `10 0 10 20 30 1` |
| `03-systems/file_processor` | writeFile/readFile/split/removeFile/getArgs pipeline | `lines=3 first=alpha last=gamma args=1 cleaned` |
| `05-data/text_stats` | tokenize + longest-word + float averages over a document | `characters=43 words=9 longest=quick avg_word_length=3.88889` |
| `09-ai/matrix` | dot, matmul, cosine similarity (Newton float sqrt), nearest-neighbor, `@target(cpu)` | `dot=32 ... cosine=0.974632 nearest_q0=1 nearest_q1=2` |
| `10_ml/linear_regression` | least-squares fit + prediction on `y = 2x + 1` | `slope=2 intercept=1 predict(10)=21 predict(-3)=-5` |
| `12_scientific/numerics` | Newton sqrt, trapezoid integration, mean/variance/stddev, LCG Monte Carlo | `sqrt_2=1.41421 ... pi_estimate=3.1692` |
| `13-finance/finance` | compounding, MA(3), vol, EMI in floats | `compound_1000@5pct_10y=1628.89 ... emi_100k@8pct_12mo=8698.84` |
| `14-security/checksum` | deterministic rolling hash; canonical unsigned-int validation; password heuristic | `checksum=381823039 ... valid_12345=1 valid_12a45=0 ...` |
| `15_devtools/banking` | multi-file app — `check`/`build`/`run` with sibling modules | `opening=$1000 after_credit=$1250 ... comfortable` |
| `15_devtools/tests` | `karkain test` — 4 self-contained `test_*` functions | `4 passed; 0 failed; 0 skipped; 4 total` |
| `15_devtools/formatting` | `karkain fmt` / `fmt --check` on deliberately messy code | `--check` exit 1 before, exit 0 after |

Not present (honest `NOT CURRENTLY SUPPORTED` markers): 04 Networking,
06 Database, 07 Web, 08 Concurrency, 11 Quantum — each above with the reason.

## Real capability findings discovered while writing this showcase

1. **`int(string)` does not work on the default (self-hosted `kcc`) engine.**
   The Go engine handles it, but the kcc pipeline mis-parses it (it is also a
   type keyword) and emits invalid C. The kcc parity corpora never exercised
   `int(...)`, which is why this gap went unnoticed. Working workarounds used
   here: character-range validation (14), arithmetic stays numeric (09-13).
2. **Arrays are passed by value into functions.** `push(arr, v)` inside a
   helper does not reach the caller's array; append via `arr = push(arr, v)`
   (returned array), as in `02-algorithms/queue`. Direct `push` on a variable
   in the same scope does mutate in place.
3. **No map iteration** — no `mapKeys`/`mapEntries`. Use parallel arrays for
   frequency/aggregation (05).
4. **No `ord`/char-code, no int-to-float cast builtin.** Float conversion via
   `x / 1.0`-style float arithmetic is deterministic and works (09-13).
5. **No randomized or wall-clock builtins.** `12_scientific/numerics` uses a
   deterministic LCG so output is reproducible everywhere.
6. **`karkain fmt` writes non-ASCII comment characters as ANSI**, corrupting
   UTF-8 em-dashes et al. Showcase sources are kept ASCII-only.
7. **`karkain test` test files are self-contained compilation units.** The
   conformance convention holds; same-directory sibling helpers are pulled
   into each test driver and collide, so helper functions are declared inside
   the test file (see `15_devtools/tests`).
8. **`getArgs()` includes the program name as element 0**, so without extra
   arguments it reports `args=1`.
9. Runtime failure model: division by zero is silent, out-of-range reads
   return 0, and there are no source-mapped runtime stack traces (only
   `assert*` failures point at `[file:line]`).

## AI / ML / Quantum — explicit assessment

- **AI**: working for classical vector/matrix math (dot, matmul, similarity,
  nearest neighbor). No tensors, no autodiff surface usable from `.kark`
  today, no model format.
- **ML**: working for closed-form / direct numeric fitting (least squares,
  statistics). No stochastic training, no neural-network runtime, no data
  loading beyond text files.
- **Quantum**: genuine NOT SUPPORTED. Do not present otherwise.

See the audit report for the full developer-journey matrix.