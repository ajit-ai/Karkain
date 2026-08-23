# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

## Roadmap

See `ROADMAP.md` for the complete development plan (Phases 50–62).
Current phase: **53** — IR Infrastructure.
Last completed: **52** — Value-by-Value Runtime API.

## Known Issues (pre-existing)

- `examples/quantum_test.kar` fails: quantum gate statements (`H qr[0]`, `CNOT qr[0], qr[1]`) are mangled by the parser (broken before Phase 52; gate operands parsed as bare identifiers). Needs a dedicated fix.

## Guiding Principles

1. **Maturity over features** — depth, correctness, performance, verification before expansion
2. **Semantic foundations first** — value representation, ownership, lifetimes, IR before features
3. **Working > ambitious** — fix broken fundamentals before adding new capabilities
4. **Incremental verification** — every phase must compile, pass E2E, pass all tests

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... -count=1
```

All tests must pass before committing.
