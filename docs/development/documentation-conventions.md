# Documentation conventions

## Where new docs go

- `architecture/` — system design, module layering, diagrams.
- `adr/` — architecture decision records, one file per decision (create the folder with the first ADR).
- `api/` — API reference.
- `product/` — PRD, requirements, roadmap, user workflow.
- `security/` — security policy, threat notes. (`SECURITY.md` stays at `docs/` root until the code/compose references to that path are updated elsewhere; list it here in the meantime via `docs/README.md`.)
- `operations/` — runbook, deployment, backup, incident procedures. (`runbook.md` stays at `docs/` root for the same reason.)
- `development/` — setup, contributing, good-and-bad-practices, testing, git/release flow.
- `reviews/` — code reviews and audits with dated filenames (only once a review exists that is not path-pinned elsewhere; `CODE_REVIEW.md` stays at `docs/` root — see `docs/README.md`).
- `archive/` — superseded material, kept for history.
- `docs/README.md` — the index. Add one line per new document, grouped by section, with a "last reviewed" date.

## Naming

- Lowercase kebab-case, no spaces (e.g. `first-deploy.md`, `branching-and-release.md`).
- No version suffixes like `final` or `v2`; history lives in git, superseded copies go to `archive/`.
- Dated files as `YYYY-MM-DD-topic.md` (reviews, incident notes, dated snapshots).

## When to write an ADR

Write an `adr/NNNN-title.md` record when a decision is hard to reverse or needs its rationale
preserved: data-model changes, auth/session design, API contract changes, infra and release
policy, deferred-work acceptances (e.g. the transactional-outbox gap). Template: context,
decision, alternatives considered, consequences. Number sequentially (`0001-`, `0002-`, …).

## When to archive

Archive to `docs/archive/` — never delete — when a document is superseded but has historical
value: completed build checklists, replaced designs, retired runbooks. When unsure between
archive and delete, choose archive. Delete only what is empty, stale beyond use, superseded
with the history fully preserved elsewhere, or generated and reproducible (note the generator).
Moves use `git mv` so history is preserved; update every inbound reference in the same branch
and keep `docs/README.md` current.
