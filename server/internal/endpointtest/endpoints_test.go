package endpointtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
	"github.com/redis/go-redis/v9"
)

// getSessionCookie extracts the session_token cookie from a response, or nil
// if it is absent.
func getSessionCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, line := range w.Header().Values("Set-Cookie") {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, auth.CookieName+"=") {
				// Re-parse the full Set-Cookie header to get the attributes.
				return parseSetCookie(line)
			}
		}
	}
	return nil
}

// parseSetCookie parses a single Set-Cookie header value into an http.Cookie.
func parseSetCookie(header string) *http.Cookie {
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return nil
	}
	kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(kv) != 2 {
		return nil
	}
	c := &http.Cookie{Name: kv[0], Value: kv[1]}
	for _, attr := range parts[1:] {
		attr = strings.TrimSpace(attr)
		lower := strings.ToLower(attr)
		switch {
		case strings.HasPrefix(lower, "httponly"):
			c.HttpOnly = true
		case strings.HasPrefix(lower, "secure"):
			c.Secure = true
		case strings.HasPrefix(lower, "samesite=strict"):
			c.SameSite = http.SameSiteStrictMode
		case strings.HasPrefix(lower, "samesite=lax"):
			c.SameSite = http.SameSiteLaxMode
		case strings.HasPrefix(lower, "samesite=none"):
			c.SameSite = http.SameSiteNoneMode
		}
	}
	return c
}

const testSecret = "endpoint-test-secret"

var (
	testUUID1 = "123e4567-e89b-42d3-a456-426614174000"
	testUUID2 = "223e4567-e89b-42d3-a456-426614174000"
	testUUID3 = "323e4567-e89b-42d3-a456-426614174000"
)

func passwordHash(p string) (string, error) { return password.Hash(p) }

type mockUserService struct {
	users      map[string]users.User
	byEmail    map[string]users.User
	byUsername map[string]users.User
}

func (m *mockUserService) Create(_ context.Context, email, passwordHash, username string) (users.User, error) {
	if _, ok := m.byEmail[email]; ok {
		return users.User{}, users.ErrDuplicate
	}
	if _, ok := m.byUsername[username]; ok {
		return users.User{}, users.ErrDuplicate
	}
	u := users.User{ID: testUUID1, Email: email, PasswordHash: passwordHash, Username: username, Role: users.RoleUser}
	return u, nil
}

