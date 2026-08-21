# Karkain Development Conventions

## Branch Workflow (MANDATORY RULE)

After EVERY Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch
3. Push both branches to origin

This ensures `main` always reflects the latest working state.
NEVER skip this step. This is a hard rule, not optional.

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... -count=1
```

All tests must pass before committing.
