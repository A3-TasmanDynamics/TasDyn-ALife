"""Draws the phone body textures (src/ALife.Altis/ui/phone/*.paa).

The phone dialog is made of rectangles; the curves come from these
textures: a rounded body with a thin bezel, a rounded screen, a punch-hole
camera and a speaker slot. Two versions:

    phone_home_ca.paa -- home screen wallpaper (sky, mountains, hills)
    phone_app_ca.paa  -- dark background for apps (keeps text readable)

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
    for name, wall in (("phone_home_ca", home_wallpaper), ("phone_app_ca", app_background)):
        png = os.path.join(png_dir, name + ".png")
        write_png(png, render(wall), W, H)
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png, os.path.join(OUT, name + ".paa")], check=True)


if __name__ == "__main__":
    main()
