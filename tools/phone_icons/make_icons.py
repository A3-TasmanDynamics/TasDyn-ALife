"""Draws the phone's app icons (src/ALife.Altis/ui/icons/*.paa).

Pure Python (no Pillow): each icon is a rounded square in the app's colour
with a white glyph, built from simple shapes and anti-aliased by
supersampling, written as a 128x128 PNG and converted to .paa with Arma 3
Tools' ImageToPAA. Run from the repo root:

    python tools/phone_icons/make_icons.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools" [icon ...]
"""
import math
import os
import struct
import subprocess
import sys
import zlib

SIZE = 128
SS = 4  # supersamples per axis
OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "icons")


# ---- shapes: each is f(x, y) -> bool, coordinates in 0..128 ----

def rrect(cx, cy, w, h, r):
    def f(x, y):
        dx = max(abs(x - cx) - (w / 2 - r), 0)
        dy = max(abs(y - cy) - (h / 2 - r), 0)
        return dx * dx + dy * dy <= r * r and abs(x - cx) <= w / 2 and abs(y - cy) <= h / 2
    return f


def circle(cx, cy, r):
    return lambda x, y: (x - cx) ** 2 + (y - cy) ** 2 <= r * r


def ring(cx, cy, r_out, r_in):
    return lambda x, y: r_in * r_in <= (x - cx) ** 2 + (y - cy) ** 2 <= r_out * r_out


def capsule(x1, y1, x2, y2, r):
    def f(x, y):
        vx, vy = x2 - x1, y2 - y1
        t = max(0, min(1, ((x - x1) * vx + (y - y1) * vy) / (vx * vx + vy * vy)))
        px, py = x1 + t * vx, y1 + t * vy
        return (x - px) ** 2 + (y - py) ** 2 <= r * r
    return f


def poly(points):
    def f(x, y):
        inside = False
        n = len(points)
        for i in range(n):
            x1, y1 = points[i]
            x2, y2 = points[(i + 1) % n]
            if (y1 > y) != (y2 > y) and x < (x2 - x1) * (y - y1) / (y2 - y1) + x1:
                inside = not inside
        return inside
    return f


def rotated_rect(cx, cy, w, h, angle):
    c, s = math.cos(-angle), math.sin(-angle)
    def f(x, y):
        dx, dy = x - cx, y - cy
        rx, ry = dx * c - dy * s, dx * s + dy * c
        return abs(rx) <= w / 2 and abs(ry) <= h / 2
    return f


def union(*shapes):
    return lambda x, y: any(s(x, y) for s in shapes)


def minus(a, b):
    return lambda x, y: a(x, y) and not b(x, y)


# ---- glyphs (white), drawn on top of the coloured tile ----

def star(cx, cy, r_out, r_in):
    pts = []
    for i in range(10):
        r = r_out if i % 2 == 0 else r_in
        a = -math.pi / 2 + i * math.pi / 5
        pts.append((cx + r * math.cos(a), cy + r * math.sin(a)))
    return poly(pts)


def gear(cx, cy):
    teeth = [rotated_rect(cx + 30 * math.cos(a), cy + 30 * math.sin(a), 14, 14, a)
             for a in [i * math.pi / 4 for i in range(8)]]
    return minus(union(circle(cx, cy, 27), *teeth), circle(cx, cy, 11))


