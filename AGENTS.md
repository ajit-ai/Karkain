# Karkain Development Conventions

## Branch Workflow

After every Phase completion and successful test run:
1. Commit all changes to `develop` branch
2. Merge `develop` into `main` branch

This ensures `main` always reflects the latest working state.

## Testing

Run the full test suite before committing:
```
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... -count=1
```

All tests must pass before committing.
