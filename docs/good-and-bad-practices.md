# Good and bad practices: sweetspot

Read this before writing or reviewing code. Each item shows a **BAD** pattern, the **GOOD** replacement, and why it matters.

- Items marked **Enforced by** are caught automatically.
- Other items are caught only in review, so reviewers (and AI agents) must check them by hand.
- Code samples are illustrative. Names like `ErrDuplicate` are examples. Check the real names in the repo before copying.

---

## Part 1: Layering (what `internal/arch` enforces)

The rule: **models and services must not know about the database driver, the web framework or the HTTP layer.** Handlers and repositories may.

```
handler  ->  service  ->  repository  ->  database
 (HTTP)     (rules)        (SQL)
```

### 1.1 Database types in a model

**BAD** `modules/pins/model.go`
```go
import "github.com/jackc/pgx/v5/pgtype"

type Pin struct {
    ID          pgtype.UUID
    Description pgtype.Text
    CreatedAt   pgtype.Timestamptz
}
```

**GOOD**
```go
import "time"

type Pin struct {
    ID          string
    Description *string   // nil means no description
    CreatedAt   time.Time
}
```
The repository converts between driver types and model types.

**Why:** A model that carries driver types forces every caller, test and handler to import the driver. You can't test business logic without a database, and a driver change touches the whole codebase.
**Enforced by:** `TestModelHasNoInfraImports`. Existing leaks in `pins`, `collections`, `comments` and `reports` are allowlisted. Don't add to the allowlist. Fix the import.

### 1.2 A service reading Postgres error codes

**BAD** `modules/auth/service.go`
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return ErrEmailTaken
}
```

**GOOD** (the repository translates, the service reads domain errors)
```go
// repository.go
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
    return ErrDuplicate
}

// service.go
if errors.Is(err, ErrDuplicate) {
    return ErrEmailTaken
}
```

**Why:** The service shouldn't know the storage engine. Today it breaks if you change the database, and a code typo ("23505") is invisible to the compiler.
**Enforced by:** `TestServiceHasNoInfraImports`. `auth` and `reports` are allowlisted. Fix these when you touch them, and remove the entry in the same commit.

### 1.3 A service importing the HTTP layer

**BAD** `modules/user/service.go`
```go
import "sweetspot247-backend/internal/http/middleware"

func (s *Service) Check(state middleware.SessionState) error { ... }
```

**GOOD**
```go
// A small type or interface owned by the service, or placed in a neutral package.
type SessionState interface {
    ValidAfter() time.Time
}
```
The middleware satisfies the interface. The service never imports `middleware`.

**Why:** If services import middleware, and middleware imports services, you get circular imports and a tangled design.
**Enforced by:** `TestServiceHasNoInfraImports`. `user` is allowlisted.

### 1.4 gin or a database handle inside a service

**BAD**
```go
func (s *Service) Create(c *gin.Context) { ... }
func (s *Service) Find(pool *pgxpool.Pool) { pool.Query(...) }
```

**GOOD**
```go
func (s *Service) Create(ctx context.Context, in CreateInput) (Pin, error) {
    return s.repo.Insert(ctx, in)
}
```
The service depends on a repository interface, takes plain values, and returns plain values or domain errors.
**Enforced by:** `TestServiceHasNoInfraImports`.

### 1.5 Dodging the check

**BAD**
- Putting forbidden imports in `entitlements.go` or `helpers.go` because the test only looks at `model.go` and `service.go`.
- Adding a new entry to an allowlist to make CI green.

**GOOD**
- Follow the intent of the rule in every file of the module.
- Change an allowlist only to **remove** an entry.

**Why:** The test only covers two file names. The design rule covers everything.

### 1.6 Business logic in a handler

**BAD**
```go
func (h *Handler) CreatePin(c *gin.Context) {
    // parse, then check plan, count pins, write SQL, build response
}
```

**GOOD**
```go
func (h *Handler) CreatePin(c *gin.Context) {
    in, err := parseCreatePin(c)         // parse and validate input
    if err != nil { respondError(c, err); return }
    pin, err := h.svc.Create(c.Request.Context(), in)
    respond(c, pin, err)                 // format the result
}
```
Handlers parse and respond. Services decide.

---

## Part 2: Go server practices

### 2.1 SQL built from strings

**BAD**
```go
q := "SELECT * FROM pins WHERE category = '" + category + "'"
```
**GOOD**
```go
rows, err := pool.Query(ctx, `SELECT id, title FROM pins WHERE category = $1 LIMIT $2`, category, limit)
```
**Why:** SQL injection. Parameters only, always.

### 2.2 Unbounded queries

**BAD**
```sql
SELECT ... FROM pins WHERE ST_Intersects(location, ST_MakeEnvelope($1,$2,$3,$4,4326))
```
**GOOD**
```sql
... ORDER BY rank DESC, created_at DESC LIMIT $5
```
**Why:** A world-sized viewport can return every pin. Every list endpoint needs a limit, a stable sort, and a maximum the client can't raise.

### 2.3 Ignored errors and silent failure

**BAD**
```go
ph, _ := password.Hash(pw)
_ = repo.Save(ctx, u)
```
**GOOD**
```go
ph, err := password.Hash(pw)
if err != nil { return fmt.Errorf("hash password: %w", err) }
```
**Why:** `ph, _ :=` hides failures. A failed hash would store an empty password hash and still return success. Tests may be the only place where ignoring is acceptable, and only with a comment.

### 2.4 Missing context and timeouts

**BAD**
```go
rows, err := pool.Query(context.Background(), q)
resp, err := http.Get(url)
```
**GOOD**
```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
rows, err := pool.Query(ctx, q)
```
**Why:** A slow dependency without a timeout ties up a goroutine for every request.

### 2.5 Reading config everywhere

**BAD**
```go
if os.Getenv("QUARANTINE_SWEEP_DRY_RUN") == "true" { ... }
```
**GOOD**
```go
// internal/config
QuarantineDryRun: getEnvBool("QUARANTINE_SWEEP_DRY_RUN", true)