func (m *mockUserService) GetByEmail(_ context.Context, email string) (users.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (m *mockUserService) GetByUsername(_ context.Context, username string) (users.User, error) {
	u, ok := m.byUsername[username]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (m *mockUserService) GetByLogin(_ context.Context, identifier string) (users.User, error) {
	if u, ok := m.byEmail[identifier]; ok {
		return u, nil
	}
	if u, ok := m.byUsername[identifier]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (m *mockUserService) GetByID(ctx context.Context, id string) (users.User, error) {
	u, ok := m.users[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

// CheckSession mirrors GetByID for the auth middleware: mock users carry no
// revocation floor (zero ValidAfter, so any issued token passes) and their
// stored role. Unknown users fail closed, like a deleted account.
func (m *mockUserService) CheckSession(_ context.Context, id string) (middleware.SessionState, error) {
	u, ok := m.users[id]
	if !ok {
		return middleware.SessionState{}, users.ErrNotFound
	}
	return middleware.SessionState{Role: u.Role}, nil
}

func (m *mockUserService) UpdateRole(ctx context.Context, actorID, userID, role string) (users.User, error) {
	if actorID == userID {
		return users.User{}, users.ErrCannotChangeOwnRole
	}
	u, ok := m.users[userID]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	if u.Role == users.RoleOwner && role != users.RoleOwner {
		owners := 0
		for _, x := range m.users {
			if x.Role == users.RoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			return users.User{}, users.ErrCannotDemoteLastOwner
		}
	}
	u.Role = role
	m.users[userID] = u
	return u, nil
}

func (m *mockUserService) SearchUsers(_ context.Context, query string, limit, offset int) ([]users.User, int, error) {
	q := strings.ToLower(query)
	out := []users.User{}
	for _, u := range m.users {
		if strings.Contains(strings.ToLower(u.Username), q) {
			out = append(out, u)
		}
	}
	total := len(out)
	if offset >= len(out) {
		return []users.User{}, total, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], total, nil
}

func (m *mockUserService) ListUsers(_ context.Context, limit, offset int) ([]users.User, int, error) {
	out := make([]users.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	if offset >= len(out) {
		return []users.User{}, len(m.users), nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], len(m.users), nil
}

func (m *mockUserService) CountOwners(_ context.Context) (int, error) {
	owners := 0
	for _, u := range m.users {
		if u.Role == users.RoleOwner {
			owners++
		}
	}
	return owners, nil
}

func (m *mockUserService) CountUsers(_ context.Context) (int, error) {
	return len(m.users), nil
}

func (m *mockUserService) UpdateProfile(_ context.Context, id string, patch users.UpdateProfilePatch) (users.User, error) {
	u, ok := m.users[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	if patch.Username != nil {
		for otherID, other := range m.users {
			if otherID != id && other.Username == *patch.Username {
				return users.User{}, users.ErrUsernameTaken
			}
		}
		u.Username = *patch.Username
	}
	if patch.AvatarURL != nil {
		u.AvatarURL = patch.AvatarURL
	}
	if patch.Socials != nil {
		u.Socials = *patch.Socials
	}
	m.users[id] = u
	return u, nil
}

type mockReportRepo struct {
	exists      bool
	created     reports.Report
	createErr   error
	list        []reports.ReportListEntry
	listErr     error
	reviewed    reports.Report
	reviewErr   error
	pinExistsFn func(ctx context.Context, pinID string) (bool, error)
}

func (m *mockReportRepo) PinExists(ctx context.Context, pinID string) (bool, error) {
	if m.pinExistsFn != nil {
		return m.pinExistsFn(ctx, pinID)
	}
	return m.exists, nil
}

func (m *mockReportRepo) CreateReport(ctx context.Context, pinID, reporterID, reason string) (reports.Report, error) {
	return m.created, m.createErr
}

func (m *mockReportRepo) ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (reports.Report, *string, error) {
	return m.reviewed, nil, m.reviewErr
}

func (m *mockReportRepo) ListReports(ctx context.Context, status *string, limit, offset int) ([]reports.ReportListEntry, error) {
	return m.list, m.listErr
}

type mockFavoriteRepo struct {
	saveErr    error
	unsaveErr  error
	isSaved    bool
	isSavedErr error
	exists     bool
	existsErr  error
	entries    []favorites.Entry
	listErr    error
	ids        []string
	idsErr     error
}

func (m *mockFavoriteRepo) Save(context.Context, string, string) error {
	return m.saveErr
}

func (m *mockFavoriteRepo) Unsave(context.Context, string, string) error {
	return m.unsaveErr
}

func (m *mockFavoriteRepo) IsSaved(context.Context, string, string) (bool, error) {
	return m.isSaved, m.isSavedErr
}

func (m *mockFavoriteRepo) PinExists(context.Context, string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockFavoriteRepo) List(_ context.Context, _ string, limit, offset int) ([]favorites.Entry, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	total := len(m.entries)
	if offset >= len(m.entries) {
		return []favorites.Entry{}, total, nil
	}
	end := offset + limit
	if end > len(m.entries) {
		end = len(m.entries)
	}
	return m.entries[offset:end], total, nil
}

func (m *mockFavoriteRepo) ListIDs(context.Context, string) ([]string, error) {
	return m.ids, m.idsErr
}

// newToken mints a token for userID. It takes no role: the JWT deliberately
// carries only the user ID, and every authorization decision reads the role
// live from the user service. Tests that need a privileged caller must seed
// the user's role in the mock user service.
func newToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := jwt.Generate(testSecret, userID, time.Hour)
	if err != nil {
		t.Fatalf("jwt.Generate: %v", err)
	}
	return tok
}

// setRole changes a seeded user's role, standing in for the role column in the
// database. Tests that need a privileged caller use this to promote them, and
// tests for demotion use it to take privileges away again.
//
// The rest of the record is preserved so email/username lookups and timestamps
// asserted by other tests keep working.
func setRole(svc *mockUserService, userID, role string) {
	u := svc.users[userID]
	u.ID = userID
	u.Role = role
	svc.users[userID] = u
}

func setupRouter(usersSvc users.Service, reportRepo reports.Repository, favRepo favorites.Repository, bl *cache.Blacklist, store *storage.Local, sameSite auth.SameSiteMode) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover(nil), middleware.SecurityHeaders())

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))

	authSvc := auth.NewService(usersSvc, testSecret)
	authH := auth.NewHandler(authSvc, bl, middleware.New(1000, time.Minute), sameSite, nil)
	auth.RegisterRoutes(jsonRoutes, authH, auth.RouteOptions{JWTSecret: testSecret, Blacklist: bl, CookieSameSite: sameSite})

	userH := users.NewHandler(usersSvc, store)
	users.RegisterRoutes(jsonRoutes, userH, users.RouteOptions{JWTSecret: testSecret, Blacklist: bl})

	events := realtime.NewBrokerWithClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}))
	reportH := reports.NewHandler(reports.NewService(reportRepo), events).WithQuarantine(store, nil)
	reports.RegisterRoutes(jsonRoutes, reportH, reports.RouteOptions{
		JWTSecret:   testSecret,
		Blacklist:   bl,
		UserService: usersSvc,
	})

	favH := favorites.NewHandler(favRepo)
	favorites.RegisterRoutes(jsonRoutes, favH, favorites.RouteOptions{JWTSecret: testSecret, Blacklist: bl})

	return r
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, w.Body.String())
	}
	return m
}

