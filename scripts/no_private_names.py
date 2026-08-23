#!/usr/bin/env python3
"""Открытый репозиторий не называет ничего внутреннего.

Этот репозиторий публичный, а живёт рядом с закрытыми: развёртывание, IdP, инфраструктура. Имя
внутреннего проекта, внутреннего домена или адреса, по которому это развёрнуто, попав сюда,
обратно уже не убирается — история остаётся у всех, кто склонировал.

**Список имён здесь не написан, и это главное в этом файле.** Первая версия перечисляла их
открытым текстом — то есть публиковала ровно то, что запрещала, да ещё и в удобном для чтения
виде: сторож, который сам и есть утечка. Теперь хранятся отпечатки: сравнение работает, а прочесть
список нельзя.

Отпечаток берётся от слова целиком, а текст режется на слова по всему, что не буква и не цифра, —
поэтому имя ловится и внутри составного адреса. Адреса хранятся отдельными отпечатками, целиком:
слово, совпадающее с именем этого репозитория, стоит на каждой странице, и запретить можно только
адрес. Формы, а не имена (внутренний домен, путь realm-а) остаются регулярными выражениями: в них
скрывать нечего.

Добавить имя или адрес: `python3 scripts/no_private_names.py --add <что>` — напечатает строку с
отпечатком, её и вписать в FINGERPRINTS (слово) или HOSTS (адрес целиком).
"""
import hashlib
import pathlib
import re
import sys

root = pathlib.Path(__file__).resolve().parent.parent


def fingerprint(word: str) -> str:
    return hashlib.sha256(word.strip().lower().encode()).hexdigest()[:16]


# Отпечатки закрытых имён: два соседних продукта и предшественник IdP.
# Список ведётся руками — угадывать тут нечего, а забыть дописать — ровно тот случай, ради которого
# проверка и существует.
FINGERPRINTS = {
    "3a2b12171abf6b6b",
    "e264d92d9270197c",
    "d63f4a0cdaab5272",
}

# Отпечатки целых имён хостов. Отдельно от слов, потому что слово `tacku` — имя этого самого
# репозитория и встречается на каждой странице: запрещать можно только адрес целиком.
HOSTS = {
    "ca6d42a00213c386",
    "12cc53a6855e641d",
}

# Формы, а не имена. Их не прячем: по ним ничего не узнать, кроме того, что внутренние адреса и
# realm-пути в этом репозитории не пишут.
SHAPES = [
    r"[a-z0-9-]*-internal\.[a-z.]+",
    r"realms/[a-z0-9-]+",
]

WORD = re.compile(r"[A-Za-z][A-Za-z0-9]{2,}")
HOST = re.compile(r"[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+")
SKIP = {".git", "build", "node_modules", ".gradle", "dist"}


def main() -> int:
    if len(sys.argv) == 3 and sys.argv[1] == "--add":
        print(f'    "{fingerprint(sys.argv[2])}",')
        return 0

    problems = []
    checked = 0
    for path in root.rglob("*"):
        if not path.is_file() or any(part in SKIP for part in path.parts):
            continue
        if path.suffix in {".png", ".ttf", ".jar", ".wasm", ".map"}:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        checked += 1

        # Этот файл содержит отпечатки, а не имена, поэтому проверяется наравне со всеми: если имя
        # окажется здесь открытым текстом, оно будет найдено.
        for match in WORD.finditer(text):
            if fingerprint(match.group(0)) in FINGERPRINTS:
                line = text[: match.start()].count("\n") + 1
                problems.append(f"{path.relative_to(root)}:{line} называет закрытое имя")

        for match in HOST.finditer(text):
            if fingerprint(match.group(0)) in HOSTS:
                line = text[: match.start()].count("\n") + 1
                problems.append(f"{path.relative_to(root)}:{line} называет закрытый адрес")

        for shape in SHAPES:
            for match in re.finditer(shape, text, re.IGNORECASE):
                line = text[: match.start()].count("\n") + 1
                problems.append(f"{path.relative_to(root)}:{line} называет внутренний адрес")

    # Проверка, ничего не прочитавшая, проходит молча — и это тот же ноль, что «нарушений нет».
    if checked == 0:
        print("не прочитано ни одного файла — проверка смотрела не туда", file=sys.stderr)
        return 2

    print(f"файлов просмотрено: {checked}")
    for problem in problems[:20]:
        print("  " + problem)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
