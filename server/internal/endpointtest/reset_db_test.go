package endpointtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// captureMailer records deliveries for assertions and wakes waiters, so tests
// can observe the detached goroutine the request handler spawns.
type captureMailer struct {
	mu    sync.Mutex
	sent  []string
	links []string
	wake  chan struct{}
	// seen counts consumed mails so consecutive waitForMail calls return
	// successive links. The old code returned links[last] every time, so a
	// second call made before the next delivery landed silently returned
	// the first link again.
	seen int
}

func (m *captureMailer) SendPasswordReset(_ context.Context, to, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, to)
	m.links = append(m.links, link)
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return nil
}

func (m *captureMailer) waitForMail(t *testing.T) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		m.mu.Lock()
		n := len(m.links)
		seen := m.seen
		m.mu.Unlock()
		if n > seen {
			m.mu.Lock()
			defer m.mu.Unlock()
			link := m.links[seen]
			m.seen = seen + 1
			return link
		}
		select {
		case <-m.wake:
		case <-timeout:
			t.Fatal("timed out waiting for reset mail")
		}
	}
}

func (m *captureMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	parts := strings.Split(link, "token=")
	if len(parts) != 2 || parts[1] == "" {
		t.Fatalf("link has no token: %s", link)
	}
	return parts[1]
}

type resetFixture struct {
	router  *gin.Engine
	mailer  *captureMailer
	userSvc users.Service
	authSvc auth.Service
	store   *auth.ResetStore
	pool    *pgxpool.Pool
}

func setupReset(t *testing.T) resetFixture {
	t.Helper()
	return setupResetWithLimiter(t, 1000)
}

func setupResetWithLimiter(t *testing.T, idLimit int) resetFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := requireEndpointDB(t)

	userSvc := users.NewService(users.NewRepository(pool))
	authSvc := auth.NewService(userSvc, testSecret)
	store := auth.NewResetStore(pool)
	mailer := &captureMailer{wake: make(chan struct{}, 16)}
	resetH := auth.NewResetHandler(userSvc, store, mailer, "http://test.local",
		middleware.New(idLimit, time.Minute), auth.SameSiteStrict, nil)

	r := gin.New()
	r.POST("/auth/password/request", resetH.Request)
	r.POST("/auth/password/reset", resetH.Reset)
	r.GET("/me", middleware.AuthRequired(testSecret, nil, userSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return resetFixture{router: r, mailer: mailer, userSvc: userSvc, authSvc: authSvc, store: store, pool: pool}
}

func seedResetUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	hash, err := password.Hash("oldpassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	username := "reset-" + strings.SplitN(email, "@", 2)[0]
	var id string
	err = pool.QueryRow(context.Background(), `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, $2, $3, 'user')
		RETURNING id
	`, strings.ToLower(email), username, hash).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func loginToken(t *testing.T, f resetFixture, identifier, pw string) string {
	t.Helper()
	_, token, err := f.authSvc.Login(context.Background(), auth.LoginRequest{Identifier: identifier, Password: pw})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return token
}

func authedGet(t *testing.T, f resetFixture, token string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, f.router, http.MethodGet, "/me", "", map[string]string{"Authorization": "Bearer " + token})
}

// TestDBPasswordResetRequestHidesExistence proves the request endpoint answers
// identically for an existing and an unknown email, while only the real
// account triggers delivery.
func TestDBPasswordResetRequestHidesExistence(t *testing.T) {
	f := setupReset(t)
	email := fmt.Sprintf("exists-%d@example.com", time.Now().UnixNano())
	seedResetUser(t, f.pool, email)

	const ack = "if an account exists for that email, a reset link is on its way"
	w := doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), ack) {
		t.Fatalf("existing email: got %d (%s)", w.Code, w.Body.String())
	}
	f.mailer.waitForMail(t)

	w = doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"nobody-here@example.com"}`, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), ack) {
		t.Fatalf("unknown email: got %d (%s)", w.Code, w.Body.String())
	}
	if got := f.mailer.count(); got != 1 {
		t.Fatalf("expected exactly 1 delivery, got %d", got)
	}
}

// TestDBPasswordResetSuccessExpiryReuse covers the token lifecycle: a fresh
// token resets the password (and the new password logs in), an expired token
// fails, and a consumed token cannot be reused.
func TestDBPasswordResetSuccessExpiryReuse(t *testing.T) {
	f := setupReset(t)
	email := fmt.Sprintf("lifecycle-%d@example.com", time.Now().UnixNano())
	seedResetUser(t, f.pool, email)

	doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	raw := tokenFromLink(t, f.mailer.waitForMail(t))

	w := doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+raw+`","password":"brandnewpassword123"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("reset: got %d (%s)", w.Code, w.Body.String())
	}
	loginToken(t, f, email, "brandnewpassword123")
	if _, _, err := f.authSvc.Login(context.Background(), auth.LoginRequest{Identifier: email, Password: "oldpassword123"}); err == nil {
		t.Fatal("old password still works after reset")
	}

	// Reuse of the consumed token fails generically.
	w = doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+raw+`","password":"anothernewpassword"}`, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid or expired") {
		t.Fatalf("reuse: got %d (%s)", w.Code, w.Body.String())
	}

	// Expired token fails the same way.
	doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	raw2 := tokenFromLink(t, f.mailer.waitForMail(t))
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE password_resets SET expires_at = now() - interval '1 minute' WHERE user_id = (SELECT id FROM users WHERE email = $1) AND used_at IS NULL`,
		strings.ToLower(email)); err != nil {
		t.Fatalf("expire token: %v", err)
	}
	w = doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+raw2+`","password":"anothernewpassword"}`, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid or expired") {
		t.Fatalf("expired: got %d (%s)", w.Code, w.Body.String())
	}
}

