#!/usr/bin/env python3
"""Parse arma3Documentation.xml and build a searchable SQLite index."""

import re
import sqlite3
import xml.etree.ElementTree as ET
from pathlib import Path

XML_PATH = Path(__file__).parent / "arma3Documentation.xml"
DB_PATH  = Path(__file__).parent / "arma3.db"
MW_NS    = "http://www.mediawiki.org/xml/export-0.11/"


# ---------------------------------------------------------------------------
# Wikitext helpers
# ---------------------------------------------------------------------------

def strip_wiki(text: str) -> str:
    """Strip common wikitext markup to plain text."""
    # [[link|label]] → label,  [[link]] → link
    text = re.sub(r'\[\[(?:[^|\]]*\|)?([^\]]*)\]\]', r'\1', text)
    # Remove {{templates}} (up to 4 levels of nesting)
    for _ in range(4):
        text = re.sub(r'\{\{[^{}]*\}\}', '', text)
    # Strip HTML/XML tags
    text = re.sub(r'<[^>]+>', '', text)
    # Strip bold/italic markers
    text = re.sub(r"'{2,3}", '', text)
    # Collapse whitespace
    return re.sub(r'\s+', ' ', text).strip()


def parse_rv(wikitext: str) -> dict | None:
    """Extract fields from the first {{RV|...}} block in wikitext."""
    m = re.search(r'\{\{RV\|(.*?)\}\}(?!\})', wikitext, re.DOTALL)
    if not m:
        return None
    fields: dict[str, str] = {}
    for chunk in re.split(r'\n\|', '\n' + m.group(1)):
        if '=' not in chunk:
            continue
        key, _, val = chunk.partition('=')
        key = key.strip()
        if key:
            fields[key] = val.strip()
    return fields


def collect(fields: dict, prefix: str) -> str:
    """Join all fields matching prefix+digit (e.g. p1, p2 …) into one string."""
    parts, i = [], 1
    while f'{prefix}{i}' in fields:
        parts.append(fields[f'{prefix}{i}'])
        i += 1
    return '\n'.join(parts)


def games_from(fields: dict) -> list[str]:
    games, i = [], 1
    while f'game{i}' in fields:
        games.append(fields[f'game{i}'])
        i += 1
    return games


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def build():
    print(f"Indexing {XML_PATH} …")
    con = sqlite3.connect(DB_PATH)
    cur = con.cursor()

    cur.executescript("""
        DROP TABLE IF EXISTS commands_fts;
        DROP TABLE IF EXISTS commands;

        CREATE TABLE commands (
            id       INTEGER PRIMARY KEY AUTOINCREMENT,
            title    TEXT NOT NULL,
            type     TEXT,
            games    TEXT,
            grp      TEXT,
            since    TEXT,
            descr    TEXT,
            syntax   TEXT,
            params   TEXT,
            returns  TEXT,
            examples TEXT,
            seealso  TEXT
        );

        CREATE VIRTUAL TABLE commands_fts USING fts5(
            title,
            descr,
            syntax,
            params,
            returns,
            examples,
            seealso,
            content='commands',
            content_rowid='id'
        );
    """)

    batch: list[tuple] = []
    indexed = skipped = 0
    tag_page = f'{{{MW_NS}}}page'
    tag_ns   = f'{{{MW_NS}}}ns'
    tag_title = f'{{{MW_NS}}}title'
    tag_text  = f'{{{MW_NS}}}text'

    for _, elem in ET.iterparse(XML_PATH, events=('end',)):
        if elem.tag != tag_page:
            continue

        ns_el = elem.find(tag_ns)
        if ns_el is None or ns_el.text != '0':
            elem.clear()
            continue

        title   = (elem.findtext(tag_title) or '').strip()
        text_el = elem.find('.//' + tag_text)
        wikitext = (text_el.text or '') if text_el is not None else ''

        if not wikitext:
            elem.clear()
            continue

        fields = parse_rv(wikitext)
        if not fields:
            skipped += 1
            elem.clear()
            continue

        cmd_type = fields.get('type', '')
        if cmd_type not in ('command', 'function'):
            skipped += 1
            elem.clear()
            continue

        games = games_from(fields)
        # Skip pages that are purely for other games (DayZ, Ylands, Reforger only)
        if games and not any(g.startswith('arma') or g == 'ofp' or g == 'tkoh' for g in games):
            skipped += 1
            elem.clear()
            continue

        batch.append((
            title,
            cmd_type,
            ','.join(games),
            fields.get('gr1', ''),
            fields.get('version1', ''),
            strip_wiki(fields.get('descr', '')),
            strip_wiki(collect(fields, 's')),
            strip_wiki(collect(fields, 'p')),
            strip_wiki(fields.get('r1', '')),
            strip_wiki(collect(fields, 'x')),
            strip_wiki(fields.get('seealso', '')),
        ))

        if len(batch) % 500 == 0:
            print(f"  {len(batch)} commands …")

        indexed += 1
        elem.clear()

    cur.executemany("""
        INSERT INTO commands
          (title, type, games, grp, since, descr, syntax, params, returns, examples, seealso)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)
    """, batch)

    cur.execute("""
        INSERT INTO commands_fts (rowid, title, descr, syntax, params, returns, examples, seealso)
        SELECT id, title, descr, syntax, params, returns, examples, seealso FROM commands
    """)

    con.commit()
    con.close()

    print(f"\nDone — {indexed} commands indexed, {skipped} pages skipped.")
    print(f"Database: {DB_PATH}")


if __name__ == '__main__':
    build()
