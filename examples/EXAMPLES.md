# Karkain Example Inventory

Authoritative list of the 15-category example corpus. Jargon:

- **Status**: `Runnable` (byte-identical on both engines and pinned by
  `pkg/cli/phase114_examples_test.go`), `Experimental` (works through the Go
  front end; runnable but not kcc-parity yet), `Planned` (no `.kark` source,
  honest README only).
- **Engine**: `both` = Go front end + self-hosted `kcc` reproduce identical
  stdout; `go` = Go front end only.
- **Target**: default native (x86_64-windows host). `wasm32-wasi` is a
  separate Phase 108 corpus (`examples/wasm`).

## Category map

| # | Category               | Files | Runnable | Experimental | Planned |
|---|------------------------|-------|----------|--------------|---------|
| 01| Fundamentals           | 12    | 12       | 0            | 0       |
| 02| Algorithms             | 10    | 10       | 0            | 0       |
| 03| Systems                | 1     | 1        | 0            | 0       |
| 04| Networking             | 0     | 0        | 0            | 1 (dir) |
| 05| Data                   | 3     | 3        | 0            | 0       |
| 06| Database               | 0     | 0        | 0            | 1 (dir) |
| 07| Web                    | 0     | 0        | 0            | 1 (dir) |
| 08| Concurrency            | 2     | 0        | 2            | 0       |
| 09| AI                     | 2     | 2        | 0            | 0       |
| 10| Machine Learning       | 2     | 2        | 0            | 0       |
| 11| Quantum                | 0     | 0        | 0            | 1 (dir) |
| 12| Scientific Computing   | 4     | 4        | 0            | 0       |
| 13| Finance                | 4     | 4        | 0            | 0       |
| 14| Security               | 4     | 4        | 0            | 0       |
| 15| Developer Tools        | 2     | 1 + 1    | 0            | 0       |
|    | **Total**              | **46**| **43 + 1 test-mode** | **2**  | **4**   |

## Individual examples

