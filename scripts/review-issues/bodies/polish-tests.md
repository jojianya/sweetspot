**Priority:** P3-polish

Batched low-priority cleanup. Source: [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md).

Coverage gaps and CI improvements. `internal/endpointtest` covers feature behaviour well; the gap is unit coverage of the security primitives and race detection on the concurrent code.

---

### [P3.17](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.17) Missing Go tests for most modules

- [ ] **Add unit tests for the security primitives at minimum** — `comments`, `favorites`, `social`, `collections`, `pins`, `realtime`, `jwt`, `password`, `validid` all report `[no test files]`. `internal/endpointtest` covers features well, but `pkg/jwt` and `pkg/password` — the security primitives — have zero unit tests.

### [P3.18](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.18) CI never runs go test -race

- [ ] **Add `-race`** — `.github/workflows/*`. The rate limiter and SSE handler are both concurrency-sensitive.

---

- [ ] Each item above addressed or explicitly deferred with a comment
- [ ] Lint + typecheck still clean

<sub>Filed automatically from the code review. See [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md) for the full prioritized list.</sub>
