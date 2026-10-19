"""Draws the interaction wheel images (src/ALife.Altis/ui/radial/*.paa) for
ALife_fnc_radial: a solid ring cut into slices, a centre disc, and for each
slice count (2-8) the highlight wedge, the divider lines and the accent arc.
Everything is white on transparent; the game tints it (ctrlSetTextColor) and
turns the wedge / arc to the slice under the mouse (ctrlSetAngle). Same
pure-Python drawing as make_icons.py; converted with Arma 3 Tools' ImageToPAA:

    python tools/phone_icons/make_radial.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"

    ring_ca                      - the ring the slices sit on
    centre_ca                    - the disc in the middle (the label)
    wedge_<n>_ca                 - one slice of n, centred on 12 o'clock
    lines_<n>_ca                 - the n divider lines between slices
    accent_<n>_ca                - a thin arc outside one slice of n
"""
import math
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from make_icons import circle, ring, write_png  # noqa: E402
from make_items import render  # noqa: E402

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "radial")
WHITE = (255, 255, 255)
C = 64             # centre of the 128 px image
INNER, OUTER = 30, 58.5  # the ring
ACCENT = (60.5, 63.5)    # the accent arc, just outside it
CENTRE = 27.5            # the middle disc (a dark gap between it and the ring)


def angle(x, y):
    """0 at 12 o'clock, clockwise, in radians."""
    return math.atan2(x - C, -(y - C)) % (2 * math.pi)


def sector(r_in, r_out, width):
    """The part of a ring within `width` radians either side of 12 o'clock / 2."""
    band = ring(C, C, r_out, r_in)

    def f(x, y):
        if not band(x, y):
            return False
        a = angle(x, y)
        return a <= width / 2 or a >= 2 * math.pi - width / 2
    return f


def lines(n, thickness=0.9):
    """Divider lines between n slices (slices are centred on 12 o'clock)."""
    cuts = [(i + 0.5) * 2 * math.pi / n for i in range(n)]

    def f(x, y):
        r = math.hypot(x - C, y - C)
        if r < INNER or r > OUTER:
            return False
        for cut in cuts:
            dx, dy = math.sin(cut), -math.cos(cut)
            if abs((x - C) * dy - (y - C) * dx) <= thickness and (x - C) * dx + (y - C) * dy > 0:
                return True
        return False
    return f


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)

    images = {
        "ring": [(ring(C, C, OUTER, INNER), WHITE)],
        "centre": [(circle(C, C, CENTRE), WHITE)],
    }
    for n in range(2, 9):
        width = 2 * math.pi / n
        images[f"wedge_{n}"] = [(sector(INNER, OUTER, width), WHITE)]
        images[f"lines_{n}"] = [(lines(n), WHITE)]
        images[f"accent_{n}"] = [(sector(ACCENT[0], ACCENT[1], width * 0.96), WHITE)]

    for name, layers in images.items():
        png = os.path.join(png_dir, f"radial_{name}_ca.png")
        write_png(png, render(layers))
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True)


if __name__ == "__main__":
    main()
