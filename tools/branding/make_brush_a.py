"""Redraws the ALife brush "A" logo as SVG. The original artwork was lost;
only a 63x73 screenshot of the app icon survived, so the three brush
strokes were measured from it (centre line + width at each point) and are
rebuilt here as smooth tapered shapes. Writes alife-a.svg (the letter) and
alife-a-tile.svg (the app icon: #1a1a1a tile, soft glow), plus a
comparison image against the screenshot.

    python tools/branding/make_brush_a.py <screenshot.png> <out_dir>
"""
import math
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from pngio import read_png, write_png  # noqa: E402  (pngio.py, beside this file)

src, out = sys.argv[1], sys.argv[2]
W, H, px = read_png(src)

# (x, y, width) along each stroke, in original pixels; width is across the stroke.
LEFT = [(8.4, 56.2, 0), (9.6, 54.6, 2.8), (12.3, 50.2, 4.6), (16.3, 44.2, 5.0), (19.6, 40.0, 4.9),
        (23.1, 35.8, 4.6), (26.2, 32.0, 4.2), (29.2, 28.0, 4.0), (31.6, 25.0, 3.9), (34.3, 21.8, 3.7),
        (37.0, 17.8, 3.0), (39.4, 14.2, 1.9), (40.6, 12.0, 0)]
RIGHT = [(40.6, 12.0, 0), (41.0, 14.4, 2.0), (40.7, 18.0, 3.3), (39.9, 22.0, 3.9), (39.2, 25.0, 4.1),
         (38.4, 28.0, 4.2), (37.4, 31.5, 4.4), (36.3, 37.0, 4.9), (35.6, 40.5, 5.1), (34.8, 44.5, 5.1),
         (34.0, 48.0, 5.0), (33.2, 51.5, 4.9), (32.6, 54.5, 4.7), (32.3, 57.0, 4.0), (32.2, 59.2, 2.6),
         (32.2, 61.2, 0)]
BAR = [(13.0, 38.9, 0), (15.5, 38.0, 1.6), (19.0, 37.0, 2.8), (24.0, 35.8, 3.4), (30.0, 34.6, 3.5),
       (36.0, 33.4, 3.3), (40.8, 32.5, 3.0), (43.2, 32.1, 2.3), (44.3, 32.0, 0)]
STROKES = [LEFT, RIGHT, BAR]


def catmull(pts, samples=16):
    """Smooth curve through (x, y, w) control points: Catmull-Rom, w carried along."""
    outp = []
    n = len(pts)
    for i in range(n - 1):
        p0 = pts[max(i - 1, 0)]
        p1, p2 = pts[i], pts[i + 1]
        p3 = pts[min(i + 2, n - 1)]
        for s in range(samples):
            t = s / samples
            t2, t3 = t * t, t * t * t
            outp.append(tuple(0.5 * ((2 * p1[k]) + (-p0[k] + p2[k]) * t + (2 * p0[k] - 5 * p1[k] + 4 * p2[k] - p3[k]) * t2
                                     + (-p0[k] + 3 * p1[k] - 3 * p2[k] + p3[k]) * t3) for k in range(3)))
    outp.append(pts[-1])
    return outp


def outline(stroke):
    """The stroke's closed outline: one side forward, the other back."""
    c = catmull(stroke)
    left, right = [], []
    for i, (x, y, w) in enumerate(c):
        a = c[max(i - 1, 0)]
        b = c[min(i + 1, len(c) - 1)]
        dx, dy = b[0] - a[0], b[1] - a[1]
        L = math.hypot(dx, dy) or 1
        nx, ny = -dy / L, dx / L
        h = max(w, 0) / 2
        left.append((x + nx * h, y + ny * h))
        right.append((x - nx * h, y - ny * h))
    return left + right[::-1][1:-1]


OUTLINES = [outline(s) for s in STROKES]


def svg_path(poly, scale, ox=0, oy=0):
    """Closed Catmull-Rom path through the outline points, as cubic Beziers."""
    pts = poly[::2]  # every other sample is plenty once smoothed
    n = len(pts)
    f = lambda p: ((p[0] - ox) * scale, (p[1] - oy) * scale)
    d = "M%.1f,%.1f" % f(pts[0])
    for i in range(n):
        p0, p1, p2, p3 = pts[(i - 1) % n], pts[i], pts[(i + 1) % n], pts[(i + 2) % n]
        c1 = (p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6)
        c2 = (p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6)
        d += " C%.1f,%.1f %.1f,%.1f %.1f,%.1f" % (*f(c1), *f(c2), *f(p2))
    return d + "Z"


