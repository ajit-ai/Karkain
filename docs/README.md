# Karkain Documentation

This directory contains the official Karkain documentation as a
**Sphinx / reStructuredText** site, published to GitHub Pages.

## Reading the documentation

The built site is hosted at:

```
https://ajit-ai.github.io/Karkain/
```

The rendered site mirrors the content in `docs/source/`.

## Building locally

Requirements:

- Python 3.9+ (or 3.11+; Sphinx 7+ requires 3.9+)
- Sphinx (installed below)

```bash
python -m pip install -r docs/requirements.txt
python -m sphinx -b html docs/source docs/build/html
```

Or, if `sphinx-build` is on PATH:

```bash
cd docs
make html
```

The HTML output lands in `docs/build/html/`. Open `index.html`.

## Checking for broken references/links

```bash
python -m sphinx -b linkcheck docs/source docs/build/linkcheck
```

The build is configured to **fail on serious documentation errors** (invalid
RST, missing references, missing included files). Warnings are reviewed in CI.

## Structure

- `source/` — reStructuredText source of the documentation
- `source/conf.py` — Sphinx configuration
- `source/_static/` — static assets (CSS)
- `requirements.txt` — Python dependencies for the docs build
- `Makefile` — convenience targets (`html`, `clean`, `linkcheck`)

## GitHub Pages

`.github/workflows/docs.yml` builds the Sphinx site and publishes it to
GitHub Pages on every push to `main` and on a manual dispatch. Generated
build output is **not** committed to the repository.

## Writing conventions

- All Karkain source examples use the `.kark` extension — never `.kar`.
- Use the status vocabulary (Stable / Implemented / Experimental /
  Developer Preview / Planned / Not Yet Implemented) from the
  documentation status model.
- Do not invent APIs or claim planned capabilities as implemented.
- See `source/status/index.rst` for the authoritative status model.