// TestDBPasswordResetRevokesOldSessions proves sessions_valid_after: a token
// minted before the reset stops authenticating afterwards.
func TestDBPasswordResetRevokesOldSessions(t *testing.T) {
	f := setupReset(t)
	email := fmt.Sprintf("revoke-%d@example.com", time.Now().UnixNano())
	seedResetUser(t, f.pool, email)
	before := loginToken(t, f, email, "oldpassword123")
	if w := authedGet(t, f, before); w.Code != http.StatusOK {
		t.Fatalf("pre-reset session: got %d (%s)", w.Code, w.Body.String())
	}

	doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	raw := tokenFromLink(t, f.mailer.waitForMail(t))
	w := doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+raw+`","password":"rotatedpassword1"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("reset: got %d (%s)", w.Code, w.Body.String())
	}
	if w := authedGet(t, f, before); w.Code != http.StatusUnauthorized {
		t.Fatalf("old session after reset: got %d (%s), want 401", w.Code, w.Body.String())
	}
	after := loginToken(t, f, email, "rotatedpassword1")
	if w := authedGet(t, f, after); w.Code != http.StatusOK {
		t.Fatalf("fresh session after reset: got %d (%s)", w.Code, w.Body.String())
	}
}

// TestDBPasswordResetIdentifierThrottle proves per-identifier limiting: the
// third request for the same email inside the window answers 429.
func TestDBPasswordResetIdentifierThrottle(t *testing.T) {
	f := setupResetWithLimiter(t, 2)
	for i := 0; i < 2; i++ {
		w := doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"throttled@example.com"}`, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d (%s)", i+1, w.Code, w.Body.String())
		}
	}
	w := doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"throttled@example.com"}`, nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestDBPasswordResetSupersedesEarlierToken proves only the latest emailed
// link works: issuing twice invalidates the first.
func TestDBPasswordResetSupersedesEarlierToken(t *testing.T) {
	f := setupReset(t)
	email := fmt.Sprintf("super-%d@example.com", time.Now().UnixNano())
	seedResetUser(t, f.pool, email)

	doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	// Request #1, then its mail #1: waitForMail consumes in order, so this
	// is the first link even though delivery runs in a detached goroutine.
	first := tokenFromLink(t, f.mailer.waitForMail(t))
	doJSON(t, f.router, http.MethodPost, "/auth/password/request", `{"email":"`+email+`"}`, nil)
	// Request #2, then its mail #2: this blocks until a second delivery
	// exists, so it can never repeat the first link.
	second := tokenFromLink(t, f.mailer.waitForMail(t))

	w := doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+first+`","password":"supersededpass1"}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("superseded token: got %d (%s), want 400", w.Code, w.Body.String())
	}
	w = doJSON(t, f.router, http.MethodPost, "/auth/password/reset",
		`{"token":"`+second+`","password":"supersededpass1"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("latest token: got %d (%s)", w.Code, w.Body.String())
	}
}
