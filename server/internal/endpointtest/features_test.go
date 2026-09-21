package endpointtest

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
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
	userExists    bool
	userExistsErr error
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
	r.Use(middleware.Recover(), middleware.SecurityHeaders())

	jsonRoutes := r.Group("")
	jsonRoutes.Use(middleware.BodyLimit(1 << 20))
	uploadRoutes := r.Group("")
	uploadRoutes.Use(middleware.BodyLimit(64 << 20))

	authSvc := auth.NewService(usersSvc, testSecret)
	authH := auth.NewHandler(authSvc, nil, middleware.New(1000, time.Minute))
	auth.RegisterRoutes(jsonRoutes, authH, auth.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	userH := users.NewHandler(usersSvc)
	users.RegisterRoutes(jsonRoutes, userH, users.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	store := storage.NewLocal(t.TempDir(), "http://test.local")
	pinH := pins.NewHandler(pins.NewService(pinRepo), store, nil)
	pins.RegisterRoutes(uploadRoutes, pinH, pins.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	commentH := comments.NewHandler(comments.NewService(commentRepo))
	comments.RegisterRoutes(jsonRoutes, commentH, comments.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	socialH := social.NewHandler(social.NewService(socialRepo))
	social.RegisterRoutes(jsonRoutes, socialH, social.RouteOptions{JWTSecret: testSecret, Blacklist: nil})

	colH := collections.NewHandler(collections.NewService(collectionRepo))
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
	r, _ := setupFeaturesRouter(t, &stubPinRepo{userPins: []pins.PinListEntry{}}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, newUsersSvc())
	w := doJSON(t, r, http.MethodGet, "/users/"+testUUID1+"/pins", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
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
