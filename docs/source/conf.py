# Sphinx configuration for the official Karkain documentation.
# https://www.sphinx-doc.org/en/master/usage/configuration.html
#
# Build with:  python -m sphinx -b html docs/source docs/build/html

import os
import sys

# Make ``_lexer.karkain_lexer`` importable as a Sphinx extension.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# -- Project information -----------------------------------------------------

project = "Karkain"
author = "Ajit Kumar"
copyright = "2026, The Karkain Project"

# Version and release describe the LANGUAGE, not one binary.
version = "1.0.0"
release = version

# Prefer the VERSION file at the repository root when building from a checkout.
version_file = os.path.normpath(
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "VERSION")
)
try:
    with open(version_file, encoding="utf-8") as f:
        v = f.read().strip()
        if v:
            version = v
            release = v
except OSError:
    pass

# -- General configuration ---------------------------------------------------

extensions = [
    "sphinx.ext.todo",
    "_lexer.karkain_lexer",
]

# Cross-reference inventories (none external at present).
intersphinx_mapping = {}

templates_path = ["_templates"]
exclude_patterns = ["_build", "_static/**", "build", "Thumbs.db", ".DS_Store", "**/.git/**", "**/*.md"]
source_suffix = {
    ".rst": "restructuredtext",
}

# Build must fail on serious documentation errors (missing refs, bad includes,
# invalid RST). Keep the default warning policy; resolve warnings rather than
# suppressing them.
nitpicky = False

# Karkain source blocks. The Karkain regex lexer (``_lexer/karkain_lexer.py``)
# is registered under both ``karkain`` and ``kark`` so that every
# ``.. code-block:: karkain``/``kark`` example renders with real syntax
# highlighting instead of a Pygments "lexer not known" warning. Fall back to
# plain ``text`` for anything else; there is no dependency beyond the Sphinx-
# bundled Pygments.
highlight_language = "text"

# GitHub serves 404 to link-checker clients for repository root/issues pages
# even when the repository is public (git access works). Ignore the canonical
# repository URLs so `sphinx -b linkcheck` reports a clean, true result.
linkcheck_ignore = [
    r"https://github\.com/ajit-ai/Karkain/?",
    r"https://github\.com/ajit-ai/Karkain/issues/?",
]

# -- Options for HTML output -------------------------------------------------
#
# Professional Read-the-Docs presentation, same visual family as the
# reference documentation used across adjacent Karkain projects. The theme
# is a pinned, locally installed package (see ``docs/requirements.txt``):
# docs build is fully offline, no third-party CDN assets at build time.
html_theme = "sphinx_rtd_theme"

html_static_path = ["_static"]
html_css_files = ["karkain.css"]

html_title = "Karkain"
html_short_title = "Karkain"

html_theme_options = {
    "collapse_navigation": False,
    "sticky_navigation": True,
    "navigation_depth": 4,
    "titles_only": False,
    "includehidden": True,
    "logo_only": False,
    # NOTE: no "display_version" — sphinx-rtd-theme 3.x (Sphinx 9) rejects it
    # as an unsupported theme option, which fails the -W docs build on CI.
}

# The Read-the-Docs theme owns the primary sidebar navigation, which is
# driven by the documentation toctrees (``index.rst`` and the ``*_index.rst``
# files under each top-level section). We deliberately do not force an
# Alabaster-style list of sidebar blocks here; RTD renders its own
# professional nav, search, and version block from the toctree and the
# ``html_title`` above.

# -- Extensions --------------------------------------------------------------

todo_include_todos = True

# -- Custom status vocabulary roles ------------------------------------------
#
# Provide role shortcuts for the Karkain documentation status vocabulary:
#   :official:`Stable`  :experimental:`Experimental`  :planned:`Planned`
# These render as the given text wrapped in a span with a class derived from
# the role name (karkain-status-stable etc.).
from docutils import nodes  # noqa: E402
from docutils.parsers.rst import roles  # noqa: E402

_STATUS_ROLES = [
    "stable",
    "implemented",
    "production-candidate",
    "experimental",
    "developer-preview",
    "planned",
    "not-implemented",
]


def _status_role(
    name: str,
    rawtext: str,
    text: str,
    lineno: int,
    inliner,
    options=None,
    content=None,
):
    node = nodes.inline(rawtext, "", classes=["karkain-status", "karkain-status-" + name])
    node += nodes.Text(text)
    return [node], []


def setup(app):
    from docutils.parsers.rst import roles  # noqa: E402

    for role in _STATUS_ROLES:
        roles.register_local_role(role, _status_role)
    return {"version": "1.0", "parallel_read_safe": True, "parallel_write_safe": True}