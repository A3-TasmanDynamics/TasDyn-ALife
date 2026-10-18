"""Generates src/ALife.Altis/ui/phone.hpp -- the phone dialog (idd 5000).

Run from the repo root:  python tools/phone_icons/make_layout.py

Positions are fractions of the safe zone. Control IDs must match the
#defines in src/ALife.Altis/functions/client/fn_phone.sqf.
"""
import os

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "src", "ALife.Altis", "ui", "phone.hpp")

# Home-screen tiles, in grid order (3 x 3). Index = the app number in fn_phone.sqf.
APPS = [
    ("Messages", "messages"), ("Bank", "bank"), ("Inventory", "inventory"),
    ("Player ID", "playerid"), ("Perks", "perks"), ("Gang", "gang"),
    ("Keys", "keys"), ("Admin", "admin"), ("Settings", "settings"),
]

# Screen area (safe-zone fractions).
SX, SY, SW, SH = 0.752, 0.285, 0.191, 0.66


def pos(x, y, w, h):
    return (f'            x = "safezoneX + safezoneW * {round(x, 4)}";\n'
            f'            y = "safezoneY + safezoneH * {round(y, 4)}";\n'
            f'            w = "safezoneW * {round(w, 4)}";\n'
            f'            h = "safezoneH * {round(h, 4)}";\n')


def ctrl(name, base, body, x, y, w, h):
    return f'        class {name}: {base}\n        {{\n{body}{pos(x, y, w, h)}        }};\n'


NO_SHADOW = ('            colorShadow[] = { 0, 0, 0, 0 };\n'
             '            colorBorder[] = { 0, 0, 0, 0 };\n'
             '            offsetPressedX = 0; offsetPressedY = 0;\n')

LIST_STYLE = ('            sizeEx = 0.028;\n            rowHeight = 0.042;\n'
              '            colorBackground[] = { 1, 1, 1, 0.035 };\n'
              '            colorSelectBackground[] = { 0.96, 0.62, 0.04, 0.9 };\n'
              '            colorSelectBackground2[] = { 0.96, 0.62, 0.04, 0.9 };\n'
              '            colorSelect[] = { 0.118, 0.071, 0, 1 };\n'
              '            colorSelect2[] = { 0.118, 0.071, 0, 1 };\n'
              '            colorPictureSelected[] = { 0.118, 0.071, 0, 1 };\n')

bg, fg = [], []

# The phone itself: one stretched picture (rounded body, bezel, rounded
# screen, camera) drawn by make_frame.py. fn_phone.sqf swaps the home
# wallpaper for the dark app background when an app is open.
bg.append(ctrl("Body", "ALife_RscPicture",
               '            idc = 5001;\n            style = 48; // stretch to fill\n'
               '            text = "ui\\phone\\phone_dark_ca.paa";\n', 0.745, 0.26, 0.205, 0.71))

# Structured text in these controls is sized in whole screen pixels by
# fn_phone.sqf (<t size='N'> on a 1-pixel base) so it renders sharp.
PX_TEXT = ('            size = "pixelH";\n'
           '            class Attributes\n            {\n'
           '                font = "RobotoCondensed";\n                color = "#FFFFFF";\n'
           '                align = "left";\n                shadow = 0;\n            };\n')

# Status bar sits clear of the screen's rounded corners and the camera.
fg.append(ctrl("Status", "ALife_RscStructuredText", '            idc = 5003;\n' + PX_TEXT, 0.765, 0.293, 0.165, 0.026))
# Stacked clock (hours over minutes) and date, top-left like the reference.
fg.append(ctrl("Clock", "ALife_RscStructuredText", '            idc = 5005;\n' + PX_TEXT, 0.768, 0.33, 0.16, 0.2))

# Home tiles: rounded icon (keeps aspect), label under it, transparent button over both.
col_w = SW / 3
for i, (label, icon) in enumerate(APPS):
    col, row = i % 3, i // 3
    cx = SX + col_w * (col + 0.5)
    y = 0.56 + row * 0.125
    fg.append(ctrl(f"TileIcon{i}", "ALife_RscPicture",
                   f'            idc = {5110 + i};\n            text = "ui\\icons\\{icon}_ca.paa";\n',
                   cx - 0.022, y, 0.044, 0.078))
    fg.append(ctrl(f"TileLabel{i}", "ALife_RscText",
                   f'            idc = {5120 + i};\n            style = 2;\n            sizeEx = 0.026;\n'
                   '            colorText[] = { 0.945, 0.961, 0.976, 1 };\n'
                   f'            text = "{label}";\n',
                   cx - col_w / 2, y + 0.082, col_w, 0.028))
    fg.append(ctrl(f"Tile{i}", "ALife_RscButton",
                   f'            idc = {5130 + i};\n'
                   '            colorBackground[] = { 0, 0, 0, 0 };\n'
                   '            colorBackgroundActive[] = { 1, 1, 1, 0.06 };\n'
                   '            colorFocused[] = { 0, 0, 0, 0 };\n'
                   + NO_SHADOW +
                   f'            action = "[\'app\', {i}] call ALife_fnc_phone";\n',
                   cx - col_w / 2 + 0.004, y - 0.006, col_w - 0.008, 0.118))

