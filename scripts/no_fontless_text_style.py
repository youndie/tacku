#!/usr/bin/env python3
"""Refuse a TextStyle built without a font family.

A `TextStyle` that names no family is drawn in whatever the machine has installed. Nothing about
that looks wrong: the fallback is a reasonable sans, the picture of it is stable, and it passes
every check made on one machine. It was a button label here — the one control the design cares most
about, set in a font the design never chose, while the text around it was in the product's typeface.

Two machines are what told them apart, and only because their fallbacks differ. So the rule is
static instead: text styles come from the design system, and the two places that construct one
name the family they construct it with.
"""

import pathlib
import re
import sys

# `TextStyle(` up to its closing paren, over newlines: the real one wrapped.
CONSTRUCTED = re.compile(r"\bTextStyle\s*\(([^()]*(?:\([^()]*\)[^()]*)*)\)", re.DOTALL)
COMMENT = re.compile(r"//[^\n]*|/\*.*?\*/", re.DOTALL)


def blanked(text: str) -> str:
    """Comments, blanked but for their newlines, so offsets still name the right line."""
    return COMMENT.sub(lambda m: re.sub(r"[^\n]", " ", m.group(0)), text)


def main() -> int:
    root = pathlib.Path(__file__).resolve().parent.parent / "client"
    sources = [p for p in sorted(root.rglob("*.kt")) if "/build/" not in str(p)]

    # A guard that read nothing passes for the wrong reason.
    if not sources:
        print("no_fontless_text_style: found no Kotlin sources to read", file=sys.stderr)
        return 1

    found = []
    for path in sources:
        text = blanked(path.read_text())
        for hit in CONSTRUCTED.finditer(text):
            arguments = hit.group(1)
            if "fontFamily" in arguments:
                continue
            # TextStyle.Default and copies of an existing style are not constructions.
            number = text.count("\n", 0, hit.start()) + 1
            found.append(f"{path.relative_to(root.parent)}:{number}: {' '.join(hit.group(0).split())[:90]}")

    if found:
        print("A TextStyle is constructed without naming a font family.", file=sys.stderr)
        print("It will be drawn in whatever the machine has installed. Ask the design", file=sys.stderr)
        print("system for the style instead: resolveTypography(TypographyToken(...)).\n", file=sys.stderr)
        for line in found:
            print("  " + line, file=sys.stderr)
        return 1

    print(f"no_fontless_text_style: {len(sources)} sources, every constructed style names a family")
    return 0


if __name__ == "__main__":
    sys.exit(main())
