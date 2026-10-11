#!/usr/bin/env python3
"""Resolve every relative markdown link in tracked .md files against the filesystem."""
import os
import re
import sys
from pathlib import Path

ROOT = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()

# markdown link target inside (...) for [text](target); also autolinks
LINK = re.compile(r"\[[^\]]*\]\(\s*<?([^)\s>]+)>?[^)]*\)")
INLINE = re.compile(r"<((?:https?://|#|mailto:)[^>\s]*)>")

def tracked_md_files():
    out = []
    for p in ROOT.rglob("*.md"):
        if any(part in (".git", "node_modules", ".next") for part in p.parts):
            continue
        out.append(p)
    return sorted(out)

broken = []
total = 0
for path in tracked_md_files():
    try:
        text = path.read_text(encoding="utf-8")
    except Exception as exc:
        print(f"UNREADABLE {path.relative_to(ROOT)}: {exc}", file=sys.stderr)
        continue
    targets = LINK.findall(text) + INLINE.findall(text)
    for raw in targets:
        target = raw.strip()
        if not target or target.startswith(("http://", "https://", "mailto:", "#")):
            continue
        # strip an anchor
        filepart = target.split("#", 1)[0]
        if not filepart:
            continue
        total += 1
        resolved = (path.parent / filepart).resolve()
        if not resolved.exists():
            broken.append((path.relative_to(ROOT).as_posix(), target))

print(f"checked {total} relative markdown links in {len(tracked_md_files())} files")
if broken:
    print(f"BROKEN LINKS: {len(broken)}")
    for src, tgt in broken:
        print(f"  {src}: {tgt}")
else:
    print("BROKEN LINKS: 0")
