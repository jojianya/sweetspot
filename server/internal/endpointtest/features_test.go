package endpointtest

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	apphttp "github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// --- mocks ---------------------------------------------------------------

// stubPinRepo overrides only the pin methods exercised by the feature tests;
// embedding the nil interface keeps it compilable against pins.Repository.
type stubPinRepo struct {
	pins.Repository
	pinDetail     pins.PinDetail
	pinDetailErr  error
	updated       pins.Pin
	updateErr     error
	userPins      []pins.PinListEntry
	userPinsErr   error
	trending      []pins.TrendingPin
	trendingErr   error
	userExists    bool
	userExistsErr error
	views         int64
	viewErr       error
}

func (s *stubPinRepo) RegisterView(context.Context, string) (int64, error) {
	return s.views, s.viewErr
}

func (s *stubPinRepo) GetPin(context.Context, string) (pins.PinDetail, error) {
	return s.pinDetail, s.pinDetailErr
}

func (s *stubPinRepo) UpdatePin(context.Context, string, pins.UpdatePinPatch) (pins.Pin, error) {
	return s.updated, s.updateErr
}

func (s *stubPinRepo) ListByUser(context.Context, string, int) ([]pins.PinListEntry, error) {
	return s.userPins, s.userPinsErr
}

func (s *stubPinRepo) ListTrending(context.Context, [4]float64, int) ([]pins.TrendingPin, error) {
	return s.trending, s.trendingErr
}

func (s *stubPinRepo) CategoryExists(context.Context, int) (bool, error) {
	return true, nil
}

func (s *stubPinRepo) UserExists(context.Context, string) (bool, error) {
	return s.userExists, s.userExistsErr
}

type mockCommentRepo struct {
	list        []comments.Comment
	listErr     error
	created     comments.Comment
	createErr   error
	get         comments.Comment
	getErr      error
	hideErr     error
	deleteErr   error
	hideCalls   int
	deleteCalls int
}

func (m *mockCommentRepo) ListByPin(context.Context, string) ([]comments.Comment, error) {
	return m.list, m.listErr
}

func (m *mockCommentRepo) Create(context.Context, string, string, string) (comments.Comment, error) {
	return m.created, m.createErr
}

func (m *mockCommentRepo) Get(context.Context, string) (comments.Comment, error) {
	return m.get, m.getErr
}

func (m *mockCommentRepo) Hide(context.Context, string) error {
	m.hideCalls++
	return m.hideErr
}

func (m *mockCommentRepo) Delete(context.Context, string) error {
	m.deleteCalls++
	return m.deleteErr
}

type mockSocialRepo struct {
	userExists    bool
	userExistsErr error
	followErr     error
	unfollowErr   error
	isFollowing   bool
	followers     int
	following     int
	pinsCount     int
	feed          []pins.PinListEntry
	feedErr       error
}

func (m *mockSocialRepo) UserExists(context.Context, string) (bool, error) {
	return m.userExists, m.userExistsErr
}

func (m *mockSocialRepo) Follow(context.Context, string, string) error {
	return m.followErr
}

func (m *mockSocialRepo) Unfollow(context.Context, string, string) error {
	return m.unfollowErr
}

func (m *mockSocialRepo) IsFollowing(context.Context, string, string) (bool, error) {
	return m.isFollowing, nil
}

func (m *mockSocialRepo) CountFollowers(context.Context, string) (int, error) {
	return m.followers, nil
}

func (m *mockSocialRepo) CountFollowing(context.Context, string) (int, error) {
	return m.following, nil
}

func (m *mockSocialRepo) CountPins(context.Context, string) (int, error) {
	return m.pinsCount, nil
}

func (m *mockSocialRepo) Feed(context.Context, string, int) ([]pins.PinListEntry, error) {
	return m.feed, m.feedErr
}

type mockCollectionRepo struct {
	exists        bool
	userExists    bool
	created       collections.Collection
	createErr     error
	collections   []collections.Collection
	collection    collections.Collection
	collectionErr error
	detail        collections.CollectionDetail
	detailErr     error
	updateErr     error
	deleteErr     error
	pins          []pins.PinListEntry
	pinExists     bool
	addErr        error
	removeErr     error
}

func (m *mockCollectionRepo) UserExists(context.Context, string) (bool, error) {
	return m.userExists, nil
}

func (m *mockCollectionRepo) Create(context.Context, string, string, *string) (collections.Collection, error) {
	return m.created, m.createErr
}

