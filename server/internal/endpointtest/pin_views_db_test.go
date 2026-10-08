package endpointtest

// DB-backed characterization of unique per-account pin views: first open
// counts, repeats and owner/anonymous opens do not, concurrent first opens
// count once, updated_at stays untouched, and hidden/missing pins 404.
// Anonymous-degradation cases (expired, revoked, malformed, Redis outage)
// must answer 200 with the current count and record nothing.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

const viewTestSecret = "pin-views-test-secret"

type viewFixture struct {
	router *gin.Engine
	repo   pins.Repository
	pool   *pgxpool.Pool
	author string
	other  string
	pinID  string
}

func setupViews(t *testing.T, bl *cache.Blacklist, checker ...middleware.SessionChecker) viewFixture {
	t.Helper()
	pool := requireEndpointDB(t)
	ctx := context.Background()
	author := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("view-author-%d@example.com", time.Now().UnixNano()), "user")
	other := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("view-other-%d@example.com", time.Now().UnixNano()), "user")
	var catID int
	if err := pool.QueryRow(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&catID); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	repo := pins.NewRepository(pool)
	caption := "view me"
	p, err := repo.CreatePin(ctx, pins.NewPin{
		UserID: author, Lat: 20, Lng: 20, Caption: &caption,
		CategoryID: catID, Geohash: "viewtest",
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}
	h := pins.NewHandler(repo, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	viewLimit := middleware.New(60, time.Minute)
	var ch middleware.SessionChecker
	if len(checker) > 0 {
		ch = checker[0]
	}
	r.POST("/pins/:id/view", middleware.OptionalAuth(viewTestSecret, bl, ch), viewLimit.Middleware(), h.RegisterView)
	return viewFixture{router: r, repo: repo, pool: pool, author: author, other: other, pinID: p.ID}
}

func viewToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := jwt.Generate(viewTestSecret, userID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return tok
}

func postView(t *testing.T, f viewFixture, pinID, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/pins/"+pinID+"/view", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func pinViews(t *testing.T, f viewFixture) int64 {
	t.Helper()
	var views int64
	if err := f.pool.QueryRow(context.Background(), `SELECT views FROM pins WHERE id = $1`, f.pinID).Scan(&views); err != nil {
		t.Fatalf("read views: %v", err)
	}
	return views
}

func pinViewRows(t *testing.T, f viewFixture) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM pin_views WHERE pin_id = $1`, f.pinID).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestDBPinViewsFirstCountsRepeatDoesNot(t *testing.T) {
	f := setupViews(t, nil)
	if got := pinViews(t, f); got != 0 {
		t.Fatalf("views = %d, want 0", got)
	}
	w := postView(t, f, f.pinID, viewToken(t, f.other))
	if w.Code != http.StatusOK {
		t.Fatalf("first open: got %d (%s)", w.Code, w.Body.String())
	}
	if got := pinViews(t, f); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
	if n := pinViewRows(t, f); n != 1 {
		t.Errorf("pin_views rows = %d, want 1", n)
	}
	w = postView(t, f, f.pinID, viewToken(t, f.other))
	if w.Code != http.StatusOK {
		t.Fatalf("repeat open: got %d (%s)", w.Code, w.Body.String())
	}
	if got := pinViews(t, f); got != 1 {
		t.Errorf("views after repeat = %d, want 1", got)
	}
	if n := pinViewRows(t, f); n != 1 {
		t.Errorf("pin_views rows after repeat = %d, want 1", n)
	}
}

func TestDBPinViewsOwnerAndAnonymousDoNotCount(t *testing.T) {
	f := setupViews(t, nil)
	if w := postView(t, f, f.pinID, viewToken(t, f.author)); w.Code != http.StatusOK {
		t.Fatalf("owner open: got %d (%s)", w.Code, w.Body.String())
	}
	if w := postView(t, f, f.pinID, ""); w.Code != http.StatusOK {
		t.Fatalf("anonymous open: got %d (%s)", w.Code, w.Body.String())
	}
	if got := pinViews(t, f); got != 0 {
		t.Errorf("views = %d, want 0", got)
	}
	if n := pinViewRows(t, f); n != 0 {
		t.Errorf("pin_views rows = %d, want 0", n)
	}
}

func TestDBPinViewsConcurrentFirstOpensCountOnce(t *testing.T) {
	f := setupViews(t, nil)
	token := viewToken(t, f.other)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := postView(t, f, f.pinID, token)
			if w.Code != http.StatusOK {
				t.Errorf("concurrent open: got %d (%s)", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if got := pinViews(t, f); got != 1 {
		t.Errorf("views = %d, want 1", got)
	}
	if n := pinViewRows(t, f); n != 1 {
		t.Errorf("pin_views rows = %d, want 1", n)
	}
}

func TestDBPinViewsLeaveUpdatedAtAlone(t *testing.T) {
	f := setupViews(t, nil)
	var before time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT updated_at FROM pins WHERE id = $1`, f.pinID).Scan(&before); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if w := postView(t, f, f.pinID, viewToken(t, f.other)); w.Code != http.StatusOK {
		t.Fatalf("open: got %d (%s)", w.Code, w.Body.String())
	}
	var after time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT updated_at FROM pins WHERE id = $1`, f.pinID).Scan(&after); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if !after.Equal(before) {
		t.Errorf("updated_at moved %v -> %v on a counted view", before, after)
	}
}

func TestDBPinViewsMissingAndHidden(t *testing.T) {
	f := setupViews(t, nil)
	w := postView(t, f, "123e4567-e89b-42d3-a456-426614174000", viewToken(t, f.other))
	if w.Code != http.StatusNotFound {
		t.Errorf("missing pin: got %d, want 404", w.Code)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE pins SET is_hidden = true WHERE id = $1`, f.pinID); err != nil {
		t.Fatalf("hide pin: %v", err)
	}
	w = postView(t, f, f.pinID, viewToken(t, f.other))
	if w.Code != http.StatusNotFound {
		t.Errorf("hidden pin: got %d, want 404", w.Code)
	}
}

