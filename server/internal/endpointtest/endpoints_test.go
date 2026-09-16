package endpointtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

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
		return users.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	}
	if _, ok := m.byUsername[username]; ok {
		return users.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
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

func (m *mockUserService) UpdateRole(ctx context.Context, actorID, userID, role string) (users.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	u.Role = role
	m.users[userID] = u
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

func (m *mockReportRepo) ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (reports.Report, error) {
	return m.reviewed, m.reviewErr
}

func (m *mockReportRepo) ListReports(ctx context.Context, status *string, limit, offset int) ([]reports.ReportListEntry, error) {
	return m.list, m.listErr
}

func newToken(t *testing.T, userID, role string) string {
	t.Helper()
	tok, err := jwt.Generate(testSecret, userID, role, time.Hour)
	if err != nil {
		t.Fatalf("jwt.Generate: %v", err)
	}
	return tok
}

func setupRouter(usersSvc users.Service, reportRepo reports.Repository, bl *cache.Blacklist) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover(), middleware.SecurityHeaders())

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))

	authSvc := auth.NewService(usersSvc, testSecret)
	authH := auth.NewHandler(authSvc, bl, middleware.New(1000, time.Minute))
	auth.RegisterRoutes(jsonRoutes, authH, auth.RouteOptions{JWTSecret: testSecret, Blacklist: bl})

	userH := users.NewHandler(usersSvc)
	users.RegisterRoutes(jsonRoutes, userH, users.RouteOptions{JWTSecret: testSecret, Blacklist: bl})

	reportH := reports.NewHandler(reports.NewService(reportRepo))
	reports.RegisterRoutes(jsonRoutes, reportH, reports.RouteOptions{
		JWTSecret:   testSecret,
		Blacklist:   bl,
		UserService: usersSvc,
	})

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
	usersSvc := &mockUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
		users:      map[string]users.User{},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, nil)

	t.Run("RegisterSuccess", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/auth/register", `{"email":"new@example.com","password":"password123","username":"newbie"}`, nil)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if _, ok := body["token"]; !ok {
			t.Fatalf("expected token in response, got %v", body)
		}
		if _, ok := body["user"]; !ok {
			t.Fatalf("expected user in response, got %v", body)
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
		body := decodeBody(t, w)
		if _, ok := body["token"]; !ok {
			t.Fatalf("expected token, got %v", body)
		}
	})

	t.Run("LoginByUsername", func(t *testing.T) {
		hash, _ := passwordHash("password123")
		usersSvc.byUsername["alice"] = users.User{ID: testUUID1, Email: "a@example.com", Username: "alice", PasswordHash: hash, Role: users.RoleUser}
		w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"alice","password":"password123"}`, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if _, ok := body["token"]; !ok {
			t.Fatalf("expected token, got %v", body)
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
		tok := newToken(t, testUUID1, users.RoleUser)
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
}

func TestUserEndpoints(t *testing.T) {
	usersSvc := &mockUserService{
		users: map[string]users.User{
			testUUID1: {ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleOwner},
			testUUID2: {ID: testUUID2, Email: "b@example.com", Username: "bob", Role: users.RoleUser},
		},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, nil)

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
		ownTok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID2, "", map[string]string{"Authorization": "Bearer " + ownTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if body["email"] != "b@example.com" {
			t.Fatalf("expected own email in response, got %v", body["email"])
		}
	})

	t.Run("GetOtherUserOmitsEmail", func(t *testing.T) {
		otherTok := newToken(t, testUUID2, users.RoleUser)
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
		ownerTok := newToken(t, testUUID1, users.RoleOwner)
		w := doJSON(t, r, http.MethodPatch, "/users/"+testUUID2+"/role", `{"role":"admin"}`, map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UpdateRoleNotOwnerForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2, users.RoleUser)
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
		ownerTok := newToken(t, testUUID1, users.RoleOwner)
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
	r := setupRouter(usersSvc, reportRepo, nil)

	t.Run("CreateReport", func(t *testing.T) {
		tok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"this is spam"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportPinNotFound", func(t *testing.T) {
		reportRepo.pinExistsFn = func(ctx context.Context, pinID string) (bool, error) { return false, nil }
		defer func() { reportRepo.pinExistsFn = nil }()
		tok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"this is spam"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportInvalidBody", func(t *testing.T) {
		tok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID3+"/report", `{"reason":"sp"}`, map[string]string{"Authorization": "Bearer " + tok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateReportReasonTooLong", func(t *testing.T) {
		tok := newToken(t, testUUID2, users.RoleUser)
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
		ownerTok := newToken(t, testUUID1, users.RoleOwner)
		w := doJSON(t, r, http.MethodGet, "/reports", "", map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListReportsAsUserForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodGet, "/reports", "", map[string]string{"Authorization": "Bearer " + userTok})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListReportsInvalidStatus", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1, users.RoleOwner)
		w := doJSON(t, r, http.MethodGet, "/reports?status=bogus", "", map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ReviewReport", func(t *testing.T) {
		ownerTok := newToken(t, testUUID1, users.RoleOwner)
		w := doJSON(t, r, http.MethodPatch, "/reports/"+testUUID3, `{"action":"dismiss"}`, map[string]string{"Authorization": "Bearer " + ownerTok})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ReviewReportUserForbidden", func(t *testing.T) {
		userTok := newToken(t, testUUID2, users.RoleUser)
		w := doJSON(t, r, http.MethodPatch, "/reports/"+testUUID3, `{"action":"dismiss"}`, map[string]string{"Authorization": "Bearer " + userTok})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

func TestSecurityHeadersPresent(t *testing.T) {
	usersSvc := &mockUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
	}
	r := setupRouter(usersSvc, &mockReportRepo{}, nil)

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
	r.Use(middleware.Recover())

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

func TestLoginLockoutAfterFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.Recover())

	hash, _ := passwordHash("password123")
	usersSvc := &mockUserService{
		byEmail: map[string]users.User{"lock@example.com": {ID: testUUID1, Email: "lock@example.com", PasswordHash: hash, Role: users.RoleUser}},
	}
	authSvc := auth.NewService(usersSvc, testSecret)
	emailLim := middleware.New(5, time.Minute)
	authH := auth.NewHandler(authSvc, nil, emailLim)
	auth.RegisterRoutes(r.Group(""), authH, auth.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	for i := 0; i < 5; i++ {
		w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"lock@example.com","password":"wrong"}`, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d (%s)", i+1, w.Code, w.Body.String())
		}
	}

	w := doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"lock@example.com","password":"wrong"}`, nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once locked, got %d (%s)", w.Code, w.Body.String())
	}

	w = doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"lock@example.com","password":"password123"}`, nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for correct password while locked, got %d (%s)", w.Code, w.Body.String())
	}

	emailLim.Reset("lock@example.com")
	w = doJSON(t, r, http.MethodPost, "/auth/login", `{"identifier":"lock@example.com","password":"password123"}`, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 after Reset, got %d (%s)", w.Code, w.Body.String())
	}
}
