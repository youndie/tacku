#!/usr/bin/env python3
"""Every question is numbered once, and every reference points at one that exists.

Written after eight agents worked in parallel from one journal and eight of them wrote "Q-24".
Git merged that without a word: the file ended up with several entries under one heading, and
nothing in the build had an opinion about it. A number that names two things is worse than a
missing one, because every reference to it still looks resolved.
"""
import pathlib, re, sys

root = pathlib.Path(__file__).resolve().parent.parent
journal = root / "docs/research/questions.md"

headings = re.findall(r"^## (Q-\d+)\. (.+)$", journal.read_text(), re.M)
numbers = [n for n, _ in headings]

problems = []
for number in sorted(set(numbers)):
    if numbers.count(number) > 1:
        titles = [t for n, t in headings if n == number]
        problems.append(f"{number} стоит у {numbers.count(number)} записей: " + "; ".join(titles))

known = set(numbers)
referenced = 0
for path in list(root.glob("docs/**/*.md")) + list(root.glob("server/**/*.go")) + list(root.glob("client/**/*.kt")):
    if path == journal:
        continue
    for number in set(re.findall(r"\bQ-\d+\b", path.read_text())):
        referenced += 1
        if number not in known:
            problems.append(f"{path.relative_to(root)} ссылается на {number}, которого в журнале нет")

if not numbers:
    problems.append("в журнале не нашлось ни одной записи — проверка смотрела не туда")
if referenced == 0:
    problems.append("ни одной ссылки на вопрос не найдено — проверка смотрела не туда")

print(f"вопросов: {len(numbers)}, ссылок на них: {referenced}")
for problem in problems:
    print("  " + problem)
sys.exit(1 if problems else 0)
