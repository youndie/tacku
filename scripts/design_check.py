#!/usr/bin/env python3
"""Сверка того, что собралось, с макетом — по значениям и по деревьям, а не по впечатлению.

Порядок здесь не случайный. Токены сравниваются числами: цвет на тон мимо выглядит на картинке
нормально, а в дереве не виден вовсе — дерево несёт имена, не значения. Деревья сравниваются
структурой: макет выписал её словами, и это единственный способ увидеть пропавший модификатор или
подменённое имя токена. Пиксели остаются на третий заход и на то, чего дерево сказать не может:
шрифт, контраст, обрезание, ховер.

Начинать сверку с картинок кажется быстрее и не быстрее: глаз нормализует. Полосы провенанса не было
ни на одном экране, мы смотрели на них много раз и не заметили — а диф деревьев показал бы её
отсутствие сразу.

Макет не хранится в репозитории намеренно: копия расходится молча, а расхождение версий видно
сразу. Файл экспортируется из проекта Claude Design и передаётся сюда путём.
"""
import argparse, json, pathlib, re, sys


def strip_tags(html: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"<[^>]+>", "|", html))


def mockup_typography(text: str) -> dict[str, tuple[int, int, str]]:
    """name -> (size, weight, hex) из таблицы «ТОКЕН ТЕКСТА»."""
    found = {}
    for m in re.finditer(r"\|\|([a-z_]+)\|\|(\d+)\s*/\s*(\d+)\|\|(#[0-9A-Fa-f]{6})\|\|", text):
        found[m.group(1)] = (int(m.group(2)), int(m.group(3)), m.group(4).upper())
    return found


def mockup_colours(text: str) -> dict[str, str]:
    """name -> hex из таблицы «ЦВЕТА», и только из неё.

    Обе таблицы написаны одинаково — `||имя||#HEX||`, — поэтому разбор без границы секции
    втягивал строки типографики и объявлял недостающими цвета, которых там и не должно быть.
    Проверка, ругающаяся на верное, учит себя игнорировать.
    """
    start = text.find("ЦВЕТА||HEX")
    if start < 0:
        return {}

    # Заканчивается на следующем заголовке таблицы, а не на следующем разделе: сразу за цветами в
    # §1 идёт «ТЕКСТ | DARK | LIGHT», написанная теми же `||имя||#HEX||`. Без границы разбор
    # втягивал её и объявлял недостающими цвета, которых в наборе цветов и нет.
    end = min(
        (position for position in (text.find(marker, start + 1) for marker in ("||ТЕКСТ||", "§2")) if position > 0),
        default=len(text),
    )
    section = text[start:end]

    found = {}
    for m in re.finditer(r"\|\|([a-z_]+)\|\|(#[0-9A-Fa-f]{6})\|\|", section):
        found.setdefault(m.group(1), m.group(2).upper())
    return found


def ours_typography(kotlin: str) -> dict[str, tuple[int, int, str]]:
    found = {}
    weights = {"Normal": 400, "Medium": 500, "SemiBold": 600, "Bold": 700}
    for m in re.finditer(r'"([a-z_]+)" to style\((\d+), FontWeight\.(\w+), (?:if \(dark\) )?0x([0-9A-Fa-f]{8})', kotlin):
        found[m.group(1)] = (int(m.group(2)), weights.get(m.group(3), 0), "#" + m.group(4)[2:].upper())
    return found


def ours_colours(kotlin: str) -> dict[str, str]:
    """Тёмная тема: первая из двух карт в файле."""
    dark = kotlin.split("} else {")[0]
    return {
        m.group(1): "#" + m.group(2)[2:].upper()
        for m in re.finditer(r'"([a-z_]+)" to Color\(0x([0-9A-Fa-f]{8})\)', dark)
    }


def mockup_usage(text: str) -> tuple[set[str], set[str]]:
    """Что макет рисует: типы узлов и имена токенов, встреченные в выписанных деревьях."""
    types, tokens = set(), set()
    for m in re.finditer(r"[├└│]\s*(column|row|text|button|paginated_list|table|image|[a-z_]+_input|read_only_field)\b", text):
        types.add(m.group(1))
    for m in re.finditer(r"→\s*background\s+([a-z_]+)", text):
        tokens.add(m.group(1))
    for m in re.finditer(r'"\s+(display|title|subtitle|body|body_muted|value|label|meta|meta_agent|error|notice|button_primary|button_quiet)\b', text):
        tokens.add(m.group(1))
    return types, tokens


