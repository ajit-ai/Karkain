# 05 — Data

Real data-manipulation programs using the implemented standard library
(`std.string`, `std.collections`, `std.encoding`). Every file is pinned by
`pkg/cli/phase114_examples_test.go` on both engines.

| Example                   | Skill demonstrated                         | Stdlib used    |
|---------------------------|--------------------------------------------|----------------|
| `01_word_frequency.kark`  | tokenize, tally, report                    | string/maps    |
| `02_csv_aggregate.kark`   | split/parse rows, aggregate by key         | string/sums    |
| `03_payload_roundtrip.kark` | hex/base64 encode-decode, UTF-8 checks   | encoding       |

## Honest scope

- Parsing here is exercise-level: no quoting/escaping CSV rules, no JSON
  library. JSON/structured formats with full grammar support are not yet in
  the stdlib.
- Byte math (raw `0xff`-style literals) is not part of the language surface.
- A production serialization standard library is a future phase.

## Run

```
karkain run examples/05-data/02_csv_aggregate.kark
```