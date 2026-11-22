#!/usr/bin/env python3
"""Replace em dashes (U+2014) and en dashes (U+2013) with plain ASCII
punctuation across the repository's text files.

Em dashes are one of the clearest giveaways of machine-written prose, so
this project keeps none of them. The replacement is deliberately natural
rather than mechanical:

  * a dash between two digits is a range, so "3-5"; it becomes "3-5"
  * any other dash becomes a spaced hyphen, " - ", which is exactly how
    people type it in real notes, commits and docs

Usage:  python scripts/strip_em_dashes.py [--check]

--check reports what would change without writing anything (exit code 1
if anything would change), which is useful in a pre-commit hook.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

TEXT_SUFFIXES = {
    ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
    ".css", ".json", ".md", ".markdown", ".html", ".txt",
    ".py", ".yml", ".yaml", ".toml", ".sql", ".sh", ".bat", ".ps1",
}

SKIP_DIRS = {
    "node_modules", "dist", ".task", "bin", ".git", ".freebuff",
    "vendor", ".vite", ".cache",
}

# en dash / em dash, surrounded by optional whitespace
NUMERIC_RANGE = re.compile(r"(?<=\d)\s*[\u2013\u2014]\s*(?=\d)")
GENERAL_DASH = re.compile(r"\s*[\u2013\u2014]\s*")


def rewrite(text: str) -> str:
    text = NUMERIC_RANGE.sub("-", text)
    text = GENERAL_DASH.sub(" - ", text)
    return text


def iter_files():
    for path in ROOT.rglob("*"):
        if not path.is_file():
            continue
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        if path.suffix.lower() not in TEXT_SUFFIXES:
            continue
        yield path


def main() -> int:
    check = "--check" in sys.argv
    changed = []
    dash_count = 0
    for path in iter_files():
        try:
            raw = path.read_bytes()
            original = raw.decode("utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        if "\u2014" not in original and "\u2013" not in original:
            continue
        dash_count += original.count("\u2014") + original.count("\u2013")
        updated = rewrite(original)
        if updated == original:
            continue
        changed.append(path)
        if not check:
            # Bytes in, bytes out: never touches CRLF line endings on
            # Windows checkouts, so this stays a focused dash-only diff.
            path.write_bytes(updated.encode("utf-8"))

    rel = [str(p.relative_to(ROOT)) for p in changed]
    for name in rel:
        print(name)
    verb = "would change" if check else "rewrote"
    print(f"\n{verb} {len(rel)} file(s), {dash_count} dash(es) total")
    if check and changed:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
