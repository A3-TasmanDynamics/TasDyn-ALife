"""Draws the ALife wordmark: the brush A (make_brush_a.py) followed by "Life"
lettered in the same hand -- quick tapered strokes with the same lean and
weight. There is no font behind it: each letter is a few strokes, each a
centre line with a width at every point (in the same units as the A, where
the A is ~49 tall).

    python tools/branding/make_wordmark.py                    # SVGs + the loading picture
    python tools/branding/make_wordmark.py --preview out.png  # also a quick look on navy

Writes alife-wordmark.svg (A amber-bright, Life white: dark grounds),
alife-wordmark-ink.svg (all navy: light grounds) and alife_loadscreen.png
(1024x512, the mission's loading picture -> ui/brand/alife_load_co.paa).
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import make_brush_a as A  # noqa: E402
from pngio import write_png  # noqa: E402

SLANT = 0.22      # forward lean: x shifts this much per unit of height
BASE = 60.0       # baseline (the A's feet sit about here)
SHIFT = -3.5      # pulls "Life" in towards the A


def lean(stroke):
    """Shear an upright stroke forward to the A's lean."""
    return [(x + (BASE - y) * SLANT + SHIFT, y, w) for x, y, w in stroke]


# "Life", drawn upright (lean() adds the slant). x positions follow the A,
# which ends at about x = 44.
L_STEM = [(51.0, 14.6, 0), (51.3, 17.0, 3.0), (51.2, 23.0, 4.8), (50.6, 33.0, 5.4), (49.9, 43.0, 5.4),
          (49.3, 51.0, 5.0), (49.0, 56.0, 4.2), (49.3, 58.8, 2.8), (50.6, 60.2, 0)]
L_FOOT = [(47.4, 59.8, 0), (50.0, 59.5, 2.8), (55.0, 59.0, 3.8), (60.0, 58.0, 3.4), (63.6, 56.4, 2.2), (65.4, 54.6, 0)]
I_STEM = [(70.4, 37.0, 0), (70.6, 39.2, 2.8), (70.4, 45.0, 4.6), (70.0, 52.0, 4.8), (70.0, 56.4, 3.8),
          (71.2, 59.3, 2.4), (73.6, 59.0, 0)]
I_DOT = [(70.4, 29.2, 0), (71.4, 28.0, 3.8), (72.8, 26.8, 0)]
F_STEM = [(91.4, 16.8, 0), (89.8, 14.6, 2.6), (87.0, 14.0, 3.8), (84.6, 16.4, 4.6), (83.4, 22.0, 5.2),
          (82.8, 32.0, 5.4), (82.3, 42.0, 5.2), (81.8, 51.0, 4.6), (81.2, 57.0, 3.6), (80.2, 61.5, 1.8), (78.6, 64.4, 0)]
F_BAR = [(75.6, 38.0, 0), (78.8, 37.4, 2.6), (84.0, 36.6, 3.4), (89.0, 35.8, 2.8), (92.0, 34.8, 0)]
E_LOOP = [(94.2, 50.0, 1.6), (95.8, 49.6, 2.6), (98.0, 48.9, 3.0), (102.8, 46.6, 3.8), (105.6, 42.8, 4.4), (104.4, 38.8, 4.2),
          (100.6, 37.6, 4.0), (96.8, 40.2, 4.2), (94.8, 46.2, 4.4), (94.9, 52.8, 4.4), (97.4, 57.8, 4.4),
          (101.8, 59.6, 3.6), (106.2, 58.0, 2.4), (108.8, 55.4, 0)]
LIFE = [L_STEM, L_FOOT, I_STEM, I_DOT, F_STEM, F_BAR, E_LOOP]
LIFE_OUTLINES = [A.outline(lean(s)) for s in LIFE]

ALL = A.OUTLINES + LIFE_OUTLINES
MINX = min(p[0] for o in ALL for p in o)
MAXX = max(p[0] for o in ALL for p in o)
MINY = min(p[1] for o in ALL for p in o)
MAXY = max(p[1] for o in ALL for p in o)


