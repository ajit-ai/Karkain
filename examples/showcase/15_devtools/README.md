# 15 Developer Tools

STATUS: WORKING TODAY (validated). Three sub-demos: a multi-file app, its
tests, and the formatter.

## banking/ — multi-file application

Functions live in sibling files (`ledger.kark`, `utils.kark`); the assembler
concatenates them into one compilation unit for `check`/`build`/`run`.

```
karkain check examples/showcase/15_devtools/banking/main.kark
karkain build examples/showcase/15_devtools/banking/main.kark
karkain run   examples/showcase/15_devtools/banking/main.kark
```

Expected output (verified):

```
opening=$1000
after_credit=$1250
after_debit=$1170
after_overdraft_attempt=$1170
comfortable
```

## tests/ — `karkain test`

Test files are self-contained compilation units (the conformance convention);
the helpers under test are declared inside the test file.

```
karkain test examples/showcase/15_devtools/tests
```

Expected output (verified):

```
  PASS test_credit_increases_balance
  PASS test_debit_above_balance_refused
  PASS test_debit_reduces_balance
  PASS test_format_amount

=== Test Summary: 4 tests, 4 passed, 0 failed, 0 skipped ===
4 passed; 0 failed; 0 skipped; 4 total
```

## formatting/ — `karkain fmt`

`messy.kark` is deliberately unformatted (but valid). `fmt --check` reports
an exit code of 1 until the file is formatted; `fmt` then normalizes it.

```
karkain fmt --check examples/showcase/15_devtools/formatting/messy.kark   # exit 1: "file is not formatted."
karkain fmt          examples/showcase/15_devtools/formatting/messy.kark   # exit 0: "formatted."
karkain fmt --check examples/showcase/15_devtools/formatting/messy.kark   # exit 0: "already formatted."
karkain run          examples/showcase/15_devtools/formatting/messy.kark   # prints 3
```

Note: the formatter writes non-ASCII comment characters as ANSI; showcase
sources are ASCII-only by design.