func TestAuthEndpoints(t *testing.T) {
	for _, mode := range []auth.SameSiteMode{auth.SameSiteStrict, auth.SameSiteLax} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			usersSvc := &mockUserService{
				byEmail:    map[string]users.User{},
				byUsername: map[string]users.User{},
				users:      map[string]users.User{},
			}
			r := setupRouter(usersSvc, &mockReportRepo{}, &mockFavoriteRepo{}, nil, storage.NewLocal(t.TempDir(), "http://test.local"), mode)

			t.Run("RegisterSuccess", func(t *testing.T) {
				w := doJSON(t, r, http.MethodPost, "/auth/register", `{"email":"new@example.com","password":"password123","username":"newbie"}`, nil)
				if w.Code != http.StatusCreated {
					t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
				}
				body := decodeBody(t, w)
				if _, ok := body["user"]; !ok {
					t.Fatalf("expected user in response, got %v", body)
				}
				cookie := getSessionCookie(w)
				if cookie == nil {
					t.Fatal("expected session_token cookie in response")
				}
				if cookie.Value == "" {
					t.Fatal("session cookie is empty")
				}
				expectedSameSite := http.SameSiteStrictMode
				if mode == auth.SameSiteLax {
					expectedSameSite = http.SameSiteLaxMode
				}
				if !cookie.HttpOnly || cookie.SameSite != expectedSameSite {
					t.Fatalf("session cookie flags are not hardened: %+v (expected SameSite=%d)", cookie, expectedSameSite)
				}
				if cookie.Secure {
					t.Fatalf("Secure flag should be false for HTTP requests, got %v", cookie.Secure)
				}
				if _, ok := body["token"]; ok {
					t.Fatal("token must not be returned in the response body")
				}
			})

			t.Run("RegisterDuplicateEmail", func(t *testing.T) {
				usersSvc.byEmail["taken@example.com"] = users.User{ID: testUUID1, Email: "taken@example.com"}
				w := doJSON(t, r, http.MethodPost, "/auth/register", `{"email":"taken@example.com","password":"password123","username":"someone"}`, nil)
				if w.Code != http.StatusConflict {
					t.Fatalf("expected 409, got %d (%s)", w.Code, w.Body.String())
				}
			})

			t.Run("RegisterInvalidBody", func(t *testing.T) {
				w := doJSON(t, r, http.MethodPost, "/auth/register", `{"email":"not-an-email","password":"123","username":"x"}`, nil)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
				}
			})

			t.Run("LoginSuccess", func(t *testing.T) {
				hash, _ := passwordHash("password123")
				usersSvc.byEmail["a@example.com"] = users.User{ID: testUUID1, Email: "a@example.com", PasswordHash: hash, Role: users.RoleUser}
				w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"a@example.com","password":"password123"}`, nil)
				if w.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
				}
				cookie := getSessionCookie(w)
				if cookie == nil || cookie.Value == "" {
					t.Fatalf("expected session_token cookie, got %v", cookie)
				}
				body := decodeBody(t, w)
				if _, ok := body["token"]; ok {
					t.Fatal("token must not be returned in the response body")
				}
			})

			t.Run("LoginByUsername", func(t *testing.T) {
				hash, _ := passwordHash("password123")
				usersSvc.byUsername["alice"] = users.User{ID: testUUID1, Email: "a@example.com", Username: "alice", PasswordHash: hash, Role: users.RoleUser}
				w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"alice","password":"password123"}`, nil)
				if w.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
				}
				cookie := getSessionCookie(w)
				if cookie == nil || cookie.Value == "" {
					t.Fatalf("expected session_token cookie, got %v", cookie)
				}
			})

			t.Run("LoginWrongPassword", func(t *testing.T) {
				w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"a@example.com","password":"wrongpass"}`, nil)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
				}
			})

			t.Run("LoginMalformedBody", func(t *testing.T) {
				w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"a@example.com"}`, nil)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
				}
			})

			t.Run("MeAuthenticated", func(t *testing.T) {
				usersSvc.users[testUUID1] = users.User{ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleUser}
				tok := newToken(t, testUUID1)
				w := doJSON(t, r, http.MethodGet, "/me", "", map[string]string{"Authorization": "Bearer " + tok})
				if w.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
				}
				body := decodeBody(t, w)
				if body["user_id"] != testUUID1 {
					t.Fatalf("expected user_id %s, got %v", testUUID1, body["user_id"])
				}
			})

			t.Run("MeUnauthenticated", func(t *testing.T) {
				w := doJSON(t, r, http.MethodGet, "/me", "", nil)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
				}
			})

			t.Run("MeInvalidToken", func(t *testing.T) {
				w := doJSON(t, r, http.MethodGet, "/me", "", map[string]string{"Authorization": "Bearer garbage.token.here"})
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
				}
			})
		})
	}
}

