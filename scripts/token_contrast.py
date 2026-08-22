#!/usr/bin/env python3
"""
Contrast and colour-blindness arithmetic for the design tokens.

    python3 scripts/token_contrast.py                       # the report
    python3 scripts/token_contrast.py --docs docs           # tree to read the tokens from

WHY THIS EXISTS. The provenance stripe is a colour and, on a board, the only thing a scanning eye
reads. "Probably survives" was the state of that claim; this turns the half of it that a machine can
decide into numbers, and leaves the other half — whether people who do not see the hue read the
board correctly — where it belongs, in a test with people (B-28).

WHAT IT MEASURES, AND WHAT EACH NUMBER MEANS.

  * WCAG contrast ratio. Built from relative luminance alone, so it is exactly what a greyscale
    screen shows and very nearly what a dichromat sees: the luminance of a colour barely moves under
    protanopia or deuteranopia. A ratio near 1 means two colours that differ only in hue — the one
    kind of difference that fails first, in the periphery and in every simulation.
  * dE2000 between the pair as drawn and the pair as simulated (Machado et al. 2009, severity 1.0,
    protanopia and deuteranopia). This is the part the ratio cannot see: how much of the difference
    was hue, and therefore how much of it a dichromat loses.

The implementation is checked against the CIEDE2000 test data of Sharma, Wu and Dalal (2005) before
it prints anything: an arithmetic error here would produce confident numbers of the wrong size, and
that is worse than no numbers at all.

WHERE THE VALUES COME FROM. `docs/design/design-spec-tokens.md`, the prose half of the token set —
the only place in this repository where the tokens have values at all. The machine-readable half
(`design/tokens.json`) carries names, because names are what the server has to agree on.
"""
import argparse
import math
import os
import re
import sys

# --- colour ----------------------------------------------------------------------------------


def srgb_to_linear(component):
    if component <= 0.04045:
        return component / 12.92
    return ((component + 0.055) / 1.055) ** 2.4


def linear_to_srgb(component):
    component = max(0.0, min(1.0, component))
    if component <= 0.0031308:
        return component * 12.92
    return 1.055 * component ** (1 / 2.4) - 0.055


def parse_hex(value):
    value = value.strip().lstrip("#")
    return tuple(int(value[i:i + 2], 16) / 255 for i in (0, 2, 4))


def linear(rgb):
    return tuple(srgb_to_linear(c) for c in rgb)