func (m *mockCollectionRepo) Get(context.Context, string) (collections.Collection, error) {
	return m.collection, m.collectionErr
}

func (m *mockCollectionRepo) ListByUser(context.Context, string) ([]collections.Collection, error) {
	return m.collections, nil
}

func (m *mockCollectionRepo) Update(context.Context, string, string, *string) error {
	return m.updateErr
}

func (m *mockCollectionRepo) Delete(context.Context, string) error {
	return m.deleteErr
}

func (m *mockCollectionRepo) ListPins(context.Context, string) ([]pins.PinListEntry, error) {
	return m.pins, nil
}

func (m *mockCollectionRepo) PinExists(context.Context, string) (bool, error) {
	return m.pinExists, nil
}

func (m *mockCollectionRepo) AddPin(context.Context, string, string) error {
	return m.addErr
}

func (m *mockCollectionRepo) RemovePin(context.Context, string, string) error {
	return m.removeErr
}

// --- router + helpers ----------------------------------------------------

func setupFeaturesRouter(
	t *testing.T,
	pinRepo pins.Repository,
	commentRepo comments.Repository,
	socialRepo social.Repository,
	collectionRepo collections.Repository,
	usersSvc users.Service,
) (*gin.Engine, *storage.Local) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Recover(nil), middleware.SecurityHeaders())

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))
	uploadRoutes := r.Group("")
	uploadRoutes.Use(middleware.BodyLimit(64 << 20))

	authSvc := auth.NewService(usersSvc, testSecret)
	authH := auth.NewHandler(authSvc, nil, middleware.New(1000, time.Minute))
	auth.RegisterRoutes(jsonRoutes, authH, auth.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	jsonRoutes.POST("/errors", apphttp.ClientErrorIngest(nil))

	store := storage.NewLocal(t.TempDir(), "http://test.local")
	userH := users.NewHandler(usersSvc, store)
	// Registered on uploadRoutes: PATCH /users/me is multipart (avatar upload).
	users.RegisterRoutes(uploadRoutes, userH, users.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	pinH := pins.NewHandler(pinRepo, store, nil)
	pins.RegisterRoutes(uploadRoutes, pinH, pins.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	commentH := comments.NewHandler(commentRepo)
	comments.RegisterRoutes(jsonRoutes, commentH, comments.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	socialH := social.NewHandler(socialRepo)
	social.RegisterRoutes(jsonRoutes, socialH, social.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	colH := collections.NewHandler(collectionRepo)
	collections.RegisterRoutes(jsonRoutes, colH, collections.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	return r, store
}

func doMultipart(t *testing.T, r *gin.Engine, method, path string, fields map[string]string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doMultipartWithFile is doMultipart plus binary file parts (field name -> bytes).
func doMultipartWithFile(t *testing.T, r *gin.Engine, method, path string, fields map[string]string, files map[string][]byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	for field, data := range files {
		part, err := mw.CreateFormFile(field, "upload.png")
		if err != nil {
			t.Fatalf("create form file %s: %v", field, err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write form file %s: %v", field, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// tinyPNG returns a valid 64x64 PNG to exercise the avatar upload path.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func newUsersSvc() *mockUserService {
	return &mockUserService{
		byEmail:    map[string]users.User{},
		byUsername: map[string]users.User{},
		users: map[string]users.User{
			testUUID1: {ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleUser},
			testUUID2: {ID: testUUID2, Email: "b@example.com", Username: "bob", Role: users.RoleUser},
		},
	}
}

func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func uuidOf(s string) (v pgtype.UUID) {
	if err := v.Scan(s); err != nil {
		panic(err)
	}
	return v
}

func pinDetailOf(userID string) pins.PinDetail {
	return pins.PinDetail{Pin: pins.Pin{UserID: uuidOf(userID)}}
}

func pinListEntryOf(id string) pins.PinListEntry {
	return pins.PinListEntry{Pin: pins.Pin{ID: uuidOf(id)}}
}

func uuidStr() string { return testUUID3 }

// --- pin editing ---------------------------------------------------------

func TestUpdatePin(t *testing.T) {
	r, _ := setupFeaturesRouter(t, &stubPinRepo{pinDetail: pinDetailOf(testUUID1)}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
	token := newToken(t, testUUID1, "user")

	t.Run("OwnerEditsCaption", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/pins/"+testUUID1, map[string]string{"caption": "new caption"}, authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("NonOwnerForbidden", func(t *testing.T) {
		otherToken := newToken(t, testUUID2, "user")
		w := doMultipart(t, r, http.MethodPatch, "/pins/"+testUUID1, map[string]string{"caption": "hi"}, authHeaders(otherToken))
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("Unauthenticated401", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/pins/"+testUUID1, map[string]string{"caption": "hi"}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("InvalidId", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/pins/not-a-uuid", map[string]string{"caption": "hi"}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("MissingPin404", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{pinDetailErr: pins.ErrNotFound}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doMultipart(t, r2, http.MethodPatch, "/pins/"+testUUID1, map[string]string{"caption": "hi"}, authHeaders(token))
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

func TestListUserPins(t *testing.T) {
	t.Run("ExistingUser", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{userExists: true, userPins: []pins.PinListEntry{}}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID1+"/pins", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UnknownUser", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{userExists: false}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID1+"/pins", "", nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- trending pins (viewport hotness) --------------------------------------

func TestTrendingPins(t *testing.T) {
	t.Run("DefaultsToViewportTrending", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{trending: []pins.TrendingPin{{
			PinListEntry: pinListEntryOf(testUUID3),
			CommentCount: 2,
			Score:        3.25,
		}}}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodGet, "/pins/trending?bbox=0,0,1,1", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body struct {
			Pins []pins.TrendingPin `json:"pins"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Pins) != 1 || body.Pins[0].CommentCount != 2 || body.Pins[0].Score != 3.25 {
			t.Fatalf("expected the stubbed trending pin, got %+v", body.Pins)
		}
	})

	t.Run("MissingBbox400", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodGet, "/pins/trending", "", nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("InvalidBbox400", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodGet, "/pins/trending?bbox=not-a-bbox", "", nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- user search (owner-gated role management) -----------------------------

func TestSearchUsers(t *testing.T) {
	svc := newUsersSvc()
	svc.users[testUUID1] = users.User{ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleOwner}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, svc)
	ownerToken := newToken(t, testUUID1, "owner")
	userToken := newToken(t, testUUID2, "user")

	t.Run("OwnerCanSearch", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users?q=ali", "", authHeaders(ownerToken))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body struct {
			Users []users.PublicUser `json:"users"`
			Total int                `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Users) != 1 || body.Users[0].Username != "alice" {
			t.Fatalf("expected [alice], got %+v", body.Users)
		}
		if body.Total != 2 {
			t.Fatalf("expected total 2, got %d", body.Total)
		}
	})

	t.Run("NonOwnerForbidden", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users?q=alice", "", authHeaders(userToken))
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("Unauthenticated401", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users?q=alice", "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("OwnerCanListAllUsers", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users", "", authHeaders(ownerToken))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body struct {
			Users []users.PublicUser `json:"users"`
			Total int                `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Users) != 2 || body.Total != 2 {
			t.Fatalf("expected 2 users with total 2, got %d users / total %d", len(body.Users), body.Total)
		}
	})

	t.Run("OwnerCanPaginateList", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users?limit=1&offset=1", "", authHeaders(ownerToken))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body struct {
			Users []users.PublicUser `json:"users"`
			Total int                `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Users) != 1 || body.Total != 2 {
			t.Fatalf("expected 1 user with total 2, got %d users / total %d", len(body.Users), body.Total)
		}
	})
}

// --- profile editing (own profile) ----------------------------------------

func TestUpdateMyProfile(t *testing.T) {
	svc := newUsersSvc()
	r, store := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, svc)
	token := newToken(t, testUUID1, "user")

	t.Run("EditsUsernameAndSocials", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{
			"username": "alice2",
			"socials":  `{"instagram":"@alice2","website":"https://alice.example"}`,
		}, authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["email"] != "a@example.com" {
			t.Fatalf("expected private profile with email, got %v", body["email"])
		}
		if body["username"] != "alice2" {
			t.Fatalf("expected username alice2, got %v", body["username"])
		}
		socials, ok := body["socials"].(map[string]any)
		if !ok || socials["instagram"] != "@alice2" || socials["website"] != "https://alice.example" {
			t.Fatalf("unexpected socials: %v", body["socials"])
		}
	})

	t.Run("EmptyUsername400", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{"username": "   "}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("TooShortUsername400", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{"username": "a"}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("InvalidSocials400", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{"socials": "not-json"}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("NothingToUpdate400", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("UsernameTaken409", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{"username": "bob"}, authHeaders(token))
		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("Unauthenticated401", func(t *testing.T) {
		w := doMultipart(t, r, http.MethodPatch, "/users/me", map[string]string{"username": "hacker"}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("AvatarUpload", func(t *testing.T) {
		// Seed a previous avatar so we can assert the superseded file is dropped.
		oldURL := "http://test.local/uploads/old.webp"
		if err := os.WriteFile(filepath.Join(store.Dir(), "old.webp"), []byte("old"), 0o644); err != nil {
			t.Fatalf("write old avatar: %v", err)
		}
		svc.users[testUUID1] = users.User{ID: testUUID1, Email: "a@example.com", Username: "alice", Role: users.RoleUser, AvatarURL: &oldURL}

		w := doMultipartWithFile(t, r, http.MethodPatch, "/users/me", nil, map[string][]byte{"avatar": tinyPNG(t)}, authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		url, _ := body["avatar_url"].(string)
		if !strings.HasPrefix(url, "http://test.local/uploads/") {
			t.Fatalf("expected stored avatar URL, got %q", url)
		}
		if _, err := os.Stat(filepath.Join(store.Dir(), filepath.Base(url))); err != nil {
			t.Fatalf("expected avatar file on disk: %v", err)
		}
		if _, err := os.Stat(filepath.Join(store.Dir(), "old.webp")); !os.IsNotExist(err) {
			t.Fatalf("expected old avatar to be removed, got %v", err)
		}
	})

	t.Run("InvalidAvatar400", func(t *testing.T) {
		w := doMultipartWithFile(t, r, http.MethodPatch, "/users/me", nil, map[string][]byte{"avatar": []byte("not an image")}, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- pin views -----------------------------------------------------------

func TestPinView(t *testing.T) {
	t.Run("PublicIncrements", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{views: 42}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID1+"/view", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("MissingPin404", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{viewErr: pins.ErrNotFound}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID1+"/view", "", nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("InvalidId", func(t *testing.T) {
		r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r, http.MethodPost, "/pins/not-a-uuid/view", "", nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

// --- comments ------------------------------------------------------------

func TestCommentEndpoints(t *testing.T) {
	commentRepo := &mockCommentRepo{
		get: comments.Comment{PinID: uuidOf(testUUID1), UserID: uuidOf(testUUID1), Body: "nice"},
	}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, commentRepo, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
	token := newToken(t, testUUID1, "user")
	adminToken := newToken(t, testUUID2, "admin")

	t.Run("ListComments", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/pins/"+testUUID1+"/comments", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateComment", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID1+"/comments", `{"body":"great spot"}`, authHeaders(token))
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateCommentRequiresBody", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID1+"/comments", `{"body":""}`, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateCommentUnauthenticated", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/pins/"+testUUID1+"/comments", `{"body":"hi"}`, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("OwnerDeletesComment", func(t *testing.T) {
		w := doJSON(t, r, http.MethodDelete, "/comments/"+testUUID1, "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
		if commentRepo.deleteCalls != 1 {
			t.Fatalf("expected 1 hard delete, got %d", commentRepo.deleteCalls)
		}
	})

	t.Run("AdminHidesComment", func(t *testing.T) {
		w := doJSON(t, r, http.MethodDelete, "/comments/"+testUUID1, "", authHeaders(adminToken))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
		if commentRepo.hideCalls != 1 {
			t.Fatalf("expected 1 hide, got %d", commentRepo.hideCalls)
		}
	})

	t.Run("NonOwnerCannotDelete", func(t *testing.T) {
		otherToken := newToken(t, testUUID2, "user")
		w := doJSON(t, r, http.MethodDelete, "/comments/"+testUUID1, "", authHeaders(otherToken))
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- follows + feed ------------------------------------------------------

func TestFollowEndpoints(t *testing.T) {
	socialRepo := &mockSocialRepo{userExists: true, followers: 3, following: 1, pinsCount: 5, isFollowing: true}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, socialRepo, &mockCollectionRepo{}, newUsersSvc())
	token := newToken(t, testUUID1, "user")

	t.Run("Follow", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPut, "/users/"+testUUID2+"/follow", "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CannotFollowSelf", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPut, "/users/"+testUUID1+"/follow", "", authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("FollowUnknownUser", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{userExists: false}, &mockCollectionRepo{}, newUsersSvc())
		w := doJSON(t, r2, http.MethodPut, "/users/"+testUUID2+"/follow", "", authHeaders(token))
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("Unfollow", func(t *testing.T) {
		w := doJSON(t, r, http.MethodDelete, "/users/"+testUUID2+"/follow", "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID2+"/stats", "", authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
		body := decodeBody(t, w)
		if body["followers"] != float64(3) || body["is_following"] != true {
			t.Fatalf("unexpected stats: %v", body)
		}
	})

	t.Run("StatsAnonymousIsFollowingFalse", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID2+"/stats", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := decodeBody(t, w)
		if body["is_following"] != false {
			t.Fatalf("expected is_following false, got %v", body["is_following"])
		}
	})
}

func TestFeed(t *testing.T) {
	socialRepo := &mockSocialRepo{feed: []pins.PinListEntry{}}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, socialRepo, &mockCollectionRepo{}, newUsersSvc())
	token := newToken(t, testUUID1, "user")

	t.Run("FeedRequiresAuth", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/feed", "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("FeedOK", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/feed", "", authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- collections ---------------------------------------------------------

func TestCollectionEndpoints(t *testing.T) {
	colRepo := &mockCollectionRepo{
		exists:     true,
		userExists: true,
		collection: collections.Collection{UserID: uuidOf(testUUID1), Name: "Weekend"},
		detail:     collections.CollectionDetail{},
		pinExists:  true,
	}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, colRepo, newUsersSvc())
	token := newToken(t, testUUID1, "user")
	otherToken := newToken(t, testUUID2, "user")

	t.Run("ListMine", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/collections", "", authHeaders(token))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("ListMineRequiresAuth", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/collections", "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Create", func(t *testing.T) {
		colRepo2 := &mockCollectionRepo{created: collections.Collection{ID: uuidOf(testUUID1), UserID: uuidOf(testUUID1), Name: "Trip"}}
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, colRepo2, newUsersSvc())
		w := doJSON(t, r2, http.MethodPost, "/collections", `{"name":"Trip","description":"summer"}`, authHeaders(token))
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("CreateRequiresName", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/collections", `{"name":""}`, authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("GetPublic", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/collections/"+testUUID1, "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("OwnerUpdates", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}}, newUsersSvc())
		w := doJSON(t, r2, http.MethodPatch, "/collections/"+testUUID1, `{"name":"Renamed"}`, authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("NonOwnerCannotUpdate", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}}, newUsersSvc())
		w := doJSON(t, r2, http.MethodPatch, "/collections/"+testUUID1, `{"name":"Stolen"}`, authHeaders(otherToken))
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("OwnerDeletes", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}}, newUsersSvc())
		w := doJSON(t, r2, http.MethodDelete, "/collections/"+testUUID1, "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("AddPin", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}, pinExists: true}, newUsersSvc())
		w := doJSON(t, r2, http.MethodPut, "/collections/"+testUUID1+"/pins/"+uuidStr(), "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("AddPinInvalidParam", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPut, "/collections/"+testUUID1+"/pins/not-a-uuid", "", authHeaders(token))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("AddPinUnknownPin", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}, pinExists: false}, newUsersSvc())
		w := doJSON(t, r2, http.MethodPut, "/collections/"+testUUID1+"/pins/"+uuidStr(), "", authHeaders(token))
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("RemovePin", func(t *testing.T) {
		r2, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{collection: collections.Collection{UserID: uuidOf(testUUID1)}}, newUsersSvc())
		w := doJSON(t, r2, http.MethodDelete, "/collections/"+testUUID1+"/pins/"+uuidStr(), "", authHeaders(token))
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("ListUserCollections", func(t *testing.T) {
		w := doJSON(t, r, http.MethodGet, "/users/"+testUUID1+"/collections", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}
	})
}

// --- monitoring / client error ingest --------------------------------------

// TestClientErrorIngest covers the POST /errors endpoint contract: client-side
// crash reports (ErrorBoundary, unhandled rejections, 5xx API failures) are
// always acknowledged with 204, valid or not, so a broken client can never
// make reporting itself fail. The reporter here is nil (log-only in tests).
func TestClientErrorIngest(t *testing.T) {
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())

	t.Run("ValidReportIsAcknowledged", func(t *testing.T) {
		body := `{"message":"the map broke","stack":"at foo (bar.js:1:2)","url":"https://app.example/map","extra":{"kind":"react"}}`
		w := doJSON(t, r, http.MethodPost, "/errors", body, nil)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("MalformedPayloadIsAcknowledged", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/errors", "not-json", nil)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("EmptyMessageIsAcknowledged", func(t *testing.T) {
		w := doJSON(t, r, http.MethodPost, "/errors", `{"message":""}`, nil)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
		}
	})
}