GLYPHS = {
    # backpack / box with a handle
    "inventory": union(
        minus(rrect(64, 70, 58, 46, 8), rrect(64, 70, 58, 3, 0)),
        minus(ring(64, 47, 13, 7), lambda x, y: y > 47),  # handle (top half of a ring)
        rrect(64, 62, 16, 8, 3),
    ),
    # bank building: roof, columns, base
    "bank": union(
        poly([(64, 26), (100, 46), (28, 46)]),
        rrect(64, 49, 72, 6, 2),
        rrect(40, 70, 8, 34, 2), rrect(56, 70, 8, 34, 2), rrect(72, 70, 8, 34, 2), rrect(88, 70, 8, 34, 2),
        rrect(64, 92, 76, 8, 3),
    ),
    # chat bubble with three dots cut out
    "messages": minus(
        union(rrect(64, 58, 76, 52, 18), poly([(40, 78), (54, 82), (36, 98)])),
        union(circle(46, 58, 6), circle(64, 58, 6), circle(82, 58, 6)),
    ),
    # ID card: card with a person and text lines cut out
    "playerid": minus(
        rrect(64, 64, 80, 56, 10),
        union(circle(44, 56, 9), rrect(44, 77, 26, 14, 6),
              rrect(82, 52, 26, 5, 2), rrect(82, 64, 26, 5, 2), rrect(78, 76, 18, 5, 2)),
    ),
    "perks": star(64, 66, 38, 16),
    # three people
    "gang": union(
        circle(64, 46, 12), rrect(64, 80, 36, 30, 14),
        circle(38, 54, 9), rrect(38, 82, 26, 22, 10),
        circle(90, 54, 9), rrect(90, 82, 26, 22, 10),
    ),
    # key: ring, shaft, teeth
    "keys": union(
        ring(44, 64, 19, 9),
        rrect(78, 64, 50, 9, 3),
        rrect(92, 74, 8, 14, 2), rrect(80, 72, 7, 10, 2),
    ),
    # shield with a check mark cut out
    "admin": minus(
        poly([(64, 24), (96, 36), (94, 70), (64, 102), (34, 70), (32, 36)]),
        union(capsule(48, 64, 60, 76, 4.5), capsule(60, 76, 82, 52, 4.5)),
    ),
    "settings": gear(64, 64),
    # wanted: a poster with a person and a big "!" mark
    "wanted": minus(
        rrect(64, 64, 76, 92, 8),
        union(circle(64, 50, 13), rrect(64, 76, 34, 18, 8), rrect(64, 100, 50, 6, 2)),
    ),
    # market: a rising price line with an arrow head, over a base line
    "market": union(
        capsule(28, 88, 50, 66, 6), capsule(50, 66, 66, 78, 6), capsule(66, 78, 92, 46, 6),
        poly([(80, 40), (102, 34), (98, 58)]),
        rrect(64, 100, 80, 6, 3),
    ),
}

COLOURS = {
    "inventory": (245, 158, 11),
    "bank": (34, 170, 94),
    "messages": (59, 130, 246),
    "playerid": (20, 160, 150),
    "perks": (139, 92, 246),
    "gang": (220, 68, 68),
    "keys": (217, 160, 6),
    "admin": (71, 85, 105),
    "settings": (107, 114, 128),
    "market": (14, 165, 233),
    "wanted": (190, 40, 40),
}


def render(name):
    tile = rrect(64, 64, 120, 120, 28)
    glyph = GLYPHS[name]
    base = COLOURS[name]
    rows = []
    for py in range(SIZE):
        row = bytearray([0])  # PNG filter: none
        for px in range(SIZE):
            cov_tile = cov_glyph = 0
            for sy in range(SS):
                for sx in range(SS):
                    x = px + (sx + 0.5) / SS
                    y = py + (sy + 0.5) / SS
                    if tile(x, y):
                        cov_tile += 1
                        if glyph(x, y):
                            cov_glyph += 1
            n = SS * SS
            if cov_tile == 0:
                row += bytes((0, 0, 0, 0))
                continue
            # tile colour, a touch lighter at the top
            shade = 1.12 - 0.24 * (py / SIZE)
            tr, tg, tb = (min(255, int(c * shade)) for c in base)
            g = cov_glyph / cov_tile
            r = int(tr + (255 - tr) * g)
            gg = int(tg + (255 - tg) * g)
            b = int(tb + (255 - tb) * g)
            row += bytes((r, gg, b, int(255 * cov_tile / n)))
        rows.append(bytes(row))
    return b"".join(rows)


def write_png(path, raw, width=SIZE, height=SIZE):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)
    only = set(sys.argv[2:])  # optional: just these icons
    for name in GLYPHS:
        if only and name not in only:
            continue
        png = os.path.join(png_dir, f"{name}_ca.png")
        write_png(png, render(name))
        print("drew", png)
        if tools:
            paa = os.path.join(OUT, f"{name}_ca.paa")
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png, paa], check=True)
            print("  ->", paa)


if __name__ == "__main__":
    main()