type floorChecker struct {
	floor time.Time
	err   error
}

func (s floorChecker) CheckSession(context.Context, string) (middleware.SessionState, error) {
	return middleware.SessionState{ValidAfter: s.floor}, s.err
}

func TestDBPinViewsDegradedAuthActsAnonymous(t *testing.T) {
	expired, err := jwt.Generate(viewTestSecret, "someone", -time.Hour)
	if err != nil {
		t.Fatalf("mint expired token: %v", err)
	}
	revoked, err := jwt.Generate(viewTestSecret, "someone", time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}

	live, liveOK := liveEndpointBlacklist(t)

	cases := []struct {
		name       string
		bl         *cache.Blacklist
		checker    middleware.SessionChecker
		token      string
		skipNoLive bool
	}{
		{"expired token", nil, nil, expired, false},
		{"malformed token", nil, nil, "not-a-token", false},
		{"redis outage", cache.New("127.0.0.1:1", ""), nil, viewToken(t, "someone"), false},
		{"revoked token", nil, nil, revoked, true},
		{"revoked floor", nil, floorChecker{floor: time.Now().Add(time.Minute)}, viewToken(t, "someone"), false},
		{"checker error", nil, floorChecker{err: errDegradedBoom}, viewToken(t, "someone"), false},
	}
	if liveOK {
		claims, err := jwt.Validate(viewTestSecret, revoked)
		if err != nil {
			t.Fatalf("validate test token: %v", err)
		}
		if err := live.Revoke(context.Background(), claims.ID, time.Minute); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		for i := range cases {
			if cases[i].name == "revoked token" {
				cases[i].bl = live
			}
		}
	}
	if liveOK {
		claims, err := jwt.Validate(viewTestSecret, revoked)
		if err != nil {
			t.Fatalf("validate test token: %v", err)
		}
		if err := live.Revoke(context.Background(), claims.ID, time.Minute); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		for i := range cases {
			if cases[i].name == "revoked token" {
				cases[i].bl = live
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipNoLive && !liveOK {
				t.Skip("redis unreachable, skipping revoked-token case")
			}
			f := setupViews(t, tc.bl, tc.checker)
			w := postView(t, f, f.pinID, tc.token)
			if w.Code != http.StatusOK {
				t.Fatalf("got %d (%s), want 200", w.Code, w.Body.String())
			}
			if got := pinViews(t, f); got != 0 {
				t.Errorf("views = %d, want 0", got)
			}
			if n := pinViewRows(t, f); n != 0 {
				t.Errorf("pin_views rows = %d, want 0", n)
			}
		})
	}
}

var errDegradedBoom = errors.New("boom")

func TestDBPinViewsRateBudgetsPerUser(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	repo := pins.NewRepository(pool)
	h := pins.NewHandler(repo, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	pins.RegisterRoutes(r.Group(""), h, pins.RouteOptions{JWTSecret: viewTestSecret})

	// Seed two viewers and one pin through the shared fixture for data.
	f := setupViews(t, nil)
	tokenA := viewToken(t, f.other)
	second := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("view-budget-%d@example.com", time.Now().UnixNano()), "user")
	tokenB := viewToken(t, second)

	post := func(token string) int {
		req := httptest.NewRequest(http.MethodPost, "/pins/"+f.pinID+"/view", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	// User A exhausts a full minute of distinct-pin opens.
	for i := 0; i < 60; i++ {
		if code := post(tokenA); code != http.StatusOK {
			t.Fatalf("user A request %d: got %d, want 200", i+1, code)
		}
	}
	if code := post(tokenA); code != http.StatusTooManyRequests {
		t.Fatalf("user A request 61: got %d, want 429", code)
	}
	// User B shares the proxy IP but has a fresh budget.
	if code := post(tokenB); code != http.StatusOK {
		t.Fatalf("user B first request: got %d, want 200", code)
	}
}

func TestDBPinViewsAnonymousNotUserCapped(t *testing.T) {
	pool := requireEndpointDB(t)
	repo := pins.NewRepository(pool)
	h := pins.NewHandler(repo, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	pins.RegisterRoutes(r.Group(""), h, pins.RouteOptions{JWTSecret: viewTestSecret})

	f := setupViews(t, nil)
	// Anonymous opens never write, so the per-user cap must not apply: 61
	// anonymous requests from one IP all pass (only the 600/min backstop
	// could trip, far above this).
	for i := 0; i < 61; i++ {
		req := httptest.NewRequest(http.MethodPost, "/pins/"+f.pinID+"/view", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("anonymous request %d: got %d, want 200", i+1, w.Code)
		}
	}
}

func liveEndpointBlacklist(t *testing.T) (*cache.Blacklist, bool) {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	bl := cache.New(addr, os.Getenv("REDIS_PASSWORD"))
	if err := bl.Ping(context.Background()); err != nil {
		return nil, false
	}
	return bl, true
}
