"""Draws the virtual-item icons (src/ALife.Altis/ui/items/*.paa), used in
the phone's Inventory, shops and the inventory screen. Transparent
background (they sit in list rows). Same pure-Python drawing as
make_icons.py; converted with Arma 3 Tools' ImageToPAA:

    python tools/phone_icons/make_items.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools"

Add an item: draw it in ITEMS, re-run, and point its CfgItems `icon` at
ui\\items\\<name>_ca.paa.
"""
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from make_icons import SIZE, SS, capsule, circle, minus, ring, rrect, union, write_png  # noqa: E402

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "items")

# Each item: list of (shape, (r, g, b)) layers, painted in order.
ITEMS = {
    "medkit": [
        (minus(ring(64, 34, 20, 12), lambda x, y: y > 40), (180, 40, 46)),           # handle
        (rrect(64, 74, 100, 70, 14), (229, 72, 77)),                                  # case
        (rrect(64, 74, 100, 8, 0), (190, 50, 56)),                                    # seam
        (union(rrect(64, 74, 16, 46, 3), rrect(64, 74, 46, 16, 3)), (255, 255, 255)),  # cross
    ],
    "cash": [
        (rrect(70, 58, 100, 52, 6), (30, 120, 70)),                                   # back note
        (rrect(58, 72, 100, 52, 6), (52, 168, 98)),                                   # front note
        (minus(rrect(58, 72, 86, 38, 4), rrect(58, 72, 82, 34, 3)), (190, 240, 205)),  # border
        (circle(58, 72, 12), (190, 240, 205)),                                        # medallion
        (circle(58, 72, 8), (52, 168, 98)),
        (capsule(58, 66, 58, 78, 1.6), (190, 240, 205)),
    ],
}


def render(layers):
    rows = []
    for py in range(SIZE):
        row = bytearray([0])
        for px in range(SIZE):
            acc = [0.0, 0.0, 0.0]
            cover = 0
            for sy in range(SS):
                for sx in range(SS):
                    x = px + (sx + 0.5) / SS
                    y = py + (sy + 0.5) / SS
                    colour = None
                    for shape, c in layers:
                        if shape(x, y):
                            colour = c
                    if colour:
                        cover += 1
                        for i in range(3):
                            acc[i] += colour[i]
            if cover:
                row += bytes((int(acc[0] / cover), int(acc[1] / cover), int(acc[2] / cover), int(255 * cover / (SS * SS))))
            else:
                row += bytes((0, 0, 0, 0))
        rows.append(bytes(row))
    return b"".join(rows)


def main():
    tools = sys.argv[1] if len(sys.argv) > 1 else None
    os.makedirs(OUT, exist_ok=True)
    png_dir = os.path.join(os.path.dirname(__file__), "png")
    os.makedirs(png_dir, exist_ok=True)
    for name, layers in ITEMS.items():
        png = os.path.join(png_dir, f"item_{name}_ca.png")
        write_png(png, render(layers))
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True)


if __name__ == "__main__":
    main()
