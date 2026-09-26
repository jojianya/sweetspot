#!/usr/bin/env python3
"""
Generate GitHub issues for the findings in CODE_REVIEW.md.

Writes one markdown body per issue into scripts/review-issues/ plus a
manifest.json describing labels, milestones and ordering. Run create.sh
afterwards to actually file them.

Usage:  python3 scripts/review-issues/generate.py
"""

import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
REPO_ROOT = HERE.parent.parent
SOURCE = REPO_ROOT / "CODE_REVIEW.md"
OUT = HERE / "bodies"
MANIFEST = HERE / "manifest.json"

# Milestones, one per priority tier.
MILESTONES = [
    ("P0-critical", "P0 — critical: fix before any deploy"),
    ("P1-high", "P1 — high: fix this cycle"),
    ("P2-medium", "P2 — medium: scheduled work"),
    ("P3-polish", "P3 — polish: opportunistic"),
]

# Extra labels to create (GitHub's defaults already exist: bug, enhancement,
# documentation, accessibility, question, invalid, duplicate, wontfix,
# good first issue, help wanted).
EXTRA_LABELS = [
    ("P0-critical", "b60205", "Critical: security, data loss, or outage"),
    ("P1-high", "d93f0b", "High: real bug or significant regression risk"),
    ("P2-medium", "fbca04", "Medium: correctness, perf, or robustness gap"),
    ("P3-polish", "0e8a16", "Polish: cleanup, consistency, DX"),
    ("backend", "1d76db", "Go server code"),
    ("frontend", "5319e7", "Next.js / React client code"),
    ("security", "b60205", "Security or auth defect"),
    ("performance", "f9d0c4", "Latency, throughput, or resource usage"),
    ("sql", "c5def5", "Database query or migration"),
    ("devops", "bfd4f2", "Docker, CI, or process"),
    ("needs-design", "d4c5f9", "Needs a decision before implementation"),
]

# Findings to batch into themed P3 issues instead of filing individually.
# Maps issue-id -> the themed issue that should mention it.
BATCHED = {
    "P3.1": "polish-client-perf",
    "P3.2": "polish-client-perf",
    "P3.3": "polish-client-perf",
    "P3.4": "polish-client-perf",
    "P3.5": "polish-client-perf",
    "P3.6": "polish-client-perf",
    "P3.7": "polish-client-perf",
    "P3.16": "polish-client-perf",
    "P3.10": "polish-a11y",
    "P3.8": "polish-a11y",
    "P3.12": "polish-a11y",
    "P3.13": "polish-a11y",
    "P3.17": "polish-tests",
    "P3.18": "polish-tests",
    "P3.19": "polish-deadcode",
    "P3.21": "polish-deadcode",
    "P3.22": "polish-deadcode",
    "P3.23": "polish-deadcode",
    "P3.9": "polish-consistency",
    "P3.11": "polish-consistency",
    "P3.14": "polish-consistency",
    "P3.15": "polish-consistency",
    "P3.20": "polish-consistency",
}

# The themed P3 issues themselves: key -> (title, milestone, labels, intro)
THEMED = {
    "polish-client-perf": (
        "Polish: client-side performance cleanups",
        "P3-polish",
        ["P3-polish", "frontend", "performance"],
        "Client perf items that are individually small but add up. None are "
        "user-visible breakages; they are cleanup that reduces re-render and "
        "network waste.",
    ),
    "polish-a11y": (
        "Polish: accessibility gaps",
        "P3-polish",
        ["P3-polish", "frontend", "accessibility"],
        "Accessibility and keyboard-navigation gaps. `useDialogFocus` already "
        "exists in the codebase, so most of these are \"use the helper that "
        "is already there\" rather than new work.",
    ),
    "polish-tests": (
        "Polish: test coverage and CI hardening",
        "P3-polish",
        ["P3-polish", "backend", "devops"],
        "Coverage gaps and CI improvements. `internal/endpointtest` covers "
        "feature behaviour well; the gap is unit coverage of the security "
        "primitives and race detection on the concurrent code.",
    ),
    "polish-deadcode": (
        "Polish: remove dead scaffolding and unused code",
        "P3-polish",
        ["P3-polish", "backend"],
        "Placeholder files, dead parameters, and empty directories. Safe to "
        "delete — nothing imports them.",
    ),
    "polish-consistency": (
        "Polish: client consistency and partial-failure fixes",
        "P3-polish",
        ["P3-polish", "frontend"],
        "Small mismatches between duplicated logic — client/server validation "
        "limits, unreachable branches, overly broad auth clearing — plus two "
        "places on the profile page that should degrade gracefully instead of "
        "blanking out.",
    ),
}

