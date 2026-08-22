#!/usr/bin/env python3
"""A claim of having reported something upstream carries the address of the report.

Written after a line saying "вынесено наверх" turned out to have been written before anything was
filed, and the repository it names had no such issue at all. Prose beside verified work is the half
nobody verifies: the test was real, the mutation table was real, and the sentence between them was
not true.

The rule is narrow on purpose. It does not try to tell a plan from a claim — Russian tense is not
something a regex should be asked about. It asks only that a paragraph asserting a report was made
contains a link to it, and it says so by name when one does not.
"""
import pathlib, re, sys

root = pathlib.Path(__file__).resolve().parent.parent
# Past tense only: "вопрос наверх" and "вынести наверх" are plans and carry no address yet.
CLAIM = re.compile(r"(сообщено наверх|вынесено наверх|^Сообщено\b)", re.IGNORECASE | re.MULTILINE)
LINK = re.compile(r"https?://\S+")
# A line explaining that a claim used to be unbacked is not itself a claim.
EXEMPT = re.compile(r"поправлено утверждение|была написана до того", re.IGNORECASE)

problems = []
claims = 0
for path in sorted(root.glob("docs/**/*.md")):
    for paragraph in path.read_text().split("\n\n"):
        if not CLAIM.search(paragraph) or EXEMPT.search(paragraph):
            continue
        claims += 1
        if not LINK.search(paragraph):
            line = paragraph.strip().splitlines()[0][:90]
            problems.append(f"{path.relative_to(root)}: «{line}» — без адреса отчёта")

if claims == 0:
    problems.append("ни одного утверждения о вынесенном наверх не найдено — проверка смотрела не туда")

print(f"утверждений об отчётах наверх: {claims}")
for problem in problems:
    print("  " + problem)
sys.exit(1 if problems else 0)