func TestUserEndpoints(t *testing.T) {
	usersSvc := &mockUserService{
		users: map[string]users.User{
			testUUID1: {ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleOwner},
			testUUID2: {
				ID:        testUUID2,
				Email:     "b@example.com",
				Username:  "bob",
				Role:      users.RoleUser,
				UpdatedAt: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			},
		},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, &mockFavoriteRepo{}, nil, storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)

	t.Run("GetUserByID", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID2, "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if body["id"] != testUUID2 {
			t.Fatalf("expected id %s, got %v", testUUID2, body["id"])
		}
		if _, leak := body["password_hash"]; leak {
			t.Fatal("password_hash leaked in public user response")
		}
		if _, leak := body["email"]; leak {
			t.Fatal("email leaked in anonymous public user response")
		}
	})

	t.Run("GetSelfIncludesEmail", func(t *testing.T) {
		ownTok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID2, "", map[string]string{"Authorization": "Bearer " + ownTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if body["email"] != "b@example.com" {
			t.Fatalf("expected own email in response, got %v", body["email"])
		}
		if body["updated_at"] != "2026-01-02T03:04:05Z" {
			t.Fatalf("expected updated_at in private profile, got %v", body["updated_at"])
		}
	})

	t.Run("GetOtherUserOmitsEmail", func(t *testing.T) {
		otherTok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID1, "", map[string]string{"Authorization": "Bearer " + otherTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if _, leak := body["email"]; leak {
			t.Fatal("email leaked to another authenticated user")
		}
	})

	t.Run("GetUserNotFound", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID3, "", nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("GetUserInvalidID", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/not-a-uuid", "", nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateRoleAsOwner", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPatch, "/users/"+testUUID2+"/role", `{"role":"admin"}`, map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateRoleNotOwnerForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodPatch, "/users/"+testUUID1+"/role", `{"role":"owner"}`, map[string]string{"Authorization": "Bearer " + userTok})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateRoleUnauthenticated", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPatch, "/users/"+testUUID2+"/role", `{"role":"admin"}`, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateRoleInvalidBody", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPatch, "/users/"+testUUID2+"/role", `{"role":"superuser"}`, map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

func TestReportEndpoints(t *testing.T) {
	usersSvc := &mockUserService{
		users: map[string]users.User{
			testUUID1: {ID: testUUID1, Email: "owner@example.com", Username: "owner", Role: users.RoleOwner},
			testUUID2: {ID: testUUID2, Email: "user@example.com", Username: "user", Role: users.RoleUser},
		},
	}
	reportRepo := &mockReportRepo{
		exists:   true,
		created:  reports.Report{Reason: "spam"},
		list:     []reports.ReportListEntry{{PinCaption: strPtr("spam pin")}},
		reviewed: reports.Report{Reason: "spam"},
	}
	r := setupRouter(usersSvc, reportRepo, &mockFavoriteRepo{}, nil, storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)

	t.Run("CreateReport", func(t *testing.T) {
		tok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"this is spam"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportPinNotFound", func(t *testing.T) {
		reportRepo.pinExistsFn = func(ctx context.Context, pinID string) (bool, error) { return false, nil }
		defer func() { reportRepo.pinExistsFn = nil }()
		tok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"this is spam"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportInvalidBody", func(t *testing.T) {
		tok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"sp"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportReasonTooLong", func(t *testing.T) {
		tok := newToken(t, testUUID2)
		long := strings.Repeat("a", 1001)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"`+long+`"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("OversizedJSONBodyRejected", func(t *testing.T) {
		big := strings.Repeat("a", 1<<20+1024)
		w := doJSON(t, r, http.MethodPost, "/auth/register", `{"reason":"`+big+`"}`, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for oversized body, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportUnauthenticated", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"this is spam"}`, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListReportsAsAdmin", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodGet, "/reports", "", map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListReportsAsUserForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodGet, "/reports", "", map[string]string{"Authorization": "Bearer " + userTok})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListReportsInvalidStatus", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodGet, "/reports?status=bogus", "", map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ReviewReport", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPatch, "/reports/"+testUUID3, `{"action":"dismiss"}`, map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ReviewReportUserForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2)
		w := doJSON(t, r, http.MethodPatch, "/reports/"+testUUID3, `{"action":"dismiss"}`, map[string]string{"Authorization": "Bearer " + userTok})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

func TestFavoriteEndpoints(t *testing.T) {
	usersSvc := &mockUserService{
		users: map[string]users.User{
			testUUID1: {ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleUser},
		},
	}
	favRepo := &mockFavoriteRepo{exists: true}
	r := setupRouter(usersSvc, &mockReportRepo{}, favRepo, nil, storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)

	t.Run("SaveFavorite", func(t *testing.T) {
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPut, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("SaveFavoriteDuplicateIdempotent", func(t *testing.T) {
		// The repository inserts with ON CONFLICT DO NOTHING, so a repeated
		// save is a successful no-op (204), not a conflict.
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPut, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 on duplicate save, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("SaveFavoritePinNotFound", func(t *testing.T) {
		favRepo.exists = false
		defer func() { favRepo.exists = true }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPut, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("SaveFavoriteInvalidPinID", func(t *testing.T) {
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPut, "/favorites/not-a-uuid", "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("SaveFavoriteUnauthenticated", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPut, "/favorites/"+testUUID3, "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("SaveFavoriteRepoError", func(t *testing.T) {
		favRepo.saveErr = errors.New("db unavailable")
		defer func() { favRepo.saveErr = nil }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodPut, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UnsaveFavorite", func(t *testing.T) {
		favRepo.isSaved = true
		defer func() { favRepo.isSaved = false }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodDelete, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UnsaveFavoriteNotSaved", func(t *testing.T) {
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodDelete, "/favorites/"+testUUID3, "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListSaved", func(t *testing.T) {
		favRepo.entries = []favorites.Entry{{SavedAt: time.Now()}}
		defer func() { favRepo.entries = nil }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodGet, "/favorites", "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		pins, ok := body["pins"].([]any)
		if !ok || len(pins) != 1 {
			t.Fatalf("expected 1 pin in response, got %v", body["pins"])
		}
	})

	t.Run("ListSavedIDs", func(t *testing.T) {
		favRepo.ids = []string{testUUID3}
		defer func() { favRepo.ids = nil }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodGet, "/favorites/ids", "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		ids, ok := body["ids"].([]any)
		if !ok || len(ids) != 1 || ids[0] != testUUID3 {
			t.Fatalf("expected [%s], got %v", testUUID3, body["ids"])
		}
	})

	t.Run("ListSavedUnauthenticated", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/favorites", "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListSavedRepoError", func(t *testing.T) {
		favRepo.listErr = errors.New("db unavailable")
		defer func() { favRepo.listErr = nil }()
		tok := newToken(t, testUUID1)
		w := doJSON(t, r, http.MethodGet, "/favorites", "", map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

func TestSecurityHeadersPresent(t *testing.T) {
	usersSvc := &mockUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, &mockFavoriteRepo{}, nil, storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)

	w := doJSON(t, r, http.MethodGet, "/me", "", nil)
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "Referrer-Policy", "Strict-Transport-Security"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("missing security header %q", h)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestSpoofedHeaderCannotBypassRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.Recover(nil))

	lim := middleware.New(1, time.Minute)
	r.POST("/auth/register", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if w := doJSON(t, r, http.MethodPost, "/auth/register", `{}`, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodPost, "/auth/register", `{}`, nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second request, got %d (%s)", w.Code, w.Body.String())
	}
	for _, spoofed := range []string{"10.0.0.1", "10.0.0.2, 10.0.0.3", "203.0.113.9"} {
		w := doJSON(t, r, http.MethodPost, "/auth/register", `{}`, map[string]string{"X-Forwarded-For": spoofed})
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 with spoofed %q, got %d (%s)", spoofed, w.Code, w.Body.String())
		}
	}
}

func TestTrustedProxyClientIPIsHonored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies([]string{"10.0.0.1"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}

	lim := middleware.New(1, time.Minute)
	r.POST("/limited", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": c.ClientIP()})
	})

	post := func(remoteAddr, xff string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/limited", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Same proxy peer, distinct forwarded clients: separate limiter buckets.
	if w := post("10.0.0.1:1111", "203.0.113.9"); w.Code != http.StatusOK {
		t.Fatalf("first client: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := post("10.0.0.1:2222", "198.51.100.7"); w.Code != http.StatusOK {
		t.Fatalf("second client behind same proxy: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	// First client again: its bucket is spent.
	if w := post("10.0.0.1:3333", "203.0.113.9"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("repeat client: expected 429, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestUntrustedPeerIgnoresForwardedForValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.GET("/ip", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ip": c.ClientIP()})
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ip"] != "192.0.2.1" {
		t.Fatalf("untrusted peer: ClientIP=%q, want peer 192.0.2.1 (XFF must be ignored)", body["ip"])
	}
}

// doLogin posts a login attempt as if it came from clientIP. Trusted proxies
// are disabled on the test routers, so RemoteAddr is what c.ClientIP() reads.
func doLogin(t *testing.T, r *gin.Engine, body, clientIP string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP + ":54321"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// loginLockoutRouter wires a login route with the given failure budget and a
// single known account, and returns the limiter so a test can inspect it.
func loginLockoutRouter(t *testing.T, budget int) (*gin.Engine, *middleware.Limiter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.Recover(nil))

	hash, _ := passwordHash("password123")
	usersSvc := &mockUserService{
		byEmail: map[string]users.User{"lock@example.com": {ID: testUUID1, Email: "lock@example.com", PasswordHash: hash, Role: users.RoleUser}},
	}
	lim := middleware.New(budget, time.Minute)
	authH := auth.NewHandler(auth.NewService(usersSvc, testSecret), nil, lim, auth.SameSiteStrict, nil)
	// Only the login route: the per-IP and global limiters registered by
	// RegisterRoutes would otherwise mask the per-identifier budget under test.
	r.POST("/auth/login", authH.Login)
	return r, lim
}

func TestLoginLockoutAfterFailures(t *testing.T) {
	r, _ := loginLockoutRouter(t, 5)

	for i := 0; i < 5; i++ {
		w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d (%s)", i+1, w.Code, w.Body.String())
		}
	}

	w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once locked, got %d (%s)", w.Code, w.Body.String())
	}

	w = doLogin(t, r, `{"identifier":"lock@example.com","password":"password123"}`, "192.0.2.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for correct password while locked, got %d (%s)", w.Code, w.Body.String())
	}
}

// The point of keying on IP+identifier: failures spent from one address must
// not lock the real owner out when they log in from another.
func TestLoginLockoutIsScopedToTheFailingIP(t *testing.T) {
	r, _ := loginLockoutRouter(t, 5)

	for i := 0; i < 5; i++ {
		if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attacker attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the attacking IP to be locked, got %d", w.Code)
	}

	// The owner's own address is unaffected, and the correct password works.
	w := doLogin(t, r, `{"identifier":"lock@example.com","password":"password123"}`, "198.51.100.7")
	if w.Code != http.StatusOK {
		t.Fatalf("victim locked out from another IP: %d (%s)", w.Code, w.Body.String())
	}
}

// One IP exhausting the budget on one identifier must not lock a second
// identifier from that same IP.
func TestLoginLockoutDoesNotBleedAcrossIdentifiers(t *testing.T) {
	r, _ := loginLockoutRouter(t, 5)

	for i := 0; i < 6; i++ {
		doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
	}
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"password123"}`, "192.0.2.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected the locked identifier to stay locked, got %d", w.Code)
	}

	w := doLogin(t, r, `{"identifier":"other@example.com","password":"whatever"}`, "192.0.2.1")
	if w.Code == http.StatusTooManyRequests {
		t.Fatalf("a different identifier was locked out too: %d (%s)", w.Code, w.Body.String())
	}
}

// A successful login clears only its own IP+identifier counter, so it cannot be
// used to release a lockout an attacker earned from their own address.
func TestLoginSuccessClearsOnlyItsOwnCounter(t *testing.T) {
	r, lim := loginLockoutRouter(t, 5)

	for i := 0; i < 6; i++ {
		doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
	}
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected attacker locked, got %d", w.Code)
	}

	// A good password from the owner's address succeeds...
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"password123"}`, "198.51.100.7"); w.Code != http.StatusOK {
		t.Fatalf("owner login: %d (%s)", w.Code, w.Body.String())
	}
	// ...and that success must not have released the attacker's lockout.
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("owner's success released the attacker's lockout: %d (%s)", w.Code, w.Body.String())
	}

	if !lim.Locked("192.0.2.1|lock@example.com") {
		t.Error("expected the attacker key to still be locked")
	}
	if lim.Locked("198.51.100.7|lock@example.com") {
		t.Error("expected the owner key to have been cleared")
	}
}

// An identifier nobody has must be indistinguishable from one that exists:
// same status, same body. (Their timing is equalized inside service.Login.)
func TestLoginDoesNotRevealAccountExistence(t *testing.T) {
	r, _ := loginLockoutRouter(t, 100)

	existing := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
	absent := doLogin(t, r, `{"identifier":"nobody@example.com","password":"wrong"}`, "192.0.2.1")

	if existing.Code != http.StatusUnauthorized || absent.Code != http.StatusUnauthorized {
		t.Fatalf("status codes differ: existing=%d absent=%d", existing.Code, absent.Code)
	}
	if existing.Body.String() != absent.Body.String() {
		t.Fatalf("bodies differ:\n existing=%s\n absent=%s", existing.Body.String(), absent.Body.String())
	}
}

// A locked-out attempt must also look the same whether or not the account
// exists: the 429 is emitted before any lookup.
func TestLoginLockoutDoesNotRevealAccountExistence(t *testing.T) {
	r, _ := loginLockoutRouter(t, 2)

	for i := 0; i < 3; i++ {
		doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1")
		doLogin(t, r, `{"identifier":"ghost@example.com","password":"wrong"}`, "192.0.2.1")
	}

	existing := doLogin(t, r, `{"identifier":"lock@example.com","password":"password123"}`, "192.0.2.1")
	absent := doLogin(t, r, `{"identifier":"ghost@example.com","password":"password123"}`, "192.0.2.1")

	if existing.Code != http.StatusTooManyRequests || absent.Code != http.StatusTooManyRequests {
		t.Fatalf("status codes differ: existing=%d absent=%d", existing.Code, absent.Code)
	}
	if existing.Body.String() != absent.Body.String() {
		t.Fatalf("bodies differ:\n existing=%s\n absent=%s", existing.Body.String(), absent.Body.String())
	}
}

// The identifier is case-folded because the account lookup is: otherwise an
// attacker mints a fresh counter per variant and never reaches the threshold.
func TestLoginLockoutFoldsIdentifierCase(t *testing.T) {
	r, lim := loginLockoutRouter(t, 3)

	for i := 0; i < 4; i++ {
		doLogin(t, r, `{"identifier":"LOCK@example.com","password":"wrong"}`, "192.0.2.1")
	}
	if !lim.Locked("192.0.2.1|lock@example.com") {
		t.Fatal("expected the case-folded key to hold the counter")
	}
	if w := doLogin(t, r, `{"identifier":"lock@example.com","password":"wrong"}`, "192.0.2.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected a case variant to be locked, got %d (%s)", w.Code, w.Body.String())
	}
}