# Per-issue label overrides. Anything not listed is derived from the id prefix
# plus a default.
EXTRA_LABELS_BY_ID = {
    "P0.1": ["backend", "security", "devops"],
    "P0.2": ["backend", "devops"],
    "P0.3": ["backend", "security", "needs-design"],
    "P0.4": ["backend", "security"],
    "P1.1": ["backend", "performance", "sql"],
    "P1.2": ["backend", "performance", "sql"],
    "P1.3": ["backend", "performance", "sql"],
    "P1.4": ["backend", "performance", "sql"],
    "P1.5": ["frontend"],
    "P1.6": ["frontend"],
    "P1.7": ["frontend"],
    "P1.8": ["frontend", "devops"],
    "P1.9": ["frontend", "security", "needs-design"],
    "P1.10": ["frontend", "backend"],
    "P2.1": ["frontend", "performance"],
    "P2.2": ["frontend", "backend", "performance"],
    "P2.3": ["frontend", "performance", "devops"],
    "P2.4": ["frontend", "performance"],
    "P2.5": ["frontend", "performance"],
    "P2.6": ["frontend"],
    "P2.7": ["frontend", "accessibility"],
    "P2.8": ["frontend", "accessibility"],
    "P2.9": ["frontend"],
    "P2.10": ["backend", "security", "devops"],
    "P2.11": ["backend"],
    "P2.12": ["backend", "devops"],
    "P2.13": ["backend", "performance", "devops"],
    "P2.14": ["backend", "security", "devops"],
    "P2.15": ["backend", "frontend", "devops"],
    "P2.16": ["backend", "security"],
    "P2.17": ["backend", "devops"],
    "P2.18": ["backend", "devops"],
    "P2.19": ["backend", "performance", "needs-design"],
    "P2.20": ["backend"],
    "P2.21": ["backend", "performance", "sql"],
    "P2.22": ["backend", "sql"],
    "P2.23": ["backend"],
    "P2.24": ["backend", "performance"],
}

PRIORITY_LABEL = {
    "P0": "P0-critical",
    "P1": "P1-high",
    "P2": "P2-medium",
    "P3": "P3-polish",
}

SOURCE_LINK = "https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md"


# --------------------------------------------------------------------------
# Parsing
# --------------------------------------------------------------------------

