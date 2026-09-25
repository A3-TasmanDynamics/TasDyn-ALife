#!/usr/bin/env python3
"""Search the Arma 3 command index.

Usage:
  search_arma3.py <query>          — full-text search
  search_arma3.py --exact <name>   — exact command lookup
"""

import sqlite3
import sys
from pathlib import Path

DB_PATH = Path(__file__).parent / "arma3.db"
SEP = "=" * 64


def connect():
    if not DB_PATH.exists():
        print(f"Database not found: {DB_PATH}")
        print("Run index_arma3.py first.")
        sys.exit(1)
    return sqlite3.connect(DB_PATH)


def lookup(name: str):
    con = connect()
    row = con.execute(
        "SELECT title,grp,since,games,descr,syntax,params,returns,examples,seealso "
        "FROM commands WHERE title = ? COLLATE NOCASE",
        (name,)
    ).fetchone()
    con.close()
    return row


def search(query: str, limit: int = 12):
    con = connect()
    try:
        rows = con.execute(
            """
            SELECT c.title, c.grp, c.since, c.descr, c.syntax, c.returns
            FROM commands_fts fts
            JOIN commands c ON c.id = fts.rowid
            WHERE commands_fts MATCH ?
            ORDER BY rank
            LIMIT ?
            """,
            (query, limit),
        ).fetchall()
    except sqlite3.OperationalError:
        rows = con.execute(
            """
            SELECT title, grp, since, descr, syntax, returns
            FROM commands
            WHERE title LIKE ? OR descr LIKE ?
            LIMIT ?
            """,
            (f'%{query}%', f'%{query}%', limit),
        ).fetchall()
    con.close()
    return rows


def print_full(row):
    title, grp, since, games, descr, syntax, params, returns, examples, seealso = row
    print(f"\n{SEP}")
    print(f"  {title}  [{grp}]  introduced: {since}  games: {games}")
    print(SEP)
    if syntax:
        print(f"\nSyntax:\n  {syntax}")
    if descr:
        print(f"\nDescription:\n  {descr}")
    if params:
        print(f"\nParameters:")
        for line in params.split('\n'):
            print(f"  {line}")
    if returns:
        print(f"\nReturns:\n  {returns}")
    if examples:
        print(f"\nExamples:")
        for line in examples.split('\n')[:6]:
            print(f"  {line}")
    if seealso:
        print(f"\nSee also: {seealso}")
    print()


def print_results(rows):
    if not rows:
        print("No results.")
        return
    for title, grp, since, descr, syntax, returns in rows:
        print(f"\n{SEP}")
        print(f"  {title}  [{grp}]  (since {since})")
        if syntax:
            print(f"  Syntax:  {syntax[:120]}")
        if descr:
            print(f"  Descr:   {descr[:200]}")
        if returns:
            print(f"  Returns: {returns[:80]}")
    print()


if __name__ == '__main__':
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        sys.exit(0)

    if args[0] == '--exact' and len(args) >= 2:
        row = lookup(' '.join(args[1:]))
        if row:
            print_full(row)
        else:
            print("Command not found.")
    else:
        print_results(search(' '.join(args)))
