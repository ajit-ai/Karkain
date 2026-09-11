# Minimal Makefile for the Karkain documentation.
#
# Requires Python 3 with Sphinx installed:
#   pip install -r docs/requirements.txt
#
# Targets:
#   make html       build HTML into docs/build/html
#   make linkcheck  verify all external links
#   make clean      remove the build directory
#   make gh-pages   build HTML (used by the GitHub Pages workflow)

SOURCEDIR     = source
BUILDDIR      = build
SPHINXOPTS    ?=
SPHINXBUILD   ?= sphinx-build

.PHONY: help clean html linkcheck gh-pages

help:
	@echo "Karkain documentation build"
	@echo "  make html        build HTML into $(BUILDDIR)/html"
	@echo "  make linkcheck   check external links"
	@echo "  make clean       remove $(BUILDDIR)"
	@echo "  make gh-pages    build HTML (CI/Pages entry point)"

clean:
	rm -rf $(BUILDDIR)

html:
	$(SPHINXBUILD) -M html "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

linkcheck:
	$(SPHINXBUILD) -M linkcheck "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

gh-pages: html