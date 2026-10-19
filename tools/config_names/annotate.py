"""Adds "// <in-game name>" to each row / class in the mission's shop configs
(CfgVehicles, CfgWeapons, CfgClothing, CfgMarket), so you can tell what a
classname is. Rows that already have a comment naming it are left alone;
re-run after adding vehicles / gear:

    python tools/config_names/arma_names.py tools/config_names/names.json   (once)
    python tools/config_names/annotate.py
"""
import json
import os
import re
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
CFG = os.path.join(HERE, "..", "..", "src", "ALife.Altis", "config")
names = json.load(open(os.path.join(HERE, "names.json"), encoding="utf-8"))

# Virtual items: their names come from the mission's own CfgItems.
items = {}
cur = None
for line in open(os.path.join(CFG, "CfgItems.hpp"), encoding="utf-8"):
    m = re.match(r"\s*class (\w+)\s*$", line)
    if m:
        cur = m.group(1).lower()
    m = re.search(r'displayName = "([^"]*)"', line)
    if m and cur:
        items[cur] = m.group(1)


def ascii_name(s):
    s = s.replace("\u00a0", " ").replace("\u2013", "-").replace("\u2014", "-")
    s = unicodedata.normalize("NFKD", s).encode("ascii", "ignore").decode()
    return re.sub(r"\s+", " ", s).strip()


def name_of(cls, virtual=False):
    n = items.get(cls.lower()) if virtual else names.get(cls.lower())
    return ascii_name(n) if n else None


missing = []


def annotate(path, row_re, virtual=False, only_after=None):
    raw = open(path, encoding="utf-8", newline="").read()
    nl = "\r\n" if "\r\n" in raw else "\n"
    lines = raw.replace("\r\n", "\n").split("\n")
    start = 0
    if only_after:
        start = next(i for i, l in enumerate(lines) if l.startswith(only_after))
    changed = 0
    for i in range(start, len(lines)):
        line = lines[i]
        m = re.match(row_re, line)
        if not m:
            continue
        cls = m.group(1)
        n = name_of(cls, virtual)
        if not n:
            missing.append((os.path.basename(path), cls))
            continue
        code, sep, comment = line.partition("//")
        if sep:
            if n.lower() in comment.lower():
                continue
            lines[i] = code.rstrip() + "  // " + n + " -- " + comment.strip()
        else:
            lines[i] = line.rstrip() + "  // " + n
        changed += 1
    open(path, "w", encoding="utf-8", newline="").write(nl.join(lines))
    print(os.path.basename(path), changed, "annotated")


ROW = r'\s*\{\s*"([A-Za-z0-9_]+)"\s*,'          # Tonic rows: {"Class", ...}
CLASS = r"\s*class ([A-Za-z0-9_]+)\b(?!\s*:)"     # class lines (not inheritance)

annotate(os.path.join(CFG, "CfgVehicles.hpp"), ROW)
annotate(os.path.join(CFG, "CfgVehicles.hpp"), CLASS, only_after="class CfgVehicles")
annotate(os.path.join(CFG, "CfgWeapons.hpp"), ROW)
annotate(os.path.join(CFG, "CfgClothing.hpp"), ROW)
annotate(os.path.join(CFG, "CfgMarket.hpp"), r"\s*class ([a-z0-9_]+)\s*\{", virtual=True, only_after="class CfgMarket")
print("no name found:", sorted(set(missing)))
