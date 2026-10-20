"""Draws the virtual-item icons (src/ALife.Altis/ui/items/*.paa), used in
the phone's Inventory, shops and the inventory screen. Transparent
background (they sit in list rows). Same pure-Python drawing as
make_icons.py; converted with Arma 3 Tools' ImageToPAA:

    python tools/phone_icons/make_items.py "G:/SteamLibrary/steamapps/common/Arma 3 Tools" [item ...]

Add an item: draw it in ITEMS, re-run, and point its CfgItems `icon` at
ui\\items\\<name>_ca.paa.
"""
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))
from make_icons import SIZE, SS, capsule, circle, minus, poly, ring, rotated_rect, rrect, union, write_png  # noqa: E402

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "items")

# Each item: list of (shape, (r, g, b)) layers, painted in order.
def ore(base, shade, fleck, crystal=False):
    """A rough chunk of rock, with flecks (or a crystal) of the mineral."""
    rock = poly([(22, 70), (34, 40), (62, 28), (92, 36), (108, 62), (98, 94), (66, 106), (34, 98)])
    layers = [
        (rock, base),
        (poly([(62, 28), (92, 36), (108, 62), (70, 60)]), shade),                     # facet
        (poly([(22, 70), (70, 60), (66, 106), (34, 98)]), shade),
    ]
    if crystal:
        layers.append((poly([(56, 64), (64, 38), (76, 46), (80, 72), (66, 82)]), fleck))
    else:
        layers.append((union(circle(46, 58, 7), circle(76, 78, 8), circle(84, 50, 5), circle(52, 86, 5)), fleck))
    return layers


def ingot(front, side, top):
    """A metal bar: top face, front face and end."""
    return [
        (poly([(20, 82), (34, 58), (110, 58), (96, 82)]), top),                       # top
        (poly([(20, 82), (96, 82), (100, 98), (16, 98)]), front),                     # front
        (poly([(96, 82), (110, 58), (114, 74), (100, 98)]), side),                    # end
    ]


def barrel(body, band):
    return [
        (rrect(64, 66, 64, 88, 8), body),                                             # drum
        (union(rrect(64, 40, 64, 5, 0), rrect(64, 92, 64, 5, 0)), band),              # ribs
        (rrect(64, 24, 64, 6, 3), band),                                              # rim
    ]


def mound(top, shade):
    """A heap (salt, sand)."""
    heap = lambda x, y: y >= 100 - 46 * max(0.0, 1 - ((x - 64) / 50) ** 2) and y <= 102 and abs(x - 64) <= 50
    return [
        (heap, top),
        (lambda x, y: heap(x, y) and x > 64 + (100 - y) * 0.2, shade),               # shadow side
    ]


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
    # ---- food and drink ----
    "water": [
        (rrect(64, 22, 16, 10, 3), (40, 110, 200)),                                   # cap
        (union(rrect(64, 34, 22, 14, 4), poly([(53, 38), (75, 38), (84, 54), (44, 54)])), (150, 205, 245)),  # neck
        (rrect(64, 80, 40, 66, 10), (150, 205, 245)),                                 # body
        (rrect(64, 78, 40, 20, 0), (40, 110, 200)),                                   # label
    ],
    "coffee": [
        (rrect(64, 34, 56, 12, 5), (60, 40, 30)),                                     # lid
        (poly([(40, 40), (88, 40), (80, 108), (48, 108)]), (235, 225, 210)),          # cup
        (poly([(43, 62), (85, 62), (83, 82), (45, 82)]), (140, 90, 55)),              # sleeve
    ],
    "donut": [
        (minus(circle(64, 66, 42), circle(64, 66, 15)), (205, 150, 85)),              # dough
        (minus(circle(64, 62, 38), circle(64, 64, 17)), (240, 120, 170)),             # icing
        (union(capsule(46, 46, 52, 44, 2.2), capsule(80, 50, 84, 56, 2.2), capsule(50, 80, 46, 74, 2.2),
               capsule(82, 76, 76, 82, 2.2), capsule(64, 36, 70, 38, 2.2)), (255, 245, 160)),  # sprinkles
    ],
    "apple": [
        (capsule(64, 36, 68, 18, 3), (110, 75, 40)),                                  # stalk
        (union(circle(50, 70, 30), circle(78, 70, 30), circle(64, 86, 26)), (210, 40, 45)),
        (poly([(68, 30), (90, 18), (84, 34)]), (70, 160, 60)),                        # leaf
        (circle(48, 60, 7), (240, 120, 120)),                                         # shine
    ],
    "tbacon": [
        (rrect(64, 64, 76, 70, 12), (150, 155, 160)),                                 # tin
        (rrect(64, 64, 70, 40, 4), (200, 80, 60)),                                    # label
        (union(capsule(40, 58, 88, 52, 3.5), capsule(40, 72, 88, 66, 3.5)), (250, 215, 190)),  # bacon strips
    ],
    "redgull": [
        (rrect(64, 24, 34, 8, 3), (170, 175, 180)),                                   # top
        (rrect(64, 70, 40, 84, 6), (30, 60, 150)),                                    # can
        (rrect(64, 70, 40, 18, 0), (200, 205, 210)),                                  # band
        (circle(64, 70, 8), (220, 40, 45)),                                           # logo
    ],
    # ---- tools ----
    "pickaxe": [
        (capsule(44, 106, 80, 38, 5), (140, 95, 55)),                                 # handle
        (poly([(24, 44), (60, 22), (100, 26), (108, 36), (96, 34), (64, 34), (30, 52)]), (150, 160, 170)),  # head
    ],
    "fuelcan": [
        (minus(rrect(56, 30, 40, 16, 5), rrect(56, 30, 28, 6, 2)), (150, 30, 30)),    # handle
        (rrect(88, 30, 14, 14, 3), (60, 60, 60)),                                     # spout
        (rrect(64, 74, 74, 76, 10), (200, 40, 40)),                                   # body
        (union(capsule(40, 52, 88, 96, 3), capsule(88, 52, 40, 96, 3)), (150, 30, 30)),  # pressed X
    ],
    "lockpick": [
        (ring(40, 64, 20, 9), (170, 175, 185)),                                     # ring
        (capsule(58, 64, 108, 64, 4), (170, 175, 185)),                               # shaft
        (union(rrect(98, 72, 6, 14, 1), rrect(86, 70, 6, 10, 1)), (170, 175, 185)),   # teeth
    ],
    "toolkit": [
        (minus(ring(64, 40, 18, 11), lambda x, y: y > 42), (40, 60, 120)),            # handle
        (rrect(64, 78, 96, 60, 10), (60, 100, 200)),                                  # box
        (rrect(64, 66, 96, 6, 0), (40, 70, 150)),                                     # lid line
        (union(rotated_rect(64, 84, 46, 9, 0.785), circle(48, 68, 9), circle(80, 100, 9)), (230, 235, 240)),  # spanner
        (union(circle(48, 68, 4), circle(80, 100, 4)), (60, 100, 200)),
    ],
    "zipties": [
        (union(ring(54, 64, 30, 24), ring(74, 64, 30, 24)), (215, 215, 220)),         # two loops
        (rrect(64, 98, 16, 12, 3), (160, 160, 168)),                                  # lock head
    ],
    "spikestrip": [
        (rrect(64, 76, 108, 16, 4), (60, 60, 65)),                                    # strip
        (union(*[poly([(x - 7, 70), (x + 7, 70), (x, 46)]) for x in (22, 43, 64, 85, 106)]), (200, 205, 215)),  # spikes
        (union(*[rrect(x, 76, 10, 16, 0) for x in (32, 75)]), (230, 190, 40)),        # hazard bands
    ],
    # ---- police placeables ----
    "cone": [
        (rrect(64, 104, 72, 10, 3), (40, 40, 45)),                                    # base
        (poly([(64, 18), (46, 98), (82, 98)]), (245, 120, 30)),                       # cone
        (union(poly([(59, 42), (69, 42), (72, 56), (56, 56)]), poly([(53, 70), (75, 70), (78, 84), (50, 84)])), (245, 245, 245)),  # bands
        (circle(64, 18, 6), (255, 220, 80)),                                          # light
    ],
    "barrier": [
        (union(rrect(34, 92, 8, 34, 2), rrect(94, 92, 8, 34, 2)), (70, 70, 75)),      # legs
        (rrect(64, 62, 104, 30, 5), (245, 245, 245)),                                 # board
        (union(*[poly([(x, 47), (x + 14, 47), (x + 2, 77), (x - 12, 77)]) for x in (22, 50, 78, 106)]), (220, 50, 55)),  # stripes
    ],
    "roadlight": [
        (union(capsule(64, 60, 64, 108, 4), capsule(64, 108, 40, 118, 3), capsule(64, 108, 88, 118, 3)), (70, 70, 75)),  # stand
        (rrect(64, 40, 56, 34, 6), (60, 64, 72)),                                     # lamp head
        (rrect(64, 40, 46, 24, 4), (255, 235, 150)),                                  # lens
    ],
    "arrowsign": [
        (union(capsule(40, 92, 34, 116, 4), capsule(88, 92, 94, 116, 4)), (70, 70, 75)),  # legs
        (rrect(64, 62, 104, 62, 8), (25, 25, 30)),                                    # board
        (poly([(30, 62), (60, 40), (60, 54), (96, 54), (96, 70), (60, 70), (60, 84)]), (250, 180, 30)),  # arrow
    ],
    # ---- medical ----
    "firstaid": [
        (rrect(64, 70, 84, 64, 12), (46, 160, 90)),                                   # pouch
        (rrect(64, 42, 84, 10, 4), (32, 125, 70)),                                    # flap
        (union(rrect(64, 72, 14, 40, 3), rrect(64, 72, 40, 14, 3)), (255, 255, 255)), # cross
    ],
    "defib": [
        (rrect(64, 72, 92, 68, 12), (60, 64, 72)),                                    # case
        (rrect(64, 50, 92, 14, 6), (220, 50, 55)),                                    # red band
        (poly([(70, 60), (50, 82), (62, 82), (56, 100), (78, 74), (66, 74), (74, 60)]), (250, 204, 21)),  # bolt
        (union(circle(30, 104, 9), circle(98, 104, 9)), (200, 205, 215)),             # paddles
    ],
    # ---- resources: raw (ore, crude) and processed ----
    "oil_raw": barrel((45, 45, 50), (25, 25, 28)) + [
        (union(circle(64, 82, 13), poly([(64, 52), (52, 78), (76, 78)])), (12, 12, 14)),  # oil drop
    ],
    "oil_refined": barrel((230, 170, 40), (190, 130, 20)) + [
        (union(circle(64, 82, 13), poly([(64, 52), (52, 78), (76, 78)])), (255, 235, 170)),
    ],
    "copper_ore": ore((105, 92, 84), (80, 70, 64), (205, 115, 60)),
    "copper_bar": ingot((215, 125, 65), (175, 95, 45), (240, 165, 105)),
    "iron_ore": ore((98, 94, 94), (72, 70, 70), (160, 85, 60)),
    "iron_bar": ingot((150, 155, 165), (115, 120, 130), (195, 200, 210)),
    "salt_raw": ore((200, 200, 206), (165, 165, 172), (235, 235, 240)),
    "salt": mound((245, 245, 248), (215, 215, 222)),
    "sand": mound((222, 190, 120), (190, 160, 95)),
    "glass": [
        (rrect(64, 64, 76, 92, 6), (150, 215, 235)),                                  # pane
        (minus(rrect(64, 64, 76, 92, 6), rrect(64, 64, 68, 84, 4)), (95, 165, 195)),  # frame edge
        (union(capsule(42, 56, 62, 36, 3), capsule(46, 76, 76, 46, 3)), (235, 250, 255)),  # glints
    ],
    "diamond_raw": ore((110, 110, 115), (82, 82, 88), (150, 230, 250), crystal=True),
    "diamond": [
        (poly([(30, 50), (46, 30), (82, 30), (98, 50), (64, 104)]), (120, 215, 245)),  # gem
        (poly([(30, 50), (98, 50), (64, 104)]), (90, 185, 230)),                      # pavilion
        (union(capsule(46, 30, 54, 50, 1.6), capsule(82, 30, 74, 50, 1.6), capsule(54, 50, 64, 104, 1.4),
               capsule(74, 50, 64, 104, 1.4), capsule(30, 50, 98, 50, 1.6)), (220, 245, 255)),  # facets
    ],
    "rock": ore((140, 140, 145), (105, 105, 110), (165, 165, 170)),
    "cement": [
        (poly([(30, 34), (98, 34), (104, 104), (24, 104)]), (175, 172, 165)),         # bag
        (rrect(64, 34, 64, 8, 3), (145, 142, 135)),                                   # folded top
        (rrect(64, 72, 60, 24, 3), (70, 75, 85)),                                     # label
        (rrect(64, 72, 40, 6, 2), (220, 220, 215)),
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
    only = set(sys.argv[2:])  # optional: just these items
    for name, layers in ITEMS.items():
        if only and name not in only:
            continue
        png = os.path.join(png_dir, f"item_{name}_ca.png")
        write_png(png, render(layers))
        print("drew", png)
        if tools:
            subprocess.run([os.path.join(tools, "ImageToPAA", "ImageToPAA.exe"), png,
                            os.path.join(OUT, f"{name}_ca.paa")], check=True)


if __name__ == "__main__":
    main()
