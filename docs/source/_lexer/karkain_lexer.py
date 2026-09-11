"""Custom Pygments lexer for the Karkain programming language.

Registered as an alias so that ``.. code-block:: karkain`` and
``.. code-block:: kark`` blocks render with real highlights instead of
falling back or warning. Kept dependency-light: pure Pygments ``RegexLexer``,
no external packages beyond Sphinx's bundled Pygments.
"""

from pygments.lexer import RegexLexer, include, bygroups
from pygments.token import (
    Comment,
    Keyword,
    Name,
    Number,
    Operator,
    Punctuation,
    String,
    Text,
)

__all__ = ["KarkainLexer"]


class KarkainLexer(RegexLexer):
    """Lexer for Karkain (.kark) source files."""

    name = "Karkain"
    aliases = ["karkain", "kark"]
    filenames = ["*.kark"]

    tokens = {
        "root": [
            (r"\s+", Text),
            (r"//[^\n]*", Comment.Single),
            (r"/\*", Comment.Multiline, "comment"),
            (r"\b(func|let|var|const|for|while|if|else|match|return|"
             r"break|continue|in|struct|enum|import|public|"
             r"spawn|receive|channel|actor|send|true|false)\b",
             Keyword),
            (r"\b(int|float64|bool|string|array|map|float)\b", Keyword.Type),
            (r"\b\d+(\.\d+)?\b", Number),
            (r'"(\\"|[^"])*"', String),
            (r"@[A-Za-z_][A-Za-z0-9_]*", Name.Decorator),
            (r"\b[A-Za-z_][A-Za-z0-9_]*\b", Name),
            (r"==|!=|<=|>=|&&|\|\||[-+*/%<>=!:]", Operator),
            (r"[()\[\]{},;.]", Punctuation),
            (r".", Text),
        ],
        "comment": [
            (r"[^*/]+", Comment.Multiline),
            (r"/\*", Comment.Multiline, "#push"),
            (r"\*/", Comment.Multiline, "#pop"),
            (r"[*/]", Comment.Multiline),
        ],
    }


def setup(app):
    app.add_lexer("karkain", KarkainLexer)
    app.add_lexer("kark", KarkainLexer)


if __name__ == "__main__":
    import sys
    from pygments import highlight
    from pygments.formatters import TerminalFormatter

    source = sys.stdin.read()
    print(highlight(source, KarkainLexer(), TerminalFormatter()))