def ours_usage(corpus: pathlib.Path) -> tuple[set[str], set[str]]:
    types, tokens = set(), set()

    def walk(node):
        if isinstance(node, dict):
            if "type" in node and isinstance(node["type"], str):
                types.add(node["type"])
            for modifier in node.get("modifiers", []) or []:
                if isinstance(modifier, dict) and modifier.get("type") == "background":
                    tokens.add(modifier.get("color"))
            if node.get("style"):
                tokens.add(node["style"])
            for key in ("children", "initialItems"):
                for child in node.get(key) or []:
                    walk(child)
            for key in ("emptyState", "screen"):
                if isinstance(node.get(key), dict):
                    walk(node[key])
        elif isinstance(node, list):
            for item in node:
                walk(item)

    for path in sorted(corpus.glob("*.json")):
        walk(json.loads(path.read_text()))
    return types, {t for t in tokens if t}


def table(title: str, rows: list[tuple[str, str, str]]) -> int:
    print(f"\n{title}")
    print("-" * 78)
    bad = 0
    for name, theirs, ours in rows:
        mark = " " if theirs == ours else "≠"
        if mark == "≠":
            bad += 1
        print(f" {mark} {name:<16} макет: {theirs:<24} у нас: {ours}")
    return bad


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--design", required=True, help="экспорт макета (.dc.html)")
    parser.add_argument("--kotlin", default="client/app/src/main/kotlin/tacku/app/TackuDesignSystem.kt")
    parser.add_argument("--screens", default="client/app/src/test/screens")
    args = parser.parse_args()

    design = pathlib.Path(args.design)
    if not design.is_file():
        print(f"нет файла макета: {design}\nэкспортируйте его из проекта Claude Design и передайте путём", file=sys.stderr)
        return 2

    text = strip_tags(design.read_text(encoding="utf-8"))
    kotlin = pathlib.Path(args.kotlin).read_text()

    theirs_type, ours_type = mockup_typography(text), ours_typography(kotlin)
    theirs_colour, ours_colour = mockup_colours(text), ours_colours(kotlin)

    # Проверка не должна проходить молча оттого, что ничего не разобралось.
    if not theirs_type or not theirs_colour:
        print("в макете не нашлось таблиц токенов — разбор смотрел не туда", file=sys.stderr)
        return 2
    if not ours_type or not ours_colour:
        print("в дизайн-системе не нашлось токенов — разбор смотрел не туда", file=sys.stderr)
        return 2

    problems = 0

    problems += table(
        f"ТИПОГРАФИКА ({len(theirs_type)} в макете, {len(ours_type)} у нас)",
        [
            (name, f"{t[0]}/{t[1]} {t[2]}", (lambda o: f"{o[0]}/{o[1]} {o[2]}" if o else "нет")(ours_type.get(name)))
            for name, t in sorted(theirs_type.items())
        ],
    )

    problems += table(
        f"ЦВЕТА ({len(theirs_colour)} в макете, {len(ours_colour)} у нас)",
        [(name, hexes, ours_colour.get(name, "нет")) for name, hexes in sorted(theirs_colour.items())],
    )

    theirs_types, theirs_tokens = mockup_usage(text)
    our_types, our_tokens = ours_usage(pathlib.Path(args.screens))

    print(f"\nСЛОВАРЬ И ТОКЕНЫ В ДЕРЕВЬЯХ")
    print("-" * 78)
    for label, theirs, ours in (("тип", theirs_types, our_types), ("токен", theirs_tokens, our_tokens)):
        only_theirs = sorted(theirs - ours)
        only_ours = sorted(ours - theirs)
        if only_theirs:
            print(f" ≠ {label}: макет рисует, мы нет — {', '.join(only_theirs)}")
            problems += len(only_theirs)
        if only_ours:
            print(f" · {label}: у нас есть, в макете нет — {', '.join(only_ours)}")

    print(f"\nрасхождений: {problems}")
    print("каждое сначала классифицировать: наше отступление / макет старше словаря / так и задумано")
    return 0


if __name__ == "__main__":
    sys.exit(main())