def wordmark_svg(a_colour, life_colour, title="ALife"):
    K, pad = 10, 2
    ox, oy = MINX - pad, MINY - pad
    a = "\n".join('    <path d="%s"/>' % A.svg_path(o, K, ox, oy) for o in A.OUTLINES)
    life = "\n".join('    <path d="%s"/>' % A.svg_path(o, K, ox, oy) for o in LIFE_OUTLINES)
    vw, vh = (MAXX - MINX + 2 * pad) * K, (MAXY - MINY + 2 * pad) * K
    return ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f">\n'
            '  <title>%s</title>\n'
            '  <g fill="%s">\n%s\n  </g>\n'
            '  <g fill="%s">\n%s\n  </g>\n</svg>\n') % (vw, vh, vw, vh, title, a_colour, a, life_colour, life)


def preview(path, scale=6):
    """Render on navy: A amber-bright, Life white."""
    pad = 4
    w = int((MAXX - MINX + 2 * pad) * scale)
    h = int((MAXY - MINY + 2 * pad) * scale)
    ox, oy = MINX - pad, MINY - pad
    cov_a = A.coverage([A.flatten(o, scale, ox, oy) for o in A.OUTLINES], w, ss=3, height=h)
    cov_l = A.coverage([A.flatten(o, scale, ox, oy) for o in LIFE_OUTLINES], w, ss=3, height=h)
    white = (0xf1, 0xf5, 0xf9)
    px = []
    for y in range(h):
        row = []
        for x in range(w):
            c = list(A.NAVY)
            a, l = min(1.0, cov_a[y][x]), min(1.0, cov_l[y][x])
            c = [c[i] * (1 - a) + A.AMBER_BRIGHT[i] * a for i in range(3)]
            c = [c[i] * (1 - l) + white[i] * l for i in range(3)]
            row.append(tuple(int(v) for v in c))
        px.append(row)
    write_png(path, w, h, px)


def loadscreen(w=1024, h=512, fill=0.42):
    """The wordmark on the night gradient with the A's amber halo (Arma loadScreen, 2:1)."""
    import math
    scale = h * fill / (MAXY - MINY)
    cx, cy = (MINX + MAXX) / 2, (MINY + MAXY) / 2
    ox, oy = cx - w / 2 / scale, cy - h / 2 / scale
    cov_a = A.coverage([A.flatten(o, scale, ox, oy) for o in A.OUTLINES], w, height=h)
    cov_l = A.coverage([A.flatten(o, scale, ox, oy) for o in LIFE_OUTLINES], w, height=h)
    halo = A.blur(cov_a, max(4, h // 40))
    white = (0xf1, 0xf5, 0xf9)
    px = []
    for y in range(h):
        row = []
        for x in range(w):
            t = min(1.0, math.hypot(x - w / 2, y - h * 0.45) / (max(w, h) * 0.7))
            c = [A.NIGHT_GLOW[i] + (A.NAVY[i] - A.NIGHT_GLOW[i]) * t for i in range(3)]
            hl = min(1.0, halo[y][x] * 0.55)
            c = [c[i] * (1 - hl) + A.AMBER[i] * hl for i in range(3)]
            a, l = min(1.0, cov_a[y][x]), min(1.0, cov_l[y][x])
            c = [c[i] * (1 - a) + A.AMBER_BRIGHT[i] * a for i in range(3)]
            c = [c[i] * (1 - l) + white[i] * l for i in range(3)]
            row.append(tuple(int(round(v)) for v in c))
        px.append(row)
    return px


if __name__ == "__main__":
    files = {
        "alife-wordmark.svg": wordmark_svg(A.hexc(A.AMBER_BRIGHT), "#f1f5f9"),
        "alife-wordmark-ink.svg": wordmark_svg(A.hexc(A.NAVY), A.hexc(A.NAVY)),
    }
    for name, text in files.items():
        with open(os.path.join(HERE, name), "w", newline="\n") as f:
            f.write(text)
        print("wrote", name)
    write_png(os.path.join(HERE, "alife_loadscreen.png"), 1024, 512, loadscreen())
    print("wrote alife_loadscreen.png")
    if "--preview" in sys.argv:
        out = sys.argv[sys.argv.index("--preview") + 1]
        preview(out)
        print("wrote", out)
