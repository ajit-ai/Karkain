# Karkain Example Inventory

Authoritative list of the 15-category example corpus. Jargon:

- **Status**: `Runnable` (byte-identical on both engines and pinned by
  `pkg/cli/phase114_examples_test.go`), `Experimental` (works through the Go
  front end; runnable but not kcc-parity yet), `Planned` (no `.kark` source,
  honest README only).
- **Engine**: `both` = Go front end + self-hosted `kcc` reproduce identical
  stdout; `go` = Go front end only.
- **Target**: default native (x86_64-windows host). `wasm32-wasi` is a
  separate Phase 108/123 corpus (`examples/wasm`) — Go-engine production
  candidate, pinned by `pkg/cli/phase123_cli_test.go` (`TestPhase123_Wasm*`).

## Category map

| # | Category               | Files | Runnable | Experimental | Planned |
|---|------------------------|-------|----------|--------------|---------|
| 01| Fundamentals           | 15    | 15       | 0            | 0       |
| 02| Algorithms             | 13    | 13       | 0            | 0       |
| 03| Systems                | 1     | 1        | 0            | 0       |
| 04| Networking             | 2     | 2        | 0            | 0       |
| 05| Data                   | 4     | 4        | 0            | 0       |
| 06| Database               | 2     | 2        | 0            | 0       |
| 07| Web                    | 2     | 2        | 0            | 0       |
| 08| Concurrency            | 2     | 0        | 2            | 0       |
| 09| AI                     | 3     | 3        | 0            | 0       |
| 10| Machine Learning       | 2     | 2        | 0            | 0       |
| 11| Quantum                | 0     | 0        | 0            | 1 (dir) |
| 12| Scientific Computing   | 4     | 4        | 0            | 0       |
| 13| Finance                | 4     | 4        | 0            | 0       |
| 14| Security               | 4     | 4        | 0            | 0       |
| 15| Developer Tools        | 2     | 1 + 1    | 0            | 0       |
|    | **Total**              | **59**| **56 + 1 test-mode** | **2**  | **1**   |

## Individual examples