def parse_sections(text: str) -> list[dict]:
    """Extract every `### Pn.n ...` section with its body and id-tagged tasks."""
    lines = text.splitlines()
    # Find the start of the findings region (skip preamble / how-to-use).
    try:
        start = next(i for i, l in enumerate(lines) if l.startswith("## P0 "))
    except StopIteration:
        sys.exit("could not find the P0 heading in CODE_REVIEW.md")

    # The findings run from the P0 heading until the first NON-priority
    # top-level section (e.g. "## Verified clean"). Take the last `## ` line so
    # we span P0..P3 rather than stopping at P1.
    ends = [
        i for i, l in enumerate(lines)
        if l.startswith("## ") and i > start and not re.match(r"^## P\d ", l)
    ]
    region = lines[start:ends[0]] if ends else lines[start:]

    sections: list[dict] = []
    cur: dict | None = None
    fence: str | None = None
    tasks: list[str] = []
    extra: list[str] = []
    intro: list[str] = []
    absorbing = False

    def flush():
        nonlocal cur, tasks, extra, intro, absorbing
        if cur is not None:
            cur["tasks"] = tasks
            cur["extra"] = extra
            cur["mainTask"] = next(
                (re.match(r"^- \[ \] \*\*(.+?)\*\*", t).group(1).strip()
                 for t in tasks if re.match(r"^- \[ \] \*\*(.+?)\*\*", t)),
                None,
            )
            cur["intro"] = "\n".join(intro).strip()
            sections.append(cur)
        tasks, extra, intro, absorbing = [], [], [], False

    for raw in region:
        stripped = raw.strip()
        indented = bool(raw[:1].isspace())

        # Fenced code blocks may be indented (the review doc indents them under
        # the bullet they illustrate). Skip their contents entirely.
        if fence is not None:
            if stripped.startswith(fence):
                fence = None
            continue
        m_fence = re.match(r"^(`{3,})", stripped)
        if m_fence:
            fence = m_fence.group(1)
            continue

        m = re.match(r"^### (P\d\.\d+)\s+(.*)$", raw)
        if m:
            flush()
            cur = {"id": m.group(1), "title": m.group(2).strip()}
            continue

        if cur is None:
            continue
        if stripped == "---":
            continue
        if not stripped:
            # A blank line ends the current task's absorption but is not itself
            # content: anything indented after it is section context, not a
            # wrapped continuation of the task.
            absorbing = False
            continue

        # A new top-level checkbox starts a fresh task and resumes absorbing.
        if re.match(r"^- \[ \] ", raw):
            tasks.append(stripped)
            absorbing = True
            continue

        # Indented, non-table line directly under a task: a wrapped
        # continuation of that task.
        if absorbing and indented and not stripped.startswith("|"):
            tasks[-1] = f"{tasks[-1]} {stripped}"
            continue

        # Anything else (tables, standalone prose) is section context, not part
        # of the task. Keep it verbatim and stop absorbing.
        extra.append(stripped)
        absorbing = False

    flush()
    return sections


def clean_title(raw: str) -> str:
    t = raw.replace("`", "")
    t = re.sub(r"^([A-Za-z][\w./*() ]{0,40}?)\s+(has|is|are|does|fires|leaves|runs|reads|re-renders|clears|uses|adds|keeps|defaults|never|also|plenty)", r"\1 \2", t)
    return t.strip().rstrip(":").strip()


def with_table_breaks(lines: list[str]) -> list[str]:
    """Insert a blank line after each run of markdown table rows."""
    out: list[str] = []
    in_table = False
    for line in lines:
        is_row = line.startswith("|")
        if in_table and not is_row:
            out.append("")
            in_table = False
        out.append(line)
        in_table = is_row
    if in_table:
        out.append("")
    return out


def build_body(section: dict) -> str:
    ident = section["id"]
    priority = ident.split(".")[0]
    tier = PRIORITY_LABEL[priority]

    lines: list[str] = []
    lines.append(f"**Priority:** {tier}")
    lines.append("")
    lines.append(f"From the full code review — see [`CODE_REVIEW.md#{ident.lower()}`]({SOURCE_LINK}).")
    lines.append("")

    if section.get("intro"):
        lines.append(section["intro"])
        lines.append("")

    lines.append("---")
    lines.append("")

    tasks = section.get("tasks") or []
    if tasks:
        if section.get("mainTask"):
            lines.append("### Fix")
            lines.append("")
        for t in tasks:
            lines.append(t)
        lines.append("")

    extra = section.get("extra") or []
    if extra:
        lines.append("### Context")
        lines.append("")
        lines.extend(with_table_breaks(extra))
        lines.append("")

    lines.append("---")
    lines.append("")
    lines.append(f"- [ ] Verified locally (`go vet ./...` / `tsc --noEmit` / `vitest run` as applicable)")
    lines.append(f"- [ ] Regression test added where the fix is testable")
    lines.append(f"- [ ] Lint + typecheck still clean")
    lines.append("")
    lines.append("<sub>Filed automatically from the code review. "
                 "See [`CODE_REVIEW.md`]({src}) for the full prioritized list.</sub>".format(src=SOURCE_LINK))

    return "\n".join(lines).rstrip() + "\n"


