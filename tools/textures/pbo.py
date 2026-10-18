"""Read files out of Arma 3 PBO archives (pure Python, read-only).

    entries(path)              -> [(name, method, original_size, data_size, offset)]
    read(path, name)           -> bytes of one file (LZSS-compressed entries unpacked)
    prefix(path)               -> the PBO's "prefix" (its virtual path, e.g. a3\\soft_f)
"""
import struct


def _cstr(f):
    b = bytearray()
    while (c := f.read(1)) not in (b"\x00", b""):
        b += c
    return b.decode("latin-1")


def _header(f):
    entries, props = [], {}
    while True:
        name = _cstr(f)
        method, orig, _res, _ts, size = struct.unpack("<5I", f.read(20))
        if name == "" and method == 0x56657273:  # 'Vers': header extension (key/value pairs)
            while True:
                k = _cstr(f)
                if k == "":
                    break
                props[k] = _cstr(f)
            continue
        if name == "":
            break
        entries.append([name, method, orig, size])
    offset = f.tell()
    out = []
    for name, method, orig, size in entries:
        out.append((name, method, orig, size, offset))
        offset += size
    return out, props


def entries(path):
    with open(path, "rb") as f:
        return _header(f)[0]


def prefix(path):
    with open(path, "rb") as f:
        return _header(f)[1].get("prefix", "")


def lzss(src, outlen):
    out = bytearray()
    i = 0
    while len(out) < outlen and i < len(src):
        flags = src[i]
        i += 1
        for bit in range(8):
            if len(out) >= outlen or i >= len(src):
                break
            if flags & (1 << bit):
                out.append(src[i])
                i += 1
            else:
                b1, b2 = src[i], src[i + 1]
                i += 2
                rpos = len(out) - (b1 | ((b2 & 0xF0) << 4))
                for k in range((b2 & 0x0F) + 3):
                    out.append(out[rpos + k] if rpos + k >= 0 else 0x20)
    return bytes(out)


def read(path, name):
    with open(path, "rb") as f:
        ents, _ = _header(f)
        for n, method, orig, size, offset in ents:
            if n.lower() == name.lower():
                f.seek(offset)
                raw = f.read(size)
                compressed = method == 0x43707273 or (orig and orig != size)
                return lzss(raw, orig) if compressed else raw
    raise KeyError(name)
