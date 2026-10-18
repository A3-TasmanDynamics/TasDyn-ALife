"""Placeholder police / EMS liveries and uniforms: Arma's own textures,
recoloured into the ALife palette (navy for police, red for EMS vehicles,
white for EMS uniforms), halved to 1024 px, written as PAA into the
mission. Stand-ins until the real PAAs are dropped in at the same paths.

    python tools/textures/make_placeholders.py

Needs Arma 3 and Arma 3 Tools (ImageToPAA) at the paths below. The recolour
keeps each texture's shading (it maps brightness onto the new colour);
decals, text and glass get recoloured too -- it's a placeholder.
"""
import os
import struct
import subprocess
import sys
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import pbo  # noqa: E402

ARMA = r"G:\SteamLibrary\steamapps\common\Arma 3"
IMAGETOPAA = r"G:\SteamLibrary\steamapps\common\Arma 3 Tools\ImageToPAA\ImageToPAA.exe"
MISSION = os.path.join(HERE, "..", "..", "src", "ALife.Altis")
WORK = os.path.join(os.environ.get("TEMP", HERE), "alife_tex")  # short: the tools dislike long paths

# Colour schemes: brightness 0..1 maps from `dark` to `light`.
SCHEMES = {
    "police": ((8, 12, 24), (150, 172, 210)),       # navy
    "ems": ((48, 6, 8), (238, 72, 72)),             # red
    "ems_uniform": ((70, 72, 78), (246, 246, 248)),  # white
}

# (output under the mission, source PBO, path inside the PBO, scheme)
JOBS = [
    ("textures/vehicles/police_offroad.paa", r"Addons\soft_f.pbo", r"Offroad_01\data\Offroad_01_ext_co.paa", "police"),
    ("textures/vehicles/police_suv.paa", r"Addons\soft_f_gamma.pbo", r"SUV_01\Data\SUV_01_ext_CO.paa", "police"),
    ("textures/vehicles/police_hatchback_sport.paa", r"Addons\soft_f_gamma.pbo", r"Hatchback_01\data\Hatchback_01_ext_SPORT01_CO.paa", "police"),
    ("textures/vehicles/police_hummingbird.paa", r"Addons\air_f.pbo", r"Heli_Light_01\Data\Heli_Light_01_ext_Blufor_CO.paa", "police"),
    ("textures/vehicles/ems_offroad.paa", r"Addons\soft_f.pbo", r"Offroad_01\data\Offroad_01_ext_co.paa", "ems"),
    ("textures/vehicles/ems_suv.paa", r"Addons\soft_f_gamma.pbo", r"SUV_01\Data\SUV_01_ext_CO.paa", "ems"),
    ("textures/vehicles/ems_m900.paa", r"Addons\air_f.pbo", r"Heli_Light_01\Data\Heli_Light_01_ext_co.paa", "ems"),
    ("textures/vehicles/ems_orca.paa", r"Addons\air_f.pbo", r"Heli_Light_02\Data\Heli_Light_02_ext_CO.paa", "ems"),
    ("textures/uniforms/cop_uniform.paa", r"Addons\characters_f.pbo", r"Civil\Data\c_poloshirtpants_1_co.paa", "police"),
    ("textures/uniforms/medic_uniform.paa", r"Addons\characters_f.pbo", r"Civil\Data\c_poloshirtpants_1_co.paa", "ems_uniform"),
]


def read_png(path):
    d = open(path, "rb").read()
    w, h, bd, ct = struct.unpack(">IIBB", d[16:26])
    assert bd == 8 and ct in (2, 6), (path, bd, ct)
    idat, i = b"", 8
    while i < len(d):
        n, = struct.unpack(">I", d[i:i + 4])
        if d[i + 4:i + 8] == b"IDAT":
            idat += d[i + 8:i + 8 + n]
        i += 12 + n
    raw = zlib.decompress(idat)
    bpp = 4 if ct == 6 else 3
    stride = w * bpp
    rows, prev, p = [], bytearray(stride), 0
    for _ in range(h):
        f = raw[p]
        line = bytearray(raw[p + 1:p + 1 + stride])
        p += 1 + stride
        if f == 1:
            for x in range(bpp, stride):
                line[x] = (line[x] + line[x - bpp]) & 255
        elif f == 2:
            for x in range(stride):
                line[x] = (line[x] + prev[x]) & 255
        elif f == 3:
            for x in range(stride):
                a = line[x - bpp] if x >= bpp else 0
                line[x] = (line[x] + ((a + prev[x]) >> 1)) & 255
        elif f == 4:
            for x in range(stride):
                a = line[x - bpp] if x >= bpp else 0
                b = prev[x]
                c = prev[x - bpp] if x >= bpp else 0
                pa, pb, pc = abs(b - c), abs(a - c), abs(a + b - 2 * c)
                line[x] = (line[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 255
        rows.append(line)
        prev = line
    return w, h, bpp, rows


def write_png(path, w, h, rows):
    """RGBA rows (bytearray of w*4) -> PNG."""
    raw = b"".join(b"\x00" + bytes(r) for r in rows)

    def chunk(t, data):
        return struct.pack(">I", len(data)) + t + data + struct.pack(">I", zlib.crc32(t + data) & 0xffffffff)
    open(path, "wb").write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 6, 0, 0, 0))
                           + chunk(b"IDAT", zlib.compress(raw, 6)) + chunk(b"IEND", b""))


def recolour(w, h, bpp, rows, scheme):
    dark, light = SCHEMES[scheme]
    # One lookup per brightness level (0..255).
    lut = [tuple(int(dark[k] + (light[k] - dark[k]) * ((v / 255) ** 0.9)) for k in range(3)) for v in range(256)]
    out = []
    for y in range(0, h, 2):  # halve: average each 2x2 block's brightness
        r0, r1 = rows[y], rows[y + 1]
        line = bytearray()
        for x in range(0, w * bpp, 2 * bpp):
            lum = 0
            for r in (r0, r1):
                for o in (x, x + bpp):
                    lum += 77 * r[o] + 150 * r[o + 1] + 29 * r[o + 2]
            c = lut[lum >> 10]  # /4 pixels, /256 weights
            alpha = r0[x + 3] if bpp == 4 else 255
            line += bytes((c[0], c[1], c[2], alpha))
        out.append(line)
    return w // 2, h // 2, out


def main():
    os.makedirs(WORK, exist_ok=True)
    for dest, pack, inner, scheme in JOBS:
        name = os.path.splitext(os.path.basename(dest))[0]
        src_paa = os.path.join(WORK, name + "_src.paa")
        src_png = os.path.join(WORK, name + "_src.png")
        new_png = os.path.join(WORK, name + ".png")
        out = os.path.normpath(os.path.join(MISSION, dest))
        os.makedirs(os.path.dirname(out), exist_ok=True)
        open(src_paa, "wb").write(pbo.read(os.path.join(ARMA, pack), inner))
        subprocess.run([IMAGETOPAA, src_paa, src_png], check=True, capture_output=True)
        w, h, rows = recolour(*read_png(src_png), scheme)
        write_png(new_png, w, h, rows)
        subprocess.run([IMAGETOPAA, new_png, out], check=True, capture_output=True)
        print("wrote", dest, f"({w}x{h}, {scheme})")


if __name__ == "__main__":
    main()
