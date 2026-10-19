"""Draws the interaction wheel images (src/ALife.Altis/ui/radial/*.paa) for
ALife_fnc_radial: a solid ring cut into slices around a centre disc.
Everything is white on transparent; the game tints it (ctrlSetTextColor).

Nothing is rotated in game -- Arma rotates pictures in non-square screen
units, which stretches them -- so the hover highlight is drawn for every
slice position of every slice count:

    python tools/phone_icons/make_radial.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"

    ring_ca                 - the ring the slices sit on
    centre_ca               - the disc in the middle (the label), touching the ring
    rim_ca                  - a thin line where the ring meets the centre
    lines_<n>_ca            - the n divider lines between slices (inset from the edges)
    wedge_<n>_<i>_ca        - slice i of n (clockwise from 12 o'clock): the hover fill
    accent_<n>_<i>_ca       - slice i of n: the amber arc on the ring's outer edge
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
C = 64                    # centre of the 128 px image
INNER, OUTER = 27, 61     # the ring
ACCENT = (57.5, 61)       # the amber arc: the ring's outer edge
GAP = 0.035               # radians left clear either side of each slice's highlight


def angle(x, y):
    """0 at 12 o'clock, clockwise, in radians."""
    return math.atan2(x - C, -(y - C)) % (2 * math.pi)


def sector(r_in, r_out, centre, width):
    """The part of a ring within width / 2 radians of `centre` (radians)."""
    band = ring(C, C, r_out, r_in)

    def f(x, y):
        if not band(x, y):
            return False
        d = (angle(x, y) - centre + math.pi) % (2 * math.pi) - math.pi
        return abs(d) <= width / 2
    return f


def lines(n, thickness=0.6):
    """Divider lines between n slices (slices centred on 12 o'clock), kept
    clear of the ring's edges."""
    cuts = [(i + 0.5) * 2 * math.pi / n for i in range(n)]

    def f(x, y):
        r = math.hypot(x - C, y - C)
        if r < INNER + 4 or r > OUTER - 4:
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
        "centre": [(circle(C, C, INNER), WHITE)],
        "rim": [(ring(C, C, INNER + 0.7, INNER - 0.7), WHITE)],
    }
    for n in range(2, 9):
        width = 2 * math.pi / n
        images[f"lines_{n}"] = [(lines(n), WHITE)]
        for i in range(n):
            centre = i * width
            images[f"wedge_{n}_{i}"] = [(sector(INNER + 1, OUTER, centre, width - 2 * GAP), WHITE)]
            images[f"accent_{n}_{i}"] = [(sector(ACCENT[0], ACCENT[1], centre, width - 2 * GAP), WHITE)]

    # Old images from the rotating version.
    for old in os.listdir(OUT):
        name = old[:-len("_ca.paa")] if old.endswith("_ca.paa") else None
        if name and name not in images:
            os.remove(os.path.join(OUT, old))

    for name, layers in images.items():
        png = os.path.join(png_dir, f"radial_{name}_ca.png")
        write_png(png, render(layers))
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True, capture_output=True)
    print(f"drew {len(images)} images")


if __name__ == "__main__":
    main()