| Example | Status | Engine | Feature demonstrated | Dependencies | Expected output |
|---------|--------|--------|----------------------|--------------|-----------------|
| 01_fundamentals/01_hello_world.kark | Runnable | both | print, main | gcc | `Hello, Karkain!` |
| 01_fundamentals/02_variables.kark | Runnable | both | let, reassignment | gcc | 42 karkain 123 |
| 01_fundamentals/03_constants.kark | Runnable | both | const bindings | gcc | 540 8 |
| 01_fundamentals/04_functions.kark | Runnable | both | funcs, recursion | gcc | 49 7 negative non-negative 120 1 |
| 01_fundamentals/05_conditionals.kark | Runnable | both | if/else-if chains | gcc | A B C D F even |
| 01_fundamentals/06_loops.kark | Runnable | both | while loops | gcc | 0..4 55 [3, 2, 1] |
| 01_fundamentals/07_strings.kark | Runnable | both | concat/len/index/slice | gcc | karkain 7 k i kark computed: 42 |
| 01_fundamentals/08_arrays.kark | Runnable | both | arrays, slices, 2D | gcc | 5 2 11 [3, 5, 7] 0 [0, 1, 4, 9, 16] 5 3 |
| 01_fundamentals/09_maps.kark | Runnable | both | maps + std.collections | gcc, std | 92 84 97 2 1 0 3 ana bob cam |
| 01_fundamentals/10_structs.kark | Runnable | both | struct records | gcc | ana 100 150 150 130 |
| 01_fundamentals/11_match.kark | Runnable | both | match + Options | gcc | 300 7 1000000 three |
| 01_fundamentals/12_casts.kark | Runnable | both | int/float/str casts | gcc | 3 3.4 4.5 9 7 256 3 3.5 640 0 |
| 02_algorithms/01_linear_search.kark | Runnable | both | linear search | gcc | 2 3 -1 |
| 02_algorithms/02_binary_search.kark | Runnable | both | halving search | gcc | 3 0 7 -1 -1 -1 |
| 02_algorithms/03_min_max.kark | Runnable | both | min/max | gcc | 1 12 -9 -1 |
| 02_algorithms/04_bubble_sort.kark | Runnable | both | bubble sort | gcc | [1, 2, 3, 5, 7, 8, 9] [1, 2, 3, 4] [1, 2, 3, 4] |
| 02_algorithms/05_frequency_count.kark | Runnable | both | map tally | gcc, std | 3 2 1 4 |
| 02_algorithms/06_fibonacci.kark | Runnable | both | iterative fib | gcc | 0 1 5 55 610 6765 |
| 02_algorithms/07_factorial.kark | Runnable | both | iterative factorial | gcc | 1 1 120 40320 479001600 |
| 02_algorithms/08_gcd.kark | Runnable | both | Euclid gcd/lcm | gcc | 6 1 20 12 42 |
| 02_algorithms/09_prime_sieve.kark | Runnable | both | Eratosthenes sieve | gcc | [2, 3, 5, 7] 10 [2, 3, 5, 7, 11, 13, 17, 19, 23, 29] |
| 02_algorithms/10_palindrome.kark | Runnable | both | reversal, equality | gcc | 1 0 1 1 olleh |
| 03_systems/01_file_io.kark | Runnable | both | std.io write/read/delete | gcc, std | true 3 alpha beta gamma true |
| 05_data/01_word_frequency.kark | Runnable | both | tokenize + tally | gcc, std | 3 2 2 1 8 |
| 05_data/02_csv_aggregate.kark | Runnable | both | CSV-ish parse + aggregate | gcc, std | 15 20 15 2 50 20 |
| 05_data/03_payload_roundtrip.kark | Runnable | both | hex/base64/utf8 | gcc, std | hex round-trips (see file) |
| 08_concurrency/01_parallel_sum.kark | Experimental | go | spawn/join grid | gcc | 285 |
| 08_concurrency/02_channel_ping.kark | Experimental | go | channel producer | gcc | 5 0 8 |
| 09_ai/01_nearest_neighbor.kark | Runnable | both | Manhattan k-NN | gcc | 10 60 100 60 |
| 09_ai/02_linear_classifier.kark | Runnable | both | perceptron updates | gcc | 5 1 -1 2 -3 |
| 10_machine_learning/01_linear_regression.kark | Runnable | both | OLS closed form | gcc | 0.9 1.3 6.7 |
| 10_machine_learning/02_gradient_descent.kark | Runnable | both | loss minimization | gcc | 3 7 |
| 12_scientific_computing/01_sqrt_newton.kark | Runnable | both | Newton sqrt | gcc | 1.41421 3 2 0.5 |
| 12_scientific_computing/02_numerical_integration.kark | Runnable | both | trapezoid rule | gcc | 0.34375 0.333374 0.333333 |
| 12_scientific_computing/03_statistics.kark | Runnable | both | mean/variance/stddev | gcc | 5 4 2 |
| 12_scientific_computing/04_matrix_multiply.kark | Runnable | both | integer matmul | gcc | rows + corner values |
| 13_finance/01_compound_interest.kark | Runnable | both | compound interest | gcc | 115762 231854 20000 10000 |
| 13_finance/02_loan_amortization.kark | Runnable | both | annuity payment | gcc | 12950.5 11231.4 12754.8 |
| 13_finance/03_npv.kark | Runnable | both | discounted cash flows | gcc | -454.545 3992.71 |
| 13_finance/04_portfolio.kark | Runnable | both | weighted returns | gcc | 5 3 7 |
| 14_security/01_digests.kark | Runnable | both | sha256/sha512 (NIST) | gcc, std | NIST vectors |
| 14_security/02_encoding_roundtrip.kark | Runnable | both | RFC 4648 hex/base64 | gcc, std | round-trips |
| 14_security/03_password_hash.kark | Runnable | both | salt+sha256 demo | gcc, std | 64-hex + booleans |
| 14_security/04_utf8_text.kark | Runnable | both | UTF-8 encode/validate | gcc, std | 1 636166c3a9 1 CAFé |
| 15_developer_tools/01_hello_toolchain.kark | Runnable | both | toolchain loop | gcc | write -> build -> run |
| 15_developer_tools/02_assertions_test.kark | Runnable | both | karkain test | gcc | 3 passed |

## Planned categories (README only, no `.kark`)

| Category | Honest status |
|----------|---------------|
| 04 Networking | sockets not implemented; std/http.go stubs not wired |
| 06 Database | no std.db surface |
| 07 Web | no web framework; http stubs not wired |
| 11 Quantum | parser/lexer do not wire circuit/qubit/qpu; infrastructure only |

## Verification

```powershell
karkain run examples/01_fundamentals/01_hello_world.kark
scripts\verify-examples.ps1
go test ./pkg/cli/ -run Phase114 -count=1
```