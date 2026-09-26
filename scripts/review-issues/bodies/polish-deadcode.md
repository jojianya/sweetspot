**Priority:** P3-polish

Batched low-priority cleanup. Source: [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md).

Placeholder files, dead parameters, and empty directories. Safe to delete — nothing imports them.

---

### [P3.19](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md#p3.19) logger.FromContext(nil) reads like a bug

- [ ] **Clean it up** — `server/internal/app/app.go:17`. Works (returns the default logger), but use `logger.Default()` and pass a real context, or drop the indirection.

### [P3.21](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md#p3.21) database.Connect uses log.Printf, not slog

- [ ] **Switch to `slog`** — `server/internal/platform/database/postgres.go`. Startup logs go to a different sink/format than everything else.

### [P3.22](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md#p3.22) server/deployments/k8s/ is an empty tracked directory

- [ ] **Add manifests or drop the path** — `-` (directory only, no files).

### [P3.23](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md#p3.23) Dead scaffolding in the server module tree

- [ ] **Delete or annotate** — 7 one-line files in `server/internal/modules/streams/` (`webhook`, `service`, `repository`, `model`, `livekit_client`, `handler`, `dto`) plus `realtime/{rooms,pubsub,hub,dto,broadcast}.go`. `migrations/0005_streams.sql` also exists with no code behind it. Delete, or add a `// TODO(owner):` header.

---

- [ ] Each item above addressed or explicitly deferred with a comment
- [ ] Lint + typecheck still clean

<sub>Filed automatically from the code review. See [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/CODE_REVIEW.md) for the full prioritized list.</sub>
