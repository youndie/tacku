#!/usr/bin/env python3
"""Открытый репозиторий не называет ничего внутреннего.

Этот репозиторий публичный, а живёт рядом с закрытыми: развёртывание, IdP, инфраструктура. Имя
внутреннего проекта или внутреннего домена, попавшее сюда, обратно уже не убирается — история
остаётся у всех, кто склонировал.

Проверка узкая намеренно. Она не пытается угадать «секрет»: она держит список имён, которые в этом
репозитории появляться не должны, и говорит, где нашла. Всё, что нужно назвать, называется
переменной окружения — тем и отличается конфигурация от кода.
"""
import pathlib, re, sys

root = pathlib.Path(__file__).resolve().parent.parent

# Имена закрытых соседей и их адресов. Список ведётся руками: угадывать тут нечего, а забыть дописать
# — это ровно тот случай, ради которого проверка и существует.
FORBIDDEN = [
    r"shildik",
    r"vedutsya",
    r"\bkeycloak\b",
    r"[a-z0-9-]*-internal\.[a-z.]+",
    r"realms/[a-z0-9-]+",
]

SKIP = {".git", "build", "node_modules", ".gradle", "dist"}

problems = []
checked = 0
for path in root.rglob("*"):
    if not path.is_file() or any(part in SKIP for part in path.parts):
        continue
    # The list of names lives here, so this file is the one place they are allowed to appear.
    if path == pathlib.Path(__file__).resolve():
        continue
    if path.suffix in {".png", ".ttf", ".jar", ".wasm", ".map"}:
        continue
    try:
        text = path.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        continue
    checked += 1
    for pattern in FORBIDDEN:
        for match in re.finditer(pattern, text, re.IGNORECASE):
            line = text[: match.start()].count("\n") + 1
            problems.append(f"{path.relative_to(root)}:{line} называет {match.group(0)!r}")

# Проверка, ничего не прочитавшая, проходит молча — и это тот же ноль, что «нарушений нет».
if checked == 0:
    print("не прочитано ни одного файла — проверка смотрела не туда", file=sys.stderr)
    sys.exit(2)

print(f"файлов просмотрено: {checked}")
for problem in problems[:20]:
    print("  " + problem)
sys.exit(1 if problems else 0)
