"""Draws the website's link-preview image (Open Graph / Discord / Twitter
card): the mission's loading picture (alife_loadscreen.png, from
make_wordmark.py) widened to 1200x630, with a one-line tagline under the
wordmark.

    python tools/branding/make_og.py

Writes src/website/web/static/brand/alife-og.png. Re-run it after
make_wordmark.py changes the loading picture. Needs Pillow; the tagline
uses Segoe UI Semibold (Windows) and falls back to Pillow's own font.
"""
import os

from PIL import Image, ImageDraw, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "alife_loadscreen.png")
OUT = os.path.join(HERE, "..", "..", "src", "website", "web", "static", "brand", "alife-og.png")

W, H = 1200, 630
TAGLINE = "Altis Life for Oceania  \u00b7  No mods  \u00b7  Civilian, Police & EMS"


def font(size):
    for name in ("seguisb.ttf", "segoeuib.ttf", "arialbd.ttf"):
        path = os.path.join(os.environ.get("WINDIR", "C:\\Windows"), "Fonts", name)
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default(size)


def main():
    src = Image.open(SRC).convert("RGB")
    # 1024x512 is 2:1 and the card is 1.9:1: scale to the card's height,
    # then trim the (empty, gradient) sides evenly.
    scale = H / src.height
    big = src.resize((round(src.width * scale), H), Image.LANCZOS)
    left = (big.width - W) // 2
    full = big.crop((left, 0, left + W, H))

    # The wordmark sits mid-card; the tagline goes in the empty space under it.
    d = ImageDraw.Draw(full)
    f = font(34)
    tw = d.textlength(TAGLINE, font=f)
    d.text(((W - tw) / 2, H - 104), TAGLINE, font=f, fill=(203, 213, 225))
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    full.save(OUT, optimize=True)
    print("wrote", os.path.normpath(OUT), full.size, os.path.getsize(OUT), "bytes")


if __name__ == "__main__":
    main()
