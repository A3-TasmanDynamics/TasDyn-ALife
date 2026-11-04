"""Draws the interaction wheel images (src/ALife.Altis/ui/radial/*.paa) for
ALife_fnc_radial: n separate slice tiles around a centre disc, with clear
gaps between them. Everything is white on transparent; the game tints it
(colorText / ctrlSetTextColor). Drawn at 4x with PIL and scaled down to
512 px, so the edges stay smooth at the wheel's on-screen size.

Nothing is rotated in game -- Arma rotates pictures in non-square screen
units, which stretches them -- so the hover images are drawn for every
slice position of every slice count:

    python tools/phone_icons/make_radial.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"

    tiles_<n>_ca            - the n slice tiles (clockwise from 12 o'clock)
    wedge_<n>_<i>_ca        - slice i of n: the hover fill
    accent_<n>_<i>_ca       - slice i of n: the amber band on its outer edge
    centre_ca               - the disc in the middle (the label)

ALife_fnc_radial's CENTRE_R / SLICE_R / TILE_IN match INNER / OUTER below.
"""
import math
import os
import subprocess
import sys

from PIL import Image, ImageDraw

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "radial")
SIZE = 512
SS = 4                     # supersampling
C = SIZE / 2
CENTRE = 100               # centre disc radius
INNER, OUTER = 112, 246    # the tiles
ACCENT = 9                 # the amber band's depth, on the outer edge
GAP = 7                    # px between tiles (constant width, not an angle)


def blank():
    return Image.new("L", (SIZE * SS, SIZE * SS), 0)


def disc(draw, r, fill):
    s = SS
    draw.ellipse([(C - r) * s, (C - r) * s, (C + r) * s, (C + r) * s], fill=fill)


def tile(n, i, r_in, r_out):
    """Slice i of n as a mask: the ring between r_in and r_out, centred on
    i * 360 / n degrees from 12 o'clock, with half a gap cut off each side."""
    m = blank()
    d = ImageDraw.Draw(m)
    width = 360 / n
    centre = -90 + i * width                     # PIL angles: 0 = 3 o'clock, clockwise
    d.pieslice([(C - r_out) * SS, (C - r_out) * SS, (C + r_out) * SS, (C + r_out) * SS],
               centre - width / 2, centre + width / 2, fill=255)
    disc(d, r_in, 0)
    # The gaps: a band of constant width along each edge of the slice.
    for edge in (centre - width / 2, centre + width / 2):
        a = math.radians(edge)
        dx, dy = math.cos(a), math.sin(a)
        nx, ny = -dy, dx
        far = OUTER + 20
        g = GAP / 2
        pts = [(C + nx * g, C + ny * g), (C + nx * g + dx * far, C + ny * g + dy * far),
               (C - nx * g + dx * far, C - ny * g + dy * far), (C - nx * g, C - ny * g)]
        d.polygon([(x * SS, y * SS) for x, y in pts], fill=0)
    return m


def finish(mask):
    """White, with the mask as alpha, scaled down smoothly."""
    alpha = mask.resize((SIZE, SIZE), Image.LANCZOS)
    img = Image.new("RGBA", (SIZE, SIZE), (255, 255, 255, 0))
    img.putalpha(alpha)
    return img


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)

    images = {}
    c = blank()
    disc(ImageDraw.Draw(c), CENTRE, 255)
    images["centre"] = c
    for n in range(2, 9):
        all_tiles = blank()
        for i in range(n):
            t = tile(n, i, INNER, OUTER)
            all_tiles.paste(255, mask=t)
            images[f"wedge_{n}_{i}"] = t
            images[f"accent_{n}_{i}"] = tile(n, i, OUTER - ACCENT, OUTER)
        images[f"tiles_{n}"] = all_tiles

    # Images of earlier wheel designs.
    for old in os.listdir(OUT):
        name = old[:-len("_ca.paa")] if old.endswith("_ca.paa") else None
        if name and name not in images:
            os.remove(os.path.join(OUT, old))

    for name, mask in images.items():
        png = os.path.join(png_dir, f"radial_{name}_ca.png")
        finish(mask).save(png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True, capture_output=True)
    print(f"drew {len(images)} images")


if __name__ == "__main__":
    main()