def build_themed_body(key: str, sections_by_id: dict[str, dict]) -> str:
    title, milestone, labels, intro = THEMED[key]
    members = sorted(
        (i for i, t in BATCHED.items() if t == key),
        key=lambda s: [int(p) for p in s.lstrip("P").split(".")],
    )

    lines: list[str] = []
    lines.append(f"**Priority:** {PRIORITY_LABEL[milestone.split('-')[0]]}")
    lines.append("")
    lines.append(f"Batched low-priority cleanup. Source: [`CODE_REVIEW.md`]({SOURCE_LINK}).")
    lines.append("")
    lines.append(intro)
    lines.append("")
    lines.append("---")
    lines.append("")

    for ident in members:
        sec = sections_by_id.get(ident)
        if not sec:
            continue
        anchor = ident.lower()
        lines.append(f"### [{ident}]({SOURCE_LINK}#{anchor}) {clean_title(sec['title'])}")
        lines.append("")
        tasks = sec.get("tasks") or []
        for t in tasks:
            lines.append(t)
        extra = sec.get("extra") or []
        if extra:
            lines.append("")
            lines.extend(with_table_breaks(extra))
        lines.append("")

    lines.append("---")
    lines.append("")
    lines.append("- [ ] Each item above addressed or explicitly deferred with a comment")
    lines.append("- [ ] Lint + typecheck still clean")
    lines.append("")
    lines.append(f"<sub>Filed automatically from the code review. "
                 f"See [`CODE_REVIEW.md`]({SOURCE_LINK}) for the full prioritized list.</sub>")

    return "\n".join(lines).rstrip() + "\n"


# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------

def main() -> None:
    text = SOURCE.read_text()
    sections = parse_sections(text)
    if not sections:
        sys.exit("no findings parsed from CODE_REVIEW.md")

    by_id = {s["id"]: s for s in sections}
    OUT.mkdir(parents=True, exist_ok=True)
    for old in OUT.glob("*.md"):
        old.unlink()

    issues: list[dict] = []

    for sec in sections:
        ident = sec["id"]
        # Skip P3 items routed into a themed batch.
        if ident in BATCHED and BATCHED[ident] in THEMED:
            continue

        priority = ident.split(".")[0]
        tier = PRIORITY_LABEL[priority]
        labels = sorted({tier, "bug"} | set(EXTRA_LABELS_BY_ID.get(ident, [])))
        body = build_body(sec)
        fname = f"{ident.lower().replace('.', '-')}.md"
        (OUT / fname).write_text(body)

        issues.append({
            "id": ident,
            "file": fname,
            "title": f"[{ident}] {clean_title(sec['title'])}",
            "labels": labels,
            "milestone": tier,
        })

    for key, (title, milestone, labels, _intro) in THEMED.items():
        body = build_themed_body(key, by_id)
        fname = f"{key}.md"
        (OUT / fname).write_text(body)
        issues.append({
            "id": key,
            "file": fname,
            "title": f"[P3] {title}",
            "labels": sorted(set(labels)),
            "milestone": milestone,
        })

    manifest = {
        "repo": "jojianya/sweetspot",
        "baseBranch": "development",
        "source": "CODE_REVIEW.md",
        "milestones": [{"title": t, "description": d} for t, d in MILESTONES],
        "extraLabels": [{"name": n, "color": c, "description": d} for n, c, d in EXTRA_LABELS],
        "issues": issues,
    }
    MANIFEST.write_text(json.dumps(manifest, indent=2) + "\n")

    by_tier: dict[str, int] = {}
    for i in issues:
        by_tier[i["milestone"]] = by_tier.get(i["milestone"], 0) + 1

    print(f"Parsed {len(sections)} findings from CODE_REVIEW.md\n")
    for t, _ in MILESTONES:
        print(f"  {t:<14} {by_tier.get(t, 0):>2} issues")
    print(f"  {'TOTAL':<14} {len(issues):>2} issues")
    print(f"\n  {len(EXTRA_LABELS)} new labels, {len(MILESTONES)} milestones")
    print(f"\n  bodies   -> {OUT.relative_to(REPO_ROOT)}/")
    print(f"  manifest -> {MANIFEST.relative_to(REPO_ROOT)}")
    print("\nNext: ./scripts/review-issues/create.sh")


if __name__ == "__main__":
    main()