// everywhere else
if cfg.QuarantineDryRun { ... }
```
**Why:** Settings are found and tested in one place. Also note that a dangerous feature should default to the safe value (`true` for dry run), so a missing env var can't cause real damage.

### 2.6 Using a value before it is initialized

**BAD**
```go
var AbsentAccountHash string          // exported, filled in later

var local = password.AbsentAccountHash // copies "" at startup
```
**GOOD**
```go
var (
    absentHash string
    absentOnce sync.Once
)

func GetAbsentAccountHash() string {
    absentOnce.Do(initAbsentAccountHash) // panics if hashing fails
    return absentHash
}
```
**Why:** The bad version can leave an empty hash. A bcrypt compare against an empty hash returns instantly, so a login with an unknown email becomes faster than one with a real email, and that leaks which emails exist. The real hash and the fake hash must use the same cost.

### 2.7 Tests that test a fake

**BAD**
```go
func TestLogin(t *testing.T) {
    s := &stubUserService{}   // the stub has its own Login method
    s.Login(...)              // proves nothing about production code
}
```
**GOOD**
```go
svc := auth.NewService(&fakeUserRepo{}, "test-secret") // real service, fake dependency
_, err := svc.Login(ctx, req)
```
**Why:** A test that calls a stub passes even when the production code is broken. Fake the dependency, never the thing under test.

### 2.8 Claiming a test passes when it was skipped

**BAD:** a test with `t.Skip("needs a running server")` counted as success.
**GOOD:** write it so it runs in CI, or list it as skipped and say why.

### 2.9 Repeating the same filter in several places

**BAD**
```go
// ListPins:     WHERE is_hidden = false AND <visibility rule>
// ListTrending: WHERE is_hidden = false AND <visibility rule, slightly different>
// Search:       WHERE is_hidden = false AND <visibility rule, forgotten>
```
**GOOD**
```go
// one place builds the visibility condition
cond, args := visibility.Filter(viewer, bbox, now)
```
**Why:** When the same rule exists in five queries, the sixth change (premium plans) will miss one, and that route leaks data.

---

## Part 3: Premium and entitlements

### 3.1 Checking plan names all over the code

**BAD**
```go
if user.Plan == "premium" || user.Plan == "business" { ... }
```
**GOOD**
```go
ent := entitlements.For(user.Plan, user.PlanUntil, now)
if ent.CanSeeAllZoomLevels { ... }
```
**Why:** Adding a tier means editing one function, not hunting through the code.

### 3.2 Trusting the client for the visibility rule

**BAD**
```go
zoom, _ := strconv.Atoi(c.Query("zoom"))
if zoom < 14 && !premium { hideFreePins() }
```
A free user sends `zoom=20` with a world-sized box and sees everything.

**GOOD:** derive the effective zoom from the viewport size on the server, and cap results for free pins when the box is wide.

### 3.3 Enforcing on one route only

**BAD:** the rule is applied in `ListPins`, but not in search, trending, a user's pins, or the SSE `/events` stream.
**GOOD:** every route that returns pins uses the shared visibility helper, and a test covers each one.

### 3.4 Expiry that needs a job

**BAD:** a cron job that sets `plan = 'free'` when the plan ends.
**GOOD:** compare `plan_until` with `now` every time (`entitlements.For(..., now)`). Expired plans fall back to free on their own, and nothing is deleted.

---

## Part 4: Next.js client

**BAD**
```tsx
function PinCard() {
  const [pin, setPin] = useState<any>(null)
  useEffect(() => { fetch('/api/pins/1').then(r => r.json()).then(setPin) }, [])
  return <div>{pin.title}</div>   // crashes while loading or on error
}
```
**GOOD**
```tsx
function PinCard({ id }: { id: string }) {
  const { pin, error, loading } = usePinDetail(id)   // existing hook in one place
  if (loading) return <Spinner />
  if (error) return <ErrorMessage error={error} />
  return <div>{pin.title}</div>
}
```
**Why:** Fetch logic lives in the hooks and `lib/api` modules. Types are shared. Every view has a loading state and an error state. No `any` or `@ts-ignore` without a comment explaining it.

**Build-time variables.** `NEXT_PUBLIC_*` values are baked in when the client is built. **BAD:** setting one only in the container's runtime `environment`. **GOOD:** pass it as a build `arg`.

---

## Part 5: Docker, nginx and environment

### 5.1 Compose images overwriting each other

**BAD:** dev and prod services with no `image:` tag both resolve to `sweetspot-server`. Building prod replaces the dev image, and the dev container then runs the prod binary (this caused the `exec: "./server": no such file` error).
**GOOD:** give dev services their own tags (`sweetspot-server-dev`), and test prod under its own project name:
```
docker compose -p sweetspot-prod -f docker-compose.prod.yml up -d --build
```

### 5.2 Dockerfiles

**BAD**
```dockerfile
FROM golang:latest
COPY . .
CMD ["./server"]          # runs as root, no healthcheck, unpinned base
```
**GOOD:** pinned base version, a non-root user (create the group before the user), and a healthcheck in the compose file.

### 5.3 Prod exposing internal services

**BAD**
```yaml
postgres:
  ports: ["5432:5432"]
