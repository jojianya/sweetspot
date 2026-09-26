#!/usr/bin/env python3
"""
Tick the standard sign-off checkboxes on a filed review issue and append a note
about where the fix landed. Leaves the issue open — the work is only done once
the PR merges.

Usage:  python3 scripts/review-issues/mark-done.py 1 2 --note "Implemented in #44."
"""

import argparse
import pathlib
import subprocess
import sys

REPO = "jojianya/sweetspot"

SIGNOFF = (
    "Verified locally",
    "Regression test added",
    "Lint + typecheck still clean",
)


def gh(*args: str) -> str:
    return subprocess.run(
        ["gh", *args], capture_output=True, text=True, check=True
    ).stdout


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("issues", nargs="+", type=int)
    ap.add_argument("--note", default="")
    ap.add_argument("--undo", action="store_true", help="clear the sign-off boxes")
    args = ap.parse_args()

    for num in args.issues:
        body = gh("issue", "view", str(num), "--repo", REPO, "--json", "body", "--jq", ".body")
        lines = body.rstrip("\n").split("\n")

        out = []
        for line in lines:
            if any(line.startswith(f"- [ ] {s}") for s in SIGNOFF):
                mark = "[ ]" if args.undo else "[x]"
                line = line.replace("- [ ] ", f"- {mark} ", 1)
            out.append(line)
        text = "\n".join(out)

        if args.note and args.note not in text:
            text += f"\n\n---\n\n{args.note}"

        gh("issue", "edit", str(num), "--repo", REPO, "--body", text)
        ticked = text.count("- [x] ")
        print(f"  #{num}: {ticked} ticked, {text.count('- [ ] ')} remaining")


if __name__ == "__main__":
    main()
