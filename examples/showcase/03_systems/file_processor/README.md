# 03 Systems — File Processing

STATUS: WORKING TODAY (validated). File paths are relative to the kcc-run
sandbox (a temp dir), so this example is fully self-contained.

## What it demonstrates

- the built-in file I/O surface: `writeFile`, `readFile`, `removeFile`
- `split`, `trim`, `len`, indexing, string concatenation
- `getArgs()` (includes the program name as element 0, hence `args=1` here)
- honest limitation: numeric parsing of file contents (`int(string)`) is not
  available on the default kcc engine today, so text (not numbers) is processed

## Commands

```
karkain check examples/showcase/03_systems/file_processor/main.kark
karkain run   examples/showcase/03_systems/file_processor/main.kark
```

## Expected output (verified)

```
lines=3
first=alpha
last=gamma
args=1
cleaned
```