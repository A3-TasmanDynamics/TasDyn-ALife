"""Draws the ALife logo: the brush "A".

The original artwork was lost; only a 63x73 screenshot of the old (blue) app
icon survived. Its three brush strokes were measured from that screenshot
(a centre line and a width at each point, in the screenshot's pixels, below)
and are rebuilt here as smooth tapered shapes, in the brand's amber.

    python tools/branding/make_brush_a.py                      # write the files below
    python tools/branding/make_brush_a.py --compare shot.png   # also check the fit

Writes, beside this script:
    alife-a.svg          the A in amber-bright, for dark grounds
    alife-a-ink.svg      the A in navy (bg), for light grounds
    alife-a-tile.svg     the app icon: navy tile, amber A with a soft glow
    alife_bot_avatar.png the Discord avatar (512 px, square; Discord crops it round)
    alife-icon-32.png    the website favicon; alife-icon-180.png the Apple touch icon
    brush_compare.png    (--compare) screenshot | redraw | overlap
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from pngio import read_png, write_png  # noqa: E402  (pngio.py, beside this file)

# Brand colours (the ALife brand system: amber-bright, amber, bg, night-glow).
AMBER_BRIGHT = (0xfb, 0xbf, 0x24)
AMBER = (0xf5, 0x9e, 0x0b)
NAVY = (0x0b, 0x12, 0x20)
NIGHT_GLOW = (0x1d, 0x2b, 0x48)
hexc = lambda c: "#%02x%02x%02x" % c

# (x, y, width) along each stroke, in the screenshot's pixels; width is across the stroke.
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
MINX = min(p[0] for o in OUTLINES for p in o)
MAXX = max(p[0] for o in OUTLINES for p in o)
MINY = min(p[1] for o in OUTLINES for p in o)
MAXY = max(p[1] for o in OUTLINES for p in o)
CX, CY = (MINX + MAXX) / 2, (MINY + MAXY) / 2


def svg_path(poly, scale, ox, oy):
    """Closed Catmull-Rom path through the outline points, as cubic Beziers."""
    pts = poly[::2]
    n = len(pts)
    f = lambda p: ((p[0] - ox) * scale, (p[1] - oy) * scale)
    d = "M%.1f,%.1f" % f(pts[0])
    for i in range(n):
        p0, p1, p2, p3 = pts[(i - 1) % n], pts[i], pts[(i + 1) % n], pts[(i + 2) % n]
        c1 = (p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6)
        c2 = (p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6)
        d += " C%.1f,%.1f %.1f,%.1f %.1f,%.1f" % (*f(c1), *f(c2), *f(p2))
    return d + "Z"


def letter_svg(title, colour):
    K, pad = 10, 2
    paths = "\n".join('    <path d="%s"/>' % svg_path(o, K, MINX - pad, MINY - pad) for o in OUTLINES)
    vw, vh = (MAXX - MINX + 2 * pad) * K, (MAXY - MINY + 2 * pad) * K
    return ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f">\n'
            '  <title>%s</title>\n  <g fill="%s">\n%s\n  </g>\n</svg>\n') % (vw, vh, vw, vh, title, colour, paths)


def tile_svg():
    T, scale = 640, 9.0  # the A ~425 tall on a 640 tile, like the old icon
    paths = "\n".join('    <path d="%s"/>' % svg_path(o, scale, CX - T / 2 / scale, CY - T / 2 / scale) for o in OUTLINES)
    return ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {T} {T}" width="{T}" height="{T}">\n'
            '  <title>ALife icon</title>\n'
            '  <defs>\n'
            '    <radialGradient id="alife-night" cx="50%" cy="45%" r="70%">\n'
            '      <stop offset="0" stop-color="{glow}"/><stop offset="1" stop-color="{navy}"/>\n'
            '    </radialGradient>\n'
            '    <filter id="alife-halo" x="-30%" y="-30%" width="160%" height="160%">\n'
            '      <feGaussianBlur in="SourceGraphic" stdDeviation="16" result="blur"/>\n'
            '      <feColorMatrix in="blur" type="matrix" values="1 0 0 0 0  0 1 0 0 0  0 0 1 0 0  0 0 0 0.55 0" result="soft"/>\n'
            '      <feMerge><feMergeNode in="soft"/><feMergeNode in="SourceGraphic"/></feMerge>\n'
            '    </filter>\n'
            '  </defs>\n'
            '  <rect width="{T}" height="{T}" rx="72" fill="url(#alife-night)"/>\n'
            '  <g fill="{amber}" filter="url(#alife-halo)">\n{paths}\n  </g>\n</svg>\n').format(
        T=T, glow=hexc(NIGHT_GLOW), navy=hexc(NAVY), amber=hexc(AMBER_BRIGHT), paths=paths)


# ---- raster (scanline fill with supersampling) for the avatar ----
def flatten(poly, scale, ox, oy):
    """The smoothed outline as a dense polygon in output pixels."""
    pts = poly[::2]
    n = len(pts)
    outp = []
    for i in range(n):
        p0, p1, p2, p3 = pts[(i - 1) % n], pts[i], pts[(i + 1) % n], pts[(i + 2) % n]
        c1 = (p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6)
        c2 = (p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6)
        for s in range(6):
            t = s / 6
            mt = 1 - t
            x = mt ** 3 * p1[0] + 3 * mt * mt * t * c1[0] + 3 * mt * t * t * c2[0] + t ** 3 * p2[0]
            y = mt ** 3 * p1[1] + 3 * mt * mt * t * c1[1] + 3 * mt * t * t * c2[1] + t ** 3 * p2[1]
            outp.append(((x - ox) * scale, (y - oy) * scale))
    return outp


def coverage(polys, size, ss=4, height=None):
    """0..1 coverage of the union of polygons (each even-odd) on a size-wide grid."""
    height = height or size
    cov = [[0.0] * size for _ in range(height)]
    for row in range(height * ss):
        y = (row + 0.5) / ss
        spans = []
        for poly in polys:
            xs = []
            n = len(poly)
            for i in range(n):
                x1, y1 = poly[i]
                x2, y2 = poly[(i + 1) % n]
                if (y1 > y) != (y2 > y):
                    xs.append(x1 + (y - y1) * (x2 - x1) / (y2 - y1))
            xs.sort()
            spans += [(xs[k], xs[k + 1]) for k in range(0, len(xs) - 1, 2)]
        spans.sort()
        merged = []
        for a, b in spans:
            if merged and a <= merged[-1][1]:
                merged[-1] = (merged[-1][0], max(merged[-1][1], b))
            else:
                merged.append((a, b))
        line = cov[row // ss]
        for a, b in merged:
            a, b = max(a, 0), min(b, size)
            for px in range(int(a), min(int(math.ceil(b)), size)):
                line[px] += (min(b, px + 1) - max(a, px)) / ss
    return cov


def blur(field, r, passes=3):
    """Box blur, separable, `passes` times (close to a Gaussian)."""
    h, w = len(field), len(field[0])
    f = [row[:] for row in field]

    def box(line):
        pre = [0.0]
        for v in line:
            pre.append(pre[-1] + v)
        n = len(line)
        return [(pre[min(n, j + r + 1)] - pre[max(0, j - r)]) / (2 * r + 1) for j in range(n)]
    for _ in range(passes):
        f = [box(row) for row in f]
        cols = [box([f[y][x] for y in range(h)]) for x in range(w)]
        f = [[cols[x][y] for x in range(w)] for y in range(h)]
    return f


def avatar(size=512, height=None, fill=0.68):
    """The A on the night gradient with its amber halo; `fill` = the A's share of the height."""
    height = height or size
    scale = height * fill / (MAXY - MINY)
    ox, oy = CX - size / 2 / scale, CY - height / 2 / scale
    polys = [flatten(o, scale, ox, oy) for o in OUTLINES]
    cov = coverage(polys, size, height=height)
    halo = blur(cov, max(4, height // 40))
    px = []
    for y in range(height):
        row = []
        for x in range(size):
            t = min(1.0, math.hypot(x - size / 2, y - height * 0.45) / (max(size, height) * 0.7))
            bg = [NIGHT_GLOW[i] + (NAVY[i] - NIGHT_GLOW[i]) * t for i in range(3)]
            h = min(1.0, halo[y][x] * 0.55)
            c = [bg[i] * (1 - h) + AMBER[i] * h for i in range(3)]
            a = min(1.0, cov[y][x])
            c = [c[i] * (1 - a) + AMBER_BRIGHT[i] * a for i in range(3)]
            row.append(tuple(int(round(v)) for v in c))
        px.append(row)
    return px


def compare(shot, path):
    """screenshot | redraw | overlap (blue both, amber redraw only, red screenshot only)."""
    W, H, px = read_png(shot)
    S = 8
    polys = [flatten(o, S, 0, 0) for o in OUTLINES]
    cov = coverage(polys, max(W, H) * S, ss=2)
    ink = lambda p: max(0.0, min(1.0, (p[2] - p[0] - 40) / 170.0))
    img = []
    for Y in range(H * S):
        row = []
        for X in range(W * 3 * S):
            panel, X0 = divmod(X, W * S)
            a = cov[Y][X0] if X0 < len(cov) else 0
            if panel == 0:
                row.append(px[Y // S][X0 // S])
            elif panel == 1:
                row.append(tuple(int(26 * (1 - a) + c * a) for c in AMBER_BRIGHT))
            else:
                orig, mine = ink(px[Y // S][X0 // S]) >= 0.5, a >= 0.5
                row.append((13, 127, 234) if mine and orig else (250, 190, 40) if mine else (230, 70, 70) if orig else (26, 26, 26))
        img.append(row)
    write_png(path, W * 3 * S, H * S, img)


if __name__ == "__main__":
    files = {
        "alife-a.svg": letter_svg("ALife", hexc(AMBER_BRIGHT)),
        "alife-a-ink.svg": letter_svg("ALife", hexc(NAVY)),
        "alife-a-tile.svg": tile_svg(),
    }
    for name, text in files.items():
        with open(os.path.join(HERE, name), "w", newline="\n") as f:
            f.write(text)
        print("wrote", name)
    write_png(os.path.join(HERE, "alife_bot_avatar.png"), 512, 512, avatar(512))
    print("wrote alife_bot_avatar.png")
    for size in (32, 180):  # favicon, Apple touch icon
        write_png(os.path.join(HERE, "alife-icon-%d.png" % size), size, size, avatar(size))
        print("wrote alife-icon-%d.png" % size)
    if "--compare" in sys.argv:
        shot = sys.argv[sys.argv.index("--compare") + 1]
        compare(shot, os.path.join(HERE, "brush_compare.png"))
        print("wrote brush_compare.png")