# ---- render for comparison (nonzero: strokes overlap) ----
def inside_poly(poly, x, y):
    c = False
    n = len(poly)
    for i in range(n):
        x1, y1 = poly[i]
        x2, y2 = poly[(i + 1) % n]
        if (y1 > y) != (y2 > y) and x < (x2 - x1) * (y - y1) / (y2 - y1) + x1:
            c = not c
    return c


def ink(p):
    return max(0.0, min(1.0, (p[2] - p[0] - 40) / 170.0))


S = 8
SS = 1
img = []
for Y in range(H * S):
    row = []
    for X in range(W * 3 * S):
        if X < W * S:
            row.append(px[Y // S][X // S])
            continue
        mode = 1 if X < W * 2 * S else 2
        X0 = X - W * S * mode
        cover = 0
        for sy in range(SS):
            for sx in range(SS):
                x = (X0 + (sx + 0.5) / SS) / S
                y = (Y + (sy + 0.5) / SS) / S
                if any(inside_poly(o, x, y) for o in OUTLINES):
                    cover += 1
        a = cover / (SS * SS)
        if mode == 1:  # clean render on the tile colour
            row.append(tuple(int(26 * (1 - a) + c * a) for c in (13, 127, 234)))
        else:  # overlap: blue both, amber only mine, red only original
            orig = ink(px[Y // S][X0 // S]) >= 0.5
            mine = a >= 0.5
            row.append((13, 127, 234) if mine and orig else (250, 190, 40) if mine else (230, 70, 70) if orig else (26, 26, 26))
    img.append(row)
write_png(os.path.join(out, "brush_compare.png"), W * 3 * S, H * S, img)

# ---- SVGs: the letter alone, and the app tile ----
minx = min(p[0] for o in OUTLINES for p in o)
maxx = max(p[0] for o in OUTLINES for p in o)
miny = min(p[1] for o in OUTLINES for p in o)
maxy = max(p[1] for o in OUTLINES for p in o)
print("ink bbox: x %.1f-%.1f y %.1f-%.1f" % (minx, maxx, miny, maxy))

K = 10  # 10 units per original pixel
pad = 2
paths = [svg_path(o, K, minx - pad, miny - pad) for o in OUTLINES]
vw, vh = (maxx - minx + 2 * pad) * K, (maxy - miny + 2 * pad) * K
letter = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f">\n'
          '  <title>ALife A</title>\n  <g fill="#0d7fea">\n%s\n  </g>\n</svg>\n') % (
    vw, vh, vw, vh, "\n".join('    <path d="%s"/>' % d for d in paths))
open(os.path.join(out, "alife-a.svg"), "w").write(letter)

# Tile: the original is 63 wide with ~7px corners; centre the letter on a square tile.
T = 640
scale = 9.0  # letter height ~ 47px * 9 = 425 on a 640 tile
cx, cy = (minx + maxx) / 2, (miny + maxy) / 2
tile_paths = [svg_path(o, scale, cx - T / 2 / scale, cy - T / 2 / scale) for o in OUTLINES]
tile = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">\n'
        '  <title>ALife icon</title>\n'
        '  <defs>\n'
        '    <filter id="alife-glow" x="-30%%" y="-30%%" width="160%%" height="160%%">\n'
        '      <feGaussianBlur in="SourceGraphic" stdDeviation="14" result="blur"/>\n'
        '      <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>\n'
        '    </filter>\n'
        '  </defs>\n'
        '  <rect width="%d" height="%d" rx="72" fill="#1a1a1a"/>\n'
        '  <g fill="#0d7fea" filter="url(#alife-glow)">\n%s\n  </g>\n</svg>\n') % (
    T, T, T, T, T, T, "\n".join('    <path d="%s"/>' % d for d in tile_paths))
open(os.path.join(out, "alife-a-tile.svg"), "w").write(tile)
print("wrote alife-a.svg, alife-a-tile.svg, brush_compare.png")