```
**GOOD:** in `docker-compose.prod.yml`, only nginx publishes ports (80 and 443). Postgres, Redis, server and client stay on the internal network.

### 5.4 nginx header inheritance

**BAD**
```nginx
location / {
    proxy_set_header Host $host;           # sets one header
}
location /_next/hmr {
    proxy_set_header Upgrade $http_upgrade; # now the outer Host is gone
    proxy_pass http://client;
}
```
**GOOD:** repeat every needed `proxy_set_header` (Host, Upgrade, Connection) in each `location` block that sets any. A block that defines its own `proxy_set_header` ignores the outer ones.

### 5.5 Dev nginx drifting from prod

**BAD:** changing security headers or routing in `dev.conf.template` only.
**GOOD:** the two configs differ only in websockets, the redirect port and the dev certificate. Shared settings change in both, and `nginx -t` runs on every change.

### 5.6 New variables, secrets and unsafe defaults

**BAD**
- Adding `MAILER_WEBHOOK_KEY` to the code but not to `.env.example`.
- A prod `.env` with `CORS_ALLOWED_ORIGINS=http://localhost:3000`.
- `QUARANTINE_SWEEP_DRY_RUN` defaulting to `false` in prod.
- Committing `.env`, `secrets/`, `.pebble-run/` or a key.

**GOOD**
- Every new variable goes into `.env.example` in the same commit, with a safe placeholder.
- Prod CORS and base URL are the real `https` domain.
- Dangerous settings default to the safe value.
- Check before every commit: `git ls-files | grep -E "(^|/)\.env$|secrets/|\.pem$|\.key$"` must print nothing.

### 5.7 Scripts that lose their execute bit

**BAD:** a script committed as `100644`. nginx's entrypoint skips it, and the container fails on first boot.
**GOOD:** commit it as `100755` and check with `git ls-files -s <file>`.

---

## Part 6: Git and release

**BAD**
- Committing straight to `main` or `release`.
- A branch that mixes a feature, a refactor and a cleanup.
- Pushing `release` before the server, DNS and `.env` exist (a push deploys).
- A workflow that still triggers on an old branch name, so deploys silently stop.

**GOOD**
- `feature/*` -> `development` -> `main` -> `release`, one task per branch.
- Messages like `fix(auth): derive absentAccountHash from real hashing settings`.
- Squash back-and-forth fixes before merging.
- Backups and a known rollback before every release.

---

## Part 7: Working with AI coding agents

| BAD | GOOD |
|---|---|
| "All done, everything passes" with no output | Shows `go build`, `go vet`, `go test -race` and client check output, including skipped tests |
| Guesses a file name, env var or port | Reads the file, or writes `NOT FOUND` |
| Applies a big change straight away | Shows the plan and the diff, then waits for approval |
| "While I'm here" refactors | One task per branch, no unrelated edits |
| Picks a certificate or pricing strategy alone | Stops and asks |
| Runs `docker compose down -v`, `volume prune`, `reset --hard` | Uses only `stop`, `start`, `up -d`, `logs`, `ps` |
| Discards uncommitted work to get a clean tree | Stops and tells the person |
| Builds on an output that looks corrupted | Says it looks wrong and re-runs it |
| Adds an allowlist entry to silence the arch test | Fixes the import |

---

## Before you open a PR

```
cd server && go build ./... && go vet ./... && go test ./... -race -count=1
cd client && pnpm lint && pnpm test && npx tsc --noEmit
git status                                   # no .env, secrets/, keys
git ls-files -s nginx/ certbot/ scripts/     # scripts are 100755
```

- Does every new env var appear in `.env.example`?
- Does every new list query have a limit and an index?
- Does any new `model.go` or `service.go` import a driver, gin or middleware?
- If pins are returned anywhere new, does it use the shared visibility helper?
- Does the nginx config pass `nginx -t`, with the same headers in dev and prod?
