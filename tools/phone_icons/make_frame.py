"""Draws the phone body textures (src/ALife.Altis/ui/phone/*.paa).

The phone dialog is made of rectangles; the curves come from these
textures: a rounded body with a thin bezel, a rounded screen, a punch-hole
camera and a speaker slot. Two versions:

    phone_<key>_ca.paa -- one per wallpaper (WALLPAPERS below); "dark" is the
                          default and the background apps use

512 x 1024 RGBA, matching the phone's on-screen shape (ui/phone.hpp:
x 0.745-0.950, y 0.26-0.97 of the safe zone, screen inset 0.007 / 0.025).
Pure Python, converted with Arma 3 Tools' ImageToPAA:

    python tools/phone_icons/make_frame.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"
"""
import math
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from make_icons import write_png  # noqa: E402

W, H = 512, 1024
OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "phone")

# Screen inset inside the body, in texture pixels (must match phone.hpp).
SL = round(W * 0.007 / 0.205)       # left/right
ST = round(H * 0.025 / 0.71)        # top/bottom
R_BODY, R_SCREEN = 58, 44


def in_rrect(x, y, x0, y0, x1, y1, r):
    if x < x0 or x > x1 or y < y0 or y > y1:
        return False
    cx = min(max(x, x0 + r), x1 - r)
    cy = min(max(y, y0 + r), y1 - r)
    return (x - cx) ** 2 + (y - cy) ** 2 <= r * r


def lerp(a, b, t):
    return tuple(a[i] + (b[i] - a[i]) * t for i in range(3))


def gradient(stops, t):
    for (t0, c0), (t1, c1) in zip(stops, stops[1:]):
        if t <= t1:
            return lerp(c0, c1, (t - t0) / (t1 - t0) if t1 > t0 else 0)
    return stops[-1][1]


def hexc(h):
    return tuple(int(h[i:i + 2], 16) for i in (1, 3, 5))


SKY = [(0.0, hexc("#1f3f73")), (0.35, hexc("#346aa8")), (0.62, hexc("#7fb0d4")),
       (0.76, hexc("#c9dfe6")), (0.82, hexc("#efe7c4")), (1.0, hexc("#efe7c4"))]


def home_wallpaper(u, v):
    """u, v in 0..1 across the screen."""
    c = gradient(SKY, v)
    # a few stars high up
    for sx, sy, r in [(0.18, 0.08, 0.004), (0.72, 0.05, 0.003), (0.55, 0.14, 0.0035), (0.86, 0.19, 0.003),
                      (0.3, 0.22, 0.0025), (0.64, 0.27, 0.003), (0.1, 0.3, 0.0025)]:
        if (u - sx) ** 2 + ((v - sy) * 2) ** 2 < r * r:
            c = lerp(c, (255, 255, 255), 0.85)
    # far mountains, near hills, foreground with pines at the edges
    far = 0.74 - 0.05 * math.sin(u * 5.0 + 0.6) - 0.025 * math.sin(u * 13 + 2)
    near = 0.82 - 0.03 * math.sin(u * 3.2 + 2.2) - 0.012 * math.sin(u * 17)
    ground = 0.9 - 0.018 * math.sin(u * 4.5 + 1)
    if v > far:
        c = lerp(hexc("#5c8ab5"), hexc("#4a77a3"), min(1, (v - far) / 0.1))
    if v > near:
        c = hexc("#2f5a86")
    for tx, th, tw in [(0.04, 0.2, 0.09), (0.12, 0.14, 0.07), (0.93, 0.18, 0.08), (0.85, 0.11, 0.06)]:
        top = ground - th
        if v > top and abs(u - tx) < tw * (v - top) / th:
            c = hexc("#1f3e63")
    if v > ground:
        c = hexc("#1f3e63")
    return c


def app_background(u, v):
    c = gradient([(0, hexc("#18223a")), (0.25, hexc("#111829")), (1, hexc("#0b0f19"))], v)
    glow = max(0, 1 - ((u - 0.5) ** 2 * 4 + (v * 3) ** 2))
    return lerp(c, hexc("#2b3f6e"), 0.35 * glow)


def ridge(u, base, waves):
    return base - sum(a * math.sin(u * f + p) for a, f, p in waves)


def sunset_wallpaper(u, v):
    c = gradient([(0, hexc("#2a1b4d")), (0.3, hexc("#6b2d6b")), (0.55, hexc("#d9534f")),
                  (0.7, hexc("#f59e0b")), (0.74, hexc("#fde68a")), (1, hexc("#fde68a"))], v)
    # sun on the horizon
    d = math.hypot(u - 0.5, (v - 0.72) * 2)
    if d < 0.16:
        c = lerp(c, hexc("#fff3c4"), min(1, (0.16 - d) / 0.03))
    if v > 0.74:  # sea, with the sun's reflection
        c = gradient([(0.74, hexc("#7c2d4f")), (1, hexc("#2a1240"))], v)
        if abs(u - 0.5) < 0.12 * (1 - (v - 0.74) / 0.3) and int(v * 140) % 3 == 0:
            c = lerp(c, hexc("#fcd34d"), 0.7)
    # headlands sloping down to the sea on both sides
    shore = 0.76 + 0.22 * math.exp(-((u - 0.5) / 0.2) ** 2) - 0.01 * math.sin(u * 23)
    if v > shore:
        c = hexc("#1a0d26")
    return c