def luminance(rgb):
    r, g, b = linear(rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(first, second):
    a, b = luminance(first), luminance(second)
    if a < b:
        a, b = b, a
    return (a + 0.05) / (b + 0.05)


def to_lab(rgb):
    r, g, b = linear(rgb)
    # sRGB D65 primaries, then the D65 white point.
    x = 0.4124564 * r + 0.3575761 * g + 0.1804375 * b
    y = 0.2126729 * r + 0.7151522 * g + 0.0721750 * b
    z = 0.0193339 * r + 0.1191920 * g + 0.9503041 * b
    white = (0.95047, 1.00000, 1.08883)

    def f(t):
        if t > (6 / 29) ** 3:
            return t ** (1 / 3)
        return t / (3 * (6 / 29) ** 2) + 4 / 29

    fx, fy, fz = f(x / white[0]), f(y / white[1]), f(z / white[2])
    return (116 * fy - 16, 500 * (fx - fy), 200 * (fy - fz))


def delta_e_2000(lab1, lab2):
    """CIEDE2000. Written out from the formula rather than approximated; checked against Sharma."""
    l1, a1, b1 = lab1
    l2, a2, b2 = lab2

    c1 = math.hypot(a1, b1)
    c2 = math.hypot(a2, b2)
    c_bar = (c1 + c2) / 2
    g = 0.5 * (1 - math.sqrt(c_bar ** 7 / (c_bar ** 7 + 25 ** 7)))

    a1p, a2p = (1 + g) * a1, (1 + g) * a2
    c1p, c2p = math.hypot(a1p, b1), math.hypot(a2p, b2)
    h1p = math.degrees(math.atan2(b1, a1p)) % 360 if (a1p or b1) else 0.0
    h2p = math.degrees(math.atan2(b2, a2p)) % 360 if (a2p or b2) else 0.0

    dlp = l2 - l1
    dcp = c2p - c1p
    if c1p * c2p == 0:
        dhp = 0.0
    elif abs(h2p - h1p) <= 180:
        dhp = h2p - h1p
    elif h2p - h1p > 180:
        dhp = h2p - h1p - 360
    else:
        dhp = h2p - h1p + 360
    dhp = 2 * math.sqrt(c1p * c2p) * math.sin(math.radians(dhp) / 2)

    lp_bar = (l1 + l2) / 2
    cp_bar = (c1p + c2p) / 2
    if c1p * c2p == 0:
        hp_bar = h1p + h2p
    elif abs(h1p - h2p) <= 180:
        hp_bar = (h1p + h2p) / 2
    elif h1p + h2p < 360:
        hp_bar = (h1p + h2p + 360) / 2
    else:
        hp_bar = (h1p + h2p - 360) / 2

    t = (1
         - 0.17 * math.cos(math.radians(hp_bar - 30))
         + 0.24 * math.cos(math.radians(2 * hp_bar))
         + 0.32 * math.cos(math.radians(3 * hp_bar + 6))
         - 0.20 * math.cos(math.radians(4 * hp_bar - 63)))
    d_theta = 30 * math.exp(-(((hp_bar - 275) / 25) ** 2))
    rc = 2 * math.sqrt(cp_bar ** 7 / (cp_bar ** 7 + 25 ** 7))
    sl = 1 + (0.015 * (lp_bar - 50) ** 2) / math.sqrt(20 + (lp_bar - 50) ** 2)
    sc = 1 + 0.045 * cp_bar
    sh = 1 + 0.015 * cp_bar * t
    rt = -math.sin(math.radians(2 * d_theta)) * rc

    return math.sqrt((dlp / sl) ** 2 + (dcp / sc) ** 2 + (dhp / sh) ** 2
                     + rt * (dcp / sc) * (dhp / sh))


# Machado, Oliveira and Fernandes (2009), severity 1.0, applied in linear RGB.
DICHROMAT = {
    "protanopia": ((0.152286, 1.052583, -0.204868),
                   (0.114503, 0.786281, 0.099216),
                   (-0.003882, -0.048116, 1.051998)),
    "deuteranopia": ((0.367322, 0.860646, -0.227968),
                     (0.280085, 0.672501, 0.047413),
                     (-0.011820, 0.042940, 0.968881)),
}


def simulate(rgb, kind):
    r, g, b = linear(rgb)
    rows = DICHROMAT[kind]
    out = [row[0] * r + row[1] * g + row[2] * b for row in rows]
    return tuple(linear_to_srgb(c) for c in out)


def self_check():
    """Sharma, Wu and Dalal (2005), three rows of the CIEDE2000 test data."""
    cases = [
        ((50.0000, 2.6772, -79.7751), (50.0000, 0.0000, -82.7485), 2.0425),
        ((50.0000, 2.4900, -0.0010), (50.0000, -2.4900, 0.0009), 7.1792),
        ((50.0000, 2.5000, 0.0000), (50.0000, 0.0000, -2.5000), 4.3065),
    ]
    for first, second, expected in cases:
        got = delta_e_2000(first, second)
        if abs(got - expected) > 0.0002:
            sys.exit("dE2000 is wrong: {0} vs {1} gave {2:.4f}, not {3}".format(
                first, second, got, expected))


# --- the token set ---------------------------------------------------------------------------

HEX = re.compile(r"#[0-9A-Fa-f]{6}")


def read_tokens(docs):
    """Both themes of every token, plus the values a proposal writes as `old` -> `new`."""
    path = os.path.join(docs, "design", "design-spec-tokens.md")
    with open(path, encoding="utf-8") as handle:
        lines = handle.read().splitlines()

    tokens = {}
    proposed = {}
    for line in lines:
        if not line.startswith("|"):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != 3:
            continue
        dark, light = HEX.findall(cells[1]), HEX.findall(cells[2])
        if not dark or not light:
            continue
        # A cell reading `#AAA111` -> `#BBB222` is a proposal: the second value is the candidate.
        names = [name.strip(" `●") for name in re.findall(r"`([^`]+)`", cells[0])]
        names = [name for name in names if not name.startswith("#")]
        for name in names:
            where = proposed if (len(dark) > 1 or len(light) > 1) else tokens
            where[name] = (dark[-1], light[-1])
    if not tokens:
        sys.exit("no tokens parsed out of {0} — the tables changed shape".format(path))
    return tokens, proposed


def value_of(tokens, proposed, name, index):
    if name.startswith("+"):
        return proposed[name[1:]][index]
    return tokens[name][index]


def report(tokens, proposed, pairs):
    for title, first, second, note in pairs:
        print("\n{0}\n{1}".format(title, "-" * len(title)))
        for theme, index in (("dark", 0), ("light", 1)):
            first_hex = value_of(tokens, proposed, first, index)
            second_hex = value_of(tokens, proposed, second, index)
            a, b = parse_hex(first_hex), parse_hex(second_hex)
            line = "  {0:<26}{1:>8.2f}:1   dE {2:>5.1f}".format(
                "{0} {1} / {2}".format(theme, first_hex, second_hex),
                contrast(a, b), delta_e_2000(to_lab(a), to_lab(b)))
            for kind in ("protanopia", "deuteranopia"):
                seen = delta_e_2000(to_lab(simulate(a, kind)), to_lab(simulate(b, kind)))
                line += "   {0} {1:>5.1f}".format(kind[:4], seen)
            print(line)
        if note:
            print("  ({0})".format(note))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--docs", default="docs")
    args = parser.parse_args()

    self_check()
    tokens, proposed = read_tokens(args.docs)

    print("tokens read: {0} in both themes, {1} of them with a proposed value"
          .format(len(tokens), len(proposed)))

    report(tokens, proposed, [
        ("the signal itself: agent stripe against the human placeholder",
         "agent", "divider",
         "the pair a reader compares; 3:1 is the threshold for a non-text carrier of meaning"),
        ("the same pair, with the proposed purple",
         "+agent", "divider", None),
        ("agent stripe against the card it stands beside",
         "agent", "surface_field", None),
        ("agent stripe against the feed row it stands beside",
         "agent", "surface_block", None),
        ("proposed purple against the card",
         "+agent", "surface_field", None),
        ("proposed purple against the feed row",
         "+agent", "surface_block", None),
        ("the stripe in use today against the accent button",
         "agent", "accent",
         "what the proposed hue is measured against"),
        ("the open risk: proposed purple stripe against the accent button",
         "+agent", "accent",
         "near 1:1 means the two are separated by hue alone"),
        ("the second colour channel: agent byline against a human one",
         "meta_agent", "meta",
         "a ratio near 1 with a large dE is a difference only a colour screen shows"),
        ("the same pair, with the proposed purple byline",
         "+meta_agent", "meta", None),
        ("byline legibility: agent byline on the feed row",
         "meta_agent", "surface_block",
         "4.5:1 is the threshold for text this size"),
        ("byline legibility: proposed purple byline on the feed row",
         "+meta_agent", "surface_block", None),
    ])


if __name__ == "__main__":
    main()