| Example | Status | Engine | Feature demonstrated | Dependencies | Expected output |
|---------|--------|--------|----------------------|--------------|-----------------|
| 01-fundamentals/01_hello_world.kark | Runnable | both | print, main | gcc | `Hello, Karkain!` |
| 01-fundamentals/02_variables.kark | Runnable | both | let, reassignment | gcc | 42 karkain 123 |
| 01-fundamentals/03_constants.kark | Runnable | both | const bindings | gcc | 540 8 |
| 01-fundamentals/04_functions.kark | Runnable | both | funcs, recursion | gcc | 49 7 negative non-negative 120 1 |
| 01-fundamentals/05_conditionals.kark | Runnable | both | if/else-if chains | gcc | A B C D F even |
| 01-fundamentals/06_loops.kark | Runnable | both | while loops | gcc | 0..4 55 [3, 2, 1] |
| 01-fundamentals/07_strings.kark | Runnable | both | concat/len/index/slice | gcc | karkain 7 k i kark computed: 42 |
| 01-fundamentals/08_arrays.kark | Runnable | both | arrays, slices, 2D | gcc | 5 2 11 [3, 5, 7] 0 [0, 1, 4, 9, 16] 5 3 |
| 01-fundamentals/09_maps.kark | Runnable | both | maps + std.collections | gcc, std | 92 84 97 2 1 0 3 ana bob cam |
| 01-fundamentals/10_structs.kark | Runnable | both | struct records | gcc | ana 100 150 150 130 |
| 01-fundamentals/11_match.kark | Runnable | both | match + Options | gcc | 300 7 1000000 three |
| 01-fundamentals/12_casts.kark | Runnable | both | int/float/str casts | gcc | 3 3.4 4.5 9 7 256 3 3.5 640 0 |
| 01-fundamentals/13_enums.kark | Runnable | both | enum variants, matching | gcc | 1 1 100 200 300 0 |
| 01-fundamentals/14_adt_match.kark | Runnable | both | ADT-style tagged match | gcc | 10 20 30 1 |
| 01-fundamentals/15_closures.kark | Runnable | both | let-bound closures, captures | gcc | 42 42 42 42 16 1 |
| 02-algorithms/01_linear_search.kark | Runnable | both | linear search | gcc | 2 3 -1 |
| 02-algorithms/02_binary_search.kark | Runnable | both | halving search | gcc | 3 0 7 -1 -1 -1 |
| 02-algorithms/03_min_max.kark | Runnable | both | min/max | gcc | 1 12 -9 -1 |
| 02-algorithms/04_bubble_sort.kark | Runnable | both | bubble sort | gcc | [1, 2, 3, 5, 7, 8, 9] [1, 2, 3, 4] [1, 2, 3, 4] |
| 02-algorithms/05_frequency_count.kark | Runnable | both | map tally | gcc, std | 3 2 1 4 |
| 02-algorithms/06_fibonacci.kark | Runnable | both | iterative fib | gcc | 0 1 5 55 610 6765 |
| 02-algorithms/07_factorial.kark | Runnable | both | iterative factorial | gcc | 1 1 120 40320 479001600 |
| 02-algorithms/08_gcd.kark | Runnable | both | Euclid gcd/lcm | gcc | 6 1 20 12 42 |
| 02-algorithms/09_prime_sieve.kark | Runnable | both | Eratosthenes sieve | gcc | [2, 3, 5, 7] 10 [2, 3, 5, 7, 11, 13, 17, 19, 23, 29] |
| 02-algorithms/10_palindrome.kark | Runnable | both | reversal, equality | gcc | 1 0 1 1 olleh |
| 02-algorithms/11_stack.kark | Runnable | both | array stack (LIFO) | gcc | 3 30 20 10 0 |
| 02-algorithms/12_queue.kark | Runnable | both | array queue (FIFO) | gcc | 10 20 50 20 30 40 50 |
| 02-algorithms/13_knapsack.kark | Runnable | both | 0/1 knapsack (DP) | gcc | 7 9 |
| 03-systems/01_file_io.kark | Runnable | both | std.io write/read/delete | gcc, std | true 3 alpha beta gamma true |
| 04-networking/01_tcp_echo.kark | Runnable | both | std.net TCP echo | gcc | 4 ping 4 pong 3 |
| 04-networking/02_tcp_roundtrip.kark | Runnable | both | std.net request/response | gcc | 3 one 3 two 3 |
| 05-data/01_word_frequency.kark | Runnable | both | tokenize + tally | gcc, std | 3 2 2 1 8 |
| 05-data/02_csv_aggregate.kark | Runnable | both | CSV-ish parse + aggregate | gcc, std | 15 20 15 2 50 20 |
| 05-data/03_payload_roundtrip.kark | Runnable | both | hex/base64/utf8 | gcc, std | hex round-trips (see file) |
| 05-data/04_token_stats.kark | Runnable | both | tokenize + filter/aggregate | gcc, std | 9 3 35 |
| 06-database/01_db_crud.kark | Runnable | both | std.db in-memory CRUD | gcc | 3 1 alice 90 2 bob 80 3 carol 95 100 alice carol db2 embedded |
| 06-database/02_db_persist.kark | Runnable | both | std.db file-backed persistence | gcc | true 2 1 alpha 2 beta |
| 07-web/01_http_loopback.kark | Runnable | both | std.http server loopback | gcc | 90 GET /hello ok 0 70 200 OK hello /hello |
| 07-web/02_http_codec.kark | Runnable | both | std.http constructors/destructors | gcc | GET /items text/plain 3 201 Created ok |
| 08-concurrency/01_parallel_sum.kark | Experimental | go | spawn/join grid | gcc | 285 |
| 08-concurrency/02_channel_ping.kark | Experimental | go | channel producer | gcc | 5 0 8 |
| 09-ai/01_nearest_neighbor.kark | Runnable | both | Manhattan k-NN | gcc | 10 60 100 60 |
| 09-ai/02_linear_classifier.kark | Runnable | both | perceptron updates | gcc | 5 1 -1 2 -3 |
| 10-machine-learning/01_linear_regression.kark | Runnable | both | OLS closed form | gcc | 0.9 1.3 6.7 |
| 10-machine-learning/02_gradient_descent.kark | Runnable | both | loss minimization | gcc | 3 7 |
| 10-machine-learning/03_mlp_forward.kark | Runnable | both | 2-layer forward pass on std.numerics | gcc | 0 0 0 0 1 3 |
| 12-scientific-computing/01_sqrt_newton.kark | Runnable | both | Newton sqrt | gcc | 1.41421 3 2 0.5 |
| 12-scientific-computing/02_numerical_integration.kark | Runnable | both | trapezoid rule | gcc | 0.34375 0.333374 0.333333 |
| 12-scientific-computing/03_statistics.kark | Runnable | both | mean/variance/stddev | gcc | 5 4 2 |
| 12-scientific-computing/04_matrix_multiply.kark | Runnable | both | integer matmul | gcc | rows + corner values |
| 13-finance/01_compound_interest.kark | Runnable | both | compound interest | gcc | 115762 231854 20000 10000 |
| 13-finance/02_loan_amortization.kark | Runnable | both | annuity payment | gcc | 12950.5 11231.4 12754.8 |
| 13-finance/03_npv.kark | Runnable | both | discounted cash flows | gcc | -454.545 3992.71 |
| 13-finance/04_portfolio.kark | Runnable | both | weighted returns | gcc | 5 3 7 |
| 14-security/01_digests.kark | Runnable | both | sha256/sha512 (NIST) | gcc, std | NIST vectors |
| 14-security/02_encoding_roundtrip.kark | Runnable | both | RFC 4648 hex/base64 | gcc, std | round-trips |
| 14-security/03_password_hash.kark | Runnable | both | salt+sha256 demo | gcc, std | 64-hex + booleans |
| 14-security/04_utf8_text.kark | Runnable | both | UTF-8 encode/validate | gcc, std | 1 636166c3a9 1 CAFé |
| 15-developer-tools/01_hello_toolchain.kark | Runnable | both | toolchain loop | gcc | write -> build -> run |
| 15-developer-tools/02_assertions_test.kark | Runnable | both | karkain test | gcc | 3 passed |

## Planned categories (README only, no `.kark`)

| Category | Honest status |
|----------|---------------|
| 11 Quantum | parser/lexer do not wire circuit/qubit/qpu; infrastructure only |

## Sibling categories (outside the 15-category pin)

| Category | Status |
|----------|--------|
| `closures/` | capture mutation + nested closures, golden byte-identical both engines (Phase 130, pinned by `pkg/cli/phase130_closures_test.go`) |
| `profiling/`, `stdlib_v2/`, `module_system/`, `language_foundation/`, `wasm/`, `self-hosting/` | dedicated phase gates (110 / 109 / 103 / 102 / 108+123 / 120) |

## Verification

```powershell
karkain run examples/01-fundamentals/01_hello_world.kark
scripts\verify-examples.ps1
go test ./pkg/cli/ -run Phase114 -count=1
```