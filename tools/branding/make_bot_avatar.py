"""Draws the ALife Discord bot's avatar (512x512 PNG), in the Server
Manager / phone palette: a navy disc, an amber ring and a heartbeat line --
the "life" in ALife. Pure Python, same drawing helpers as the phone icons.

    python tools/branding/make_bot_avatar.py

Writes tools/branding/alife_bot_avatar.png; upload it on the bot's page in
the Discord Developer Portal (Bot -> Icon) and the app's General
Information page (App Icon).
"""
import math
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "phone_icons"))
from make_icons import capsule, circle, ring, union, write_png  # noqa: E402

SIZE = 512
SS = 3  # supersamples per axis
OUT = os.path.join(os.path.dirname(__file__), "alife_bot_avatar.png")

C = SIZE / 2
NAVY_IN, NAVY_OUT = (29, 43, 72), (11, 18, 32)
AMBER, AMBER_BRIGHT = (245, 158, 11), (251, 191, 36)

# Heartbeat: flat, a small bump, the spike, a dip, flat again.
PULSE = [(106, 270), (190, 270), (215, 240), (238, 270), (262, 270), (292, 130),
         (322, 380), (350, 236), (372, 270), (406, 270)]
pulse = union(*[capsule(x1, y1, x2, y2, 13) for (x1, y1), (x2, y2) in zip(PULSE, PULSE[1:])])
outer_ring = ring(C, C, 214, 196)
inner_ring = ring(C, C, 182, 178)
disc = circle(C, C, 240)


def colour_at(x, y):
    """Colour of one sample, painted back to front; None = transparent."""
    if not disc(x, y):
        return None
    # Navy radial gradient, lighter in the middle.
    t = min(1.0, math.hypot(x - C, y - C) / 240)
    col = [NAVY_IN[i] + (NAVY_OUT[i] - NAVY_IN[i]) * t for i in range(3)]
    if inner_ring(x, y):
        col = [col[i] * 0.6 + AMBER[i] * 0.4 for i in range(3)]
    if outer_ring(x, y):
        col = list(AMBER)
    if pulse(x, y):
        col = list(AMBER_BRIGHT)
    return col


def render():
    rows = []
    for py in range(SIZE):
        row = bytearray([0])
        for px in range(SIZE):
            acc, cover = [0.0, 0.0, 0.0], 0
            for sy in range(SS):
                for sx in range(SS):
                    c = colour_at(px + (sx + 0.5) / SS, py + (sy + 0.5) / SS)
                    if c:
                        cover += 1
                        for i in range(3):
                            acc[i] += c[i]
            if cover:
                row += bytes((int(acc[0] / cover), int(acc[1] / cover), int(acc[2] / cover), int(255 * cover / (SS * SS))))
            else:
                row += bytes((0, 0, 0, 0))
        rows.append(bytes(row))
    return b"".join(rows)


if __name__ == "__main__":
    write_png(OUT, render(), SIZE, SIZE)
    print("wrote", OUT)