def ocean_wallpaper(u, v):
    # dark at the top so the white clock stays readable
    c = gradient([(0, hexc("#062a43")), (0.35, hexc("#0c4a6e")), (0.7, hexc("#0e7490")), (1, hexc("#22b5c9"))], v)
    for i in range(7):
        y = 0.45 + i * 0.08
        if abs(v - ridge(u, y, [(0.012, 9 + i, i)])) < 0.004:
            c = lerp(c, (255, 255, 255), 0.25)
    return c


def forest_wallpaper(u, v):
    c = gradient([(0, hexc("#0f2b1d")), (0.5, hexc("#3f7d5a")), (0.75, hexc("#b7d7a8")), (1, hexc("#e5f2d5"))], v)
    layers = [(0.62, "#5f8f6a", [(0.04, 3, 0), (0.015, 11, 1)]),
              (0.72, "#3d6b4b", [(0.035, 4, 2), (0.01, 17, 0)]),
              (0.84, "#22442f", [(0.025, 5, 1)])]
    for base, colour, waves in layers:
        if v > ridge(u, base, waves):
            c = hexc(colour)
    for tx, th, tw in [(0.06, 0.24, 0.08), (0.16, 0.17, 0.06), (0.83, 0.2, 0.07), (0.94, 0.27, 0.09), (0.5, 0.1, 0.04)]:
        top = 0.94 - th
        if v > top and abs(u - tx) < tw * (v - top) / th:
            c = hexc("#11261a")
    if v > 0.94:
        c = hexc("#11261a")
    return c


def aurora_wallpaper(u, v):
    c = gradient([(0, hexc("#050814")), (0.6, hexc("#0b1530")), (1, hexc("#0e1a36"))], v)
    for centre, colour, strength, waves in [
        (0.28, "#34d399", 0.85, [(0.06, 4, 0), (0.02, 11, 1)]),
        (0.38, "#a78bfa", 0.6, [(0.05, 3, 2), (0.015, 13, 0)]),
        (0.5, "#22d3ee", 0.45, [(0.04, 5, 1)]),
    ]:
        d = v - ridge(u, centre, waves)
        glow = math.exp(-(d / 0.035) ** 2) if d > -0.02 else math.exp(-(d / 0.012) ** 2)
        c = lerp(c, hexc(colour), strength * glow)
    if v > ridge(u, 0.88, [(0.025, 4, 0.5), (0.01, 14, 0)]):
        c = hexc("#03050b")
    for sx, sy in [(0.15, 0.06), (0.8, 0.1), (0.6, 0.04), (0.33, 0.12), (0.9, 0.2), (0.07, 0.18)]:
        if (u - sx) ** 2 + ((v - sy) * 2) ** 2 < 0.003 ** 2:
            c = (240, 240, 255)
    return c


# Wallpapers (key -> generator). Keys match CfgPhone.hpp; "dark" is the
# default and the background apps always use.
WALLPAPERS = {
    "dark": app_background,
    "dusk": home_wallpaper,
    "sunset": sunset_wallpaper,
    "ocean": ocean_wallpaper,
    "forest": forest_wallpaper,
    "aurora": aurora_wallpaper,
}


def render(wallpaper):
    rows = []
    sx0, sy0, sx1, sy1 = SL, ST, W - 1 - SL, H - 1 - ST
    cam_x, cam_y, cam_r = W / 2, ST + 20, 9
    for py in range(H):
        row = bytearray([0])
        for px in range(W):
            body = screen = 0
            for oy in (0.25, 0.75):
                for ox in (0.25, 0.75):
                    x, y = px + ox, py + oy
                    if in_rrect(x, y, 0, 0, W, H, R_BODY):
                        body += 1
                        if in_rrect(x, y, sx0, sy0, sx1 + 1, sy1 + 1, R_SCREEN):
                            screen += 1
            if body == 0:
                row += bytes((0, 0, 0, 0))
                continue
            # bezel: near-black with a faint lighter rim at the outer edge
            rim = not in_rrect(px + 0.5, py + 0.5, 2, 2, W - 2, H - 2, R_BODY - 2)
            bezel = (44, 47, 54) if rim else (10, 11, 14)
            # speaker slot in the top bezel
            if abs(px - W / 2) < 46 and abs(py - ST / 2 + 1) < 2.2:
                bezel = (30, 32, 38)
            c = bezel
            if screen:
                u = (px + 0.5 - sx0) / (sx1 - sx0)
                v = (py + 0.5 - sy0) / (sy1 - sy0)
                wall = wallpaper(u, v)
                # punch-hole camera
                d = math.hypot(px + 0.5 - cam_x, py + 0.5 - cam_y)
                if d < cam_r:
                    wall = (4, 4, 6) if d < cam_r - 2 else (40, 42, 50)
                c = lerp(bezel, wall, screen / 4)
            row += bytes((int(c[0]), int(c[1]), int(c[2]), int(255 * body / 4)))
        rows.append(bytes(row))
    return b"".join(rows)


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)
    for key, wall in WALLPAPERS.items():
        name = f"phone_{key}_ca"
        png = os.path.join(png_dir, name + ".png")
        write_png(png, render(wall), W, H)
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png, os.path.join(OUT, name + ".paa")], check=True)


if __name__ == "__main__":
    main()
