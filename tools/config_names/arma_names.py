"""Class -> English display name, for CfgVehicles / CfgWeapons / CfgMagazines /
CfgGlasses, read straight from the game's PBOs (rapified config.bin, plus
stringtable.xml for $STR_ keys) -- vanilla Arma 3 and its DLC, no mods.
Writes the cache annotate.py uses (takes a minute or two):

    python tools/config_names/arma_names.py tools/config_names/names.json ["G:/SteamLibrary/steamapps/common/Arma 3" ...]
"""
import io
import json
import os
import re
import struct
import sys
import xml.etree.ElementTree as ET

ROOTS = sys.argv[2:] or [r"G:\SteamLibrary\steamapps\common\Arma 3", r"G:\SteamLibrary\steamapps\common\Arma 3 Server"]
WANT = {"CfgVehicles", "CfgWeapons", "CfgMagazines", "CfgGlasses"}


def cstr(f):
    b = bytearray()
    while (c := f.read(1)) not in (b"\x00", b""):
        b += c
    return b.decode("latin-1")


def lzss(src, outlen):
    out = bytearray(); i = 0
    while len(out) < outlen and i < len(src):
        flags = src[i]; i += 1
        for bit in range(8):
            if len(out) >= outlen or i >= len(src):
                break
            if flags & (1 << bit):
                out.append(src[i]); i += 1
            else:
                b1, b2 = src[i], src[i + 1]; i += 2
                rpos = len(out) - ((b1 | ((b2 & 0xF0) << 4)))
                n = (b2 & 0x0F) + 3
                for k in range(n):
                    out.append(out[rpos + k] if rpos + k >= 0 else 0x20)
    return bytes(out)


def pbo_files(path, want):
    """{name: bytes} for entries whose lowercase name ends with one of `want`."""
    out = {}
    with open(path, "rb") as f:
        entries = []
        while True:
            name = cstr(f)
            method, orig, _r, _t, size = struct.unpack("<5I", f.read(20))
            if name == "" and method == 0x56657273:
                while cstr(f) != "":
                    pass
                continue
            if name == "":
                break
            entries.append((name, method, orig, size))
        off = f.tell()
        for name, method, orig, size in entries:
            if name.lower().endswith(want):
                f.seek(off)
                data = f.read(size)
                if method == 0x43707273 or (orig and orig != size):
                    data = lzss(data, orig)
                out[name] = data
            off += size
    return out


def cint(f):
    v = 0; shift = 0
    while True:
        b = f.read(1)[0]
        v |= (b & 0x7F) << shift
        if not b & 0x80:
            return v
        shift += 7


def read_class(data, offset, depth, path, found):
    f = io.BytesIO(data); f.seek(offset)
    parent = cstr(f)
    n = cint(f)
    entries = {}
    subclasses = []
    for _ in range(n):
        t = f.read(1)[0]
        if t == 0:
            name = cstr(f); off = struct.unpack("<I", f.read(4))[0]
            subclasses.append((name, off))
        elif t == 1:
            sub = f.read(1)[0]; name = cstr(f)
            if sub == 0:
                val = cstr(f)
            elif sub == 1:
                val = struct.unpack("<f", f.read(4))[0]
            elif sub == 2:
                val = struct.unpack("<i", f.read(4))[0]
            else:
                val = cstr(f)
            entries[name.lower()] = val
        elif t in (2, 5):
            if t == 5:
                f.read(4)
            name = cstr(f); skip_array(f)
        elif t in (3, 4):
            cstr(f)
        else:
            return parent, entries
    if depth == 1:
        for name, off in subclasses:
            if name in WANT:
                read_class(data, off, 2, name, found)
    elif depth == 2:
        for name, off in subclasses:
            p, e = read_class(data, off, 3, None, None)
            d = found.setdefault(path, {})
            if name.lower() not in d or ("displayname" in e and d[name.lower()][1] is None):
                d[name.lower()] = (p.lower(), e.get("displayname"), e.get("scope"))
    return parent, entries


def skip_array(f):
    n = cint(f)
    for _ in range(n):
        t = f.read(1)[0]
        if t in (0, 4):
            cstr(f)
        elif t in (1, 2):
            f.read(4)
        elif t == 3:
            skip_array(f)


def parse_config(data, found):
    if data[:4] != b"\x00raP":
        return
    f = io.BytesIO(data); f.seek(16)
    read_class(data, 16, 0, None, None)  # root: walk its classes
    f.seek(16)
    cstr(f)
    n = cint(f)
    for _ in range(n):
        t = f.read(1)[0]
        if t == 0:
            name = cstr(f); off = struct.unpack("<I", f.read(4))[0]
            if name in WANT:
                read_class(data, off, 2, name, found)
        elif t == 1:
            sub = f.read(1)[0]; cstr(f)
            if sub in (0, 3):
                cstr(f)
            else:
                f.read(4)
        elif t in (2, 5):
            if t == 5:
                f.read(4)
            cstr(f); skip_array(f)
        elif t in (3, 4):
            cstr(f)


def main():
    found, strings = {}, {}
    pbos = []
    for root in ROOTS:
        for dp, _dn, fn in os.walk(root):
            low = dp.lower()
            if "!workshop" in low or "\@" in low or os.sep + "@" in low or "keys" in low:
                continue  # vanilla and its DLC only: no mods
            pbos += [os.path.join(dp, x) for x in fn if x.lower().endswith(".pbo")]
    for p in pbos:
        try:
            files = pbo_files(p, ("config.bin", "stringtable.xml"))
        except Exception:
            continue
        for name, data in files.items():
            if name.lower().endswith("config.bin"):
                try:
                    parse_config(data, found)
                except Exception:
                    pass
            else:
                try:
                    tree = ET.fromstring(data)
                    for key in tree.iter("Key"):
                        eng = key.find("English") or key.find("Original")
                        if eng is not None and eng.text:
                            strings[key.get("ID").lower()] = eng.text
                except Exception:
                    pass

    def resolve(cfg, cls, seen=0):
        e = found.get(cfg, {}).get(cls)
        if not e or seen > 30:
            return None
        parent, dn, _scope = e
        if dn:
            if isinstance(dn, str) and dn.startswith("$"):
                return strings.get(dn[1:].lower(), dn)
            return dn
        return resolve(cfg, parent, seen + 1) if parent else None

    out = {}
    for cfg, classes in found.items():
        for cls in classes:
            name = resolve(cfg, cls)
            if name:
                out.setdefault(cls, name)
    json.dump(out, open(sys.argv[1], "w", encoding="utf-8"), ensure_ascii=False)
    print(len(pbos), "pbos;", len(out), "names;", len(strings), "strings")


if __name__ == "__main__":
    main()
