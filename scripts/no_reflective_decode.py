#!/usr/bin/env python3
"""Refuse a decode that only works where there is reflection.

The vocabulary's two roots — KompotComponent and KompotAction — are plain interfaces. Asking
kotlinx-serialization for their serializer by type argument compiles everywhere and works on the JVM
only, where the serializer is found by reflection at runtime. In a page it throws on the first
response from the server, which is long after every build has gone green.

So the roots must be decoded through a named PolymorphicSerializer, and this refuses the shape that
compiles into the other behaviour.
"""

import pathlib
import re
import sys

ROOTS = ("KompotComponent", "KompotAction")

# `decodeFromString<KompotComponent>(...)`, and the same written as a declared return type:
# `fun f(): KompotComponent = json.decodeFromString(body)` — the type argument is inferred there,
# which is the form that bit. Both are matched over the whole file rather than line by line: the
# real one wrapped after the `=`, and a line-by-line version of this guard saw nothing.
EXPLICIT = re.compile(r"decodeFromString\s*<\s*(" + "|".join(ROOTS) + r")\s*>")
INFERRED = re.compile(
    r":\s*(" + "|".join(ROOTS) + r")\s*=\s*(?:\s|\n)*[A-Za-z_.]*\bdecodeFromString\s*\("
    r"(?!\s*PolymorphicSerializer)"
)

COMMENT = re.compile(r"//[^\n]*|/\*.*?\*/", re.DOTALL)

# A function whose declared return type is a root, and whose body decodes without naming a
# serializer. The two forms above only see an expression body; this one shipped in a block body and
# reached a browser, where it threw on the first click that produced an action.
RETURNING = re.compile(r"\)\s*:\s*(" + "|".join(ROOTS) + r")\s*\{")
BARE_DECODE = re.compile(r"\bdecodeFromString\s*\((?!\s*PolymorphicSerializer)")

def blanked(text: str) -> str:
    """Comments, blanked but for their newlines, so offsets still name the right line.

    Prose that names the forbidden shape in order to explain it is not the shape.
    """
    return COMMENT.sub(lambda m: re.sub(r"[^\n]", " ", m.group(0)), text)

def balanced(text: str, opening: int) -> str:
    """The block that starts at `opening`, counting braces.

    Crude and enough: strings in this codebase do not carry unbalanced braces, and a miscount would
    make the guard read too much rather than too little — which is the safe direction for a guard.
    """
    depth = 0
    for index in range(opening, len(text)):
        if text[index] == "{":
            depth += 1
        elif text[index] == "}":
            depth -= 1
            if depth == 0:
                return text[opening : index + 1]
    return text[opening:]


def main() -> int:
    root = pathlib.Path(__file__).resolve().parent.parent / "client"
    sources = sorted(root.rglob("*.kt"))
    sources = [p for p in sources if "/build/" not in str(p)]

    # A guard that read nothing passes for the wrong reason.
    if not sources:
        print("no_reflective_decode: found no Kotlin sources to read", file=sys.stderr)
        return 1

    found = []
    for path in sources:
        text = blanked(path.read_text())
        for pattern in (EXPLICIT, INFERRED):
            for hit in pattern.finditer(text):
                number = text.count("\n", 0, hit.start()) + 1
                where = f"{path.relative_to(root.parent)}:{number}"
                found.append(f"{where}: {' '.join(hit.group(0).split())}")

        for hit in RETURNING.finditer(text):
            body = balanced(text, hit.end() - 1)
            for decode in BARE_DECODE.finditer(body):
                number = text.count("\n", 0, hit.end() - 1 + decode.start()) + 1
                where = f"{path.relative_to(root.parent)}:{number}"
                found.append(f"{where}: a function returning {hit.group(1)} decodes without naming one")

    if found:
        print("A root of the vocabulary is decoded without naming its serializer.", file=sys.stderr)
        print("That works on the JVM and throws in a browser. Pass", file=sys.stderr)
        print("PolymorphicSerializer(Type::class) as the first argument.\n", file=sys.stderr)
        for line in found:
            print("  " + line, file=sys.stderr)
        return 1

    print(f"no_reflective_decode: {len(sources)} sources, no reflective root decode")
    return 0

if __name__ == "__main__":
    sys.exit(main())
