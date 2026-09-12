# Contributing to Karkain

Thanks for your interest in Karkain! This guide explains how the project is
organized, how to run the toolchain and its tests, and the conventions you
must follow before your contribution can land.

Karkain is **Beta 1** software. Every contribution is expected to
respect the honesty rules documented in `docs/source/status/index.rst`:
never claim a feature works unless it is verified by an automated gate.

## Project status and expectations

Read `docs/source/development/developer-preview.rst` before starting. The
toolchain is real and tested, but its public surface is still evolving;
APIs may change between releases. Contributions that silently introduce
unsupported behavior, or that describe planned features as implemented, will
**not** be accepted.

## Getting started

1. Fork the repository on GitHub.
2. Clone your fork and create a feature branch off `develop`:
   ```bash
   git clone https://github.com/<you>/Karkain.git
   cd Karkain
   git checkout -b feature/my-change develop
   ```
3. Install the prerequisites: Go 1.21+ and a C compiler (GCC or Clang).
4. Build the toolchain:
   ```bash
   go build -o karkain ./cmd/karkain   # Linux/macOS
   go build -o karkain.exe ./cmd/karkain  # Windows (PowerShell)
   ```
5. Verify the developer-preview gate passes (see "Testing" below).

## Branch workflow

* `main` always reflects the latest working, released state.
* All development happens on `develop`.
* Feature branches are cut from `develop` and merged back into `develop`.
* After every completed phase with a green test run, changes are committed on
  `develop` and merged into `main` (see `AGENTS.md`).

## Testing

Run the standard unit-test set before committing:

```bash
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1
go vet ./...
go build ./...
```

Run the developer-preview gate (fresh-checkout simulation, version/help
contract, repo-integrity checks):

```bash
go test ./pkg/cli/ -run TestPhase115 -count=1 -v
```

Run the release-candidate gate (external journey: first program, multi-file
project, workspace dependency, release/issue/documentation metadata):

```bash
go test ./pkg/cli/ -run TestPhase118 -count=1 -v
```

Run the full CLI suite (slow; includes the example-corpus parity gates):

```bash
go test ./pkg/cli/ -count=1
```

Verify the example corpus against its pinned goldens:

```bash
powershell -ExecutionPolicy Bypass -File scripts\verify-examples.ps1
```

If you change the documentation, the Sphinx build must stay warning-free and
link-clean:

```bash
python -m pip install -r docs/requirements.txt
python -m sphinx -b html docs/source docs/build/html -W --keep-going
python -m sphinx -b linkcheck docs/source docs/build/linkcheck
```

## Conventions

* **Two engines, one behavior.** The Go front end and the self-hosted
  `kcc` engine must produce byte-identical output for everything labeled
  Implemented. If you change semantics, run both engines.
* **Status is honest.** Every feature you add must be labeled through the
  status vocabulary (`:stable:`, `:implemented:`, `:experimental:`,
  `:planned:`, `:not-implemented:`) and backed by a gate test that exercises
  it through the real CLI.
* **ASCII source.** Keep source files free of non-ASCII characters.
* **No comments unless asked.** Code should be self-documenting; do not add
  gratuitous comments.
* **No secrets.** Never commit credentials, tokens, or private keys.

## What to work on / where to ask

* Report bugs through the issue templates (see
  `docs/development/reporting-bugs`); **never** open a public issue for a
  security vulnerability — use the private path in `SECURITY.md`.
* Use the GitHub **Issues** tracker for bug reports, feature requests, and
  questions: https://github.com/ajit-ai/Karkain/issues
* Planned work is tracked in `ROADMAP.md` and the roadmap docs page.
* Example programs belong under `examples/NN_category/`; every new example
  must run byte-identically on both engines and be listed in the Phase 114
  gate (`pkg/cli/phase114_examples_test.go`) if it is Runnable.

## Code of conduct

All interactions are governed by the Contributor Covenant — see
`CODE_OF_CONDUCT.md`.

## License

By contributing you agree that your contributions are licensed under the MIT
License (see `LICENSE`).