# App view: header bar, back, title.
fg.append(ctrl("AppHeader", "ALife_RscBackground", '            idc = 5201;\n            colorBackground[] = { 1, 1, 1, 0.045 };\n', SX, 0.32, SW, 0.048))
fg.append(ctrl("Back", "ALife_RscButton",
               '            idc = 5202;\n            text = "<";\n            font = "RobotoCondensedBold";\n            sizeEx = 0.042;\n'
               '            colorBackground[] = { 0, 0, 0, 0 };\n            colorBackgroundActive[] = { 1, 1, 1, 0.1 };\n'
               '            colorFocused[] = { 0, 0, 0, 0 };\n' + NO_SHADOW +
               '            action = "[\'home\'] call ALife_fnc_phone";\n', 0.754, 0.323, 0.024, 0.042))
fg.append(ctrl("AppIcon", "ALife_RscPicture", '            idc = 5204;\n', 0.779, 0.326, 0.02, 0.036))
fg.append(ctrl("AppTitle", "ALife_RscText",
               '            idc = 5203;\n            font = "RobotoCondensedBold";\n            sizeEx = 0.036;\n',
               0.802, 0.323, 0.135, 0.042))

# Card backgrounds (drawn under the content), then the generic content controls.
for n in range(2):
    fg.append(ctrl(f"Card{n}", "ALife_RscBackground",
                   f'            idc = {5230 + n};\n            colorBackground[] = {{ 1, 1, 1, 0.05 }};\n', 0.757, 0.38, 0.181, 0.1))
for n in range(3):
    fg.append(ctrl(f"Label{n}", "ALife_RscText",
                   f'            idc = {5215 + n};\n            font = "RobotoCondensedBold";\n            sizeEx = 0.024;\n'
                   '            colorText[] = { 0.545, 0.596, 0.702, 1 };\n', 0.757, 0.38, 0.181, 0.025))
fg.append(ctrl("Info", "ALife_RscStructuredText", '            idc = 5210;\n' + PX_TEXT, 0.757, 0.38, 0.181, 0.1))
for n in range(2):  # drop-downs (Settings: view distance, wallpaper)
    fg.append(ctrl(f"Combo{n}", "ALife_RscCombo",
                   f'            idc = {5240 + n};\n            sizeEx = 0.03;\n'
                   f'            onLBSelChanged = "[\'select\', {5240 + n}, _this select 1] call ALife_fnc_phone";\n',
                   0.757, 0.42, 0.181, 0.035))
fg.append(ctrl("List", "ALife_RscListNBox",
               '            idc = 5211;\n' + LIST_STYLE + '            columns[] = { 0.04, 0.72 };\n'
               '            onLBSelChanged = "[\'select\', 5211, _this select 1] call ALife_fnc_phone";\n',
               0.757, 0.48, 0.181, 0.2))
fg.append(ctrl("List2", "ALife_RscListNBox",
               '            idc = 5212;\n' + LIST_STYLE + '            columns[] = { 0.04 };\n'
               '            onLBSelChanged = "[\'select\', 5212, _this select 1] call ALife_fnc_phone";\n',
               0.757, 0.69, 0.181, 0.1))
fg.append(ctrl("Edit", "ALife_RscEdit",
               '            idc = 5213;\n            maxChars = 9;\n            sizeEx = 0.032;\n'
               '            colorBackground[] = { 1, 1, 1, 0.06 };\n', 0.757, 0.8, 0.06, 0.035))
fg.append(ctrl("EditMessage", "ALife_RscEdit",
               '            idc = 5214;\n            style = 16;\n            maxChars = 300;\n            sizeEx = 0.028;\n'
               '            colorBackground[] = { 1, 1, 1, 0.06 };\n', 0.757, 0.84, 0.181, 0.06))
for n in range(6):
    fg.append(ctrl(f"Button{n}", "ALife_RscButton",
                   f'            idc = {5220 + n};\n            sizeEx = 0.03;\n'
                   '            colorBackground[] = { 1, 1, 1, 0.09 };\n            colorBackgroundActive[] = { 1, 1, 1, 0.18 };\n'
                   '            colorFocused[] = { 1, 1, 1, 0.12 };\n' + NO_SHADOW +
                   f'            action = "[\'button\', {n}] call ALife_fnc_phone";\n',
                   0.757, 0.9, 0.06, 0.035))

fg.append(ctrl("HomeBar", "ALife_RscButton",
               '            idc = 5004;\n            colorBackground[] = { 1, 1, 1, 0.55 };\n'
               '            colorBackgroundActive[] = { 1, 1, 1, 0.85 };\n            colorFocused[] = { 1, 1, 1, 0.55 };\n'
               + NO_SHADOW + '            action = "[\'home\'] call ALife_fnc_phone";\n',
               0.822, 0.932, 0.051, 0.005))

header = '''// Phone (opened with Y). GENERATED by tools/phone_icons/make_layout.py --
// edit that and re-run it rather than editing this file.
//
// A Galaxy-style phone in the bottom-right: bezel, screen, status bar, a
// clock widget and a 3 x 3 grid of app icons (ui\\icons, drawn by
// tools/phone_icons/make_icons.py) on the home screen, and one app view
// whose generic controls (cards, labels, info text, two lists, inputs,
// buttons) each app arranges for itself in functions\\client\\fn_phone.sqf.

class ALife_Phone
{
    idd = 5000;
    movingEnable = 0;
    enableSimulation = 1;
    onLoad = "['onLoad', _this select 0] call ALife_fnc_phone";

    class controlsBackground
    {
'''

with open(OUT, "w", encoding="utf-8", newline="\r\n") as f:
    f.write(header + "".join(bg) + "    };\n\n    class controls\n    {\n" + "".join(fg) + "    };\n};\n")
print("wrote", os.path.abspath(OUT))
