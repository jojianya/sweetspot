# Documentation index

One line per document. Dates are the last git commit touching that file at restructure
time ("last reviewed"); owners are unassigned unless noted.

## Architecture

- `architecture/codebase-overview.md` — narrative codebase onboarding (last reviewed 2026-10-10).
- `architecture/system-architecture.md` — system diagram, components, data flows, scaling, repo map, plus merged requirements/estimates/fan-out/failures appendix (last reviewed 2026-10-07; appendix merged 2026-10-11).
- `architecture/tech-stack.md` — stack and version inventory (last reviewed 2026-10-10).

## API

- `api/rest-api.md` — REST endpoint reference (last reviewed 2026-09-26; known stale per `product/roadmap.md` item J).

## Product

- `product/prd.md` — product requirements (last reviewed 2026-10-04).
- `product/user-workflow.md` — end-to-end user journey (last reviewed 2026-10-04).
- `product/roadmap.md` — live A–K feature tracker (last reviewed 2026-10-03).

## Security

- `SECURITY.md` — security policy and review findings; stays at `docs/` root because `server/` code comments and `docker-compose.prod.yml` point to this path (last reviewed 2026-10-10).

## Operations

- `runbook.md` — production runbook; stays at `docs/` root because `server/` code comments and `docker-compose.prod.yml` point to this path (last reviewed 2026-10-10).
- `operations/first-deploy.md` — fresh-VPS deploy walkthrough (last reviewed 2026-10-10).

## Development

- `good-and-bad-practices.md` — binding coding/design rules, required reading per `AGENTS.md:2`; stays at `docs/` root because that pointer cannot move (last reviewed 2026-10-08).
- `development/local-development.md` — local Docker dev loop (last reviewed 2026-09-26).
- `development/branching-and-release.md` — `development → main → release` flow (last reviewed 2026-10-07).
- `development/documentation-conventions.md` — where new docs go, naming, ADRs, archiving.

## Reviews

- `CODE_REVIEW.md` — prioritized P0–P3 findings; stays at `docs/` root because `scripts/review-issues/generate.py`, `manifest.json` and 43 filed-issue permalinks depend on that exact path (last reviewed 2026-10-07). No `docs/reviews/` folder: it would be empty.

## Archive (history only, not current)

- `archive/detailed-checklist.md` — phased build checklist, newer revision (last reviewed 2026-10-07).
- `archive/old-detailed-checklist.md` — same checklist, older revision (last reviewed 2026-10-07).
- `archive/codebase-map.md` — generated 2026-10-08 codebase scan, reproducible via rescan.

No `docs/adr/` folder yet: no architecture decision records exist. See
`development/documentation-conventions.md` for when to write one.
