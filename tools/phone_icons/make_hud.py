"""Draws the player HUD images (src/ALife.Altis/ui/hud/*.paa): white glyphs
for health / food / water, and the ring pieces for the "rings" HUD style.
Everything is white on transparent; the game tints it (ctrlSetTextColor).
Same pure-Python drawing as make_icons.py; converted with Arma 3 Tools'
ImageToPAA:

    python tools/phone_icons/make_hud.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"

    health_ca / food_ca / water_ca  - the glyphs
    ring_bg_ca                      - the full ring (the empty track)
    ring_00_ca .. ring_20_ca        - the filled arc, 0% .. 100% in 5% steps,
                                      clockwise from the top
"""
import math
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from make_icons import capsule, circle, poly, ring, union, write_png  # noqa: E402
from make_items import render  # noqa: E402

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "hud")
WHITE = (255, 255, 255)


def ellipse(cx, cy, a, b, angle):
    c, s = math.cos(angle), math.sin(angle)
    return lambda x, y: (((x - cx) * c + (y - cy) * s) / a) ** 2 + ((-(x - cx) * s + (y - cy) * c) / b) ** 2 <= 1


GLYPHS = {
    # A heart: two lobes and a point.
    "health": union(circle(44, 50, 26), circle(84, 50, 26), poly([(20, 62), (108, 62), (64, 110)])),
    # A drumstick: a tilted oval of meat, a short bone and its knuckle.
    "food": union(
        ellipse(50, 50, 40, 29, math.pi / 4),
        capsule(72, 72, 94, 94, 8),
        circle(92, 104, 10), circle(104, 92, 10),
    ),
    # A drop of water.
    "water": union(circle(64, 82, 32), poly([(64, 10), (33, 74), (95, 74)])),
}


def arc(fraction, r_out=60, r_in=49):
    """The ring from 12 o'clock clockwise, `fraction` of the way round."""
    band = ring(64, 64, r_out, r_in)

    def f(x, y):
        if not band(x, y):
            return False
        a = math.atan2(x - 64, -(y - 64)) % (2 * math.pi)  # 0 at the top, clockwise
        return a <= fraction * 2 * math.pi
    return f


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)

    images = {name: [(shape, WHITE)] for name, shape in GLYPHS.items()}
    images["ring_bg"] = [(ring(64, 64, 60, 49), WHITE)]
    for step in range(21):
        images["ring_%02d" % step] = [(arc(step / 20), WHITE)] if step else []

    for name, layers in images.items():
        png = os.path.join(png_dir, f"hud_{name}_ca.png")
        write_png(png, render(layers))
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True)


if __name__ == "__main__":
    main()
