# Goodspot client

Next.js (App Router) + TypeScript + Tailwind + Vitest frontend for Goodspot.
Proxied same-origin to the Go API (see `next.config.ts`); session is an
httpOnly cookie. Run with pnpm 10 (`packageManager` pins it).

## Commands

```bash
pnpm dev                  # next dev
pnpm build                # production build (also runs type/bundle checks)
pnpm lint                 # eslint — 0 errors; a few pre-existing <img> warnings
pnpm next typegen         # required once on a cold checkout before tsc
pnpm tsc --noEmit         # type-check
pnpm test                 # vitest run (64 files, 438 tests)
```

## Source layout

Feature-based — see `AGENTS.md` for the tree and the import rules. In
short: `src/app/` holds thin routes, `src/features/<name>/` owns one
domain behind an `index.ts` public surface, and `src/shared/` holds what
2+ features use. Cross-feature imports go through `@/features/<name>`;
intra-feature imports stay relative; `vi.mock` targets keep leaf paths.
