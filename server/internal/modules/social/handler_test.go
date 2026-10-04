package social

// Stub-repository characterization of the follow guard and the stats
// fan-out: self-follow rejection, missing targets, the conditional
// is-following lookup, and per-call error mapping.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

var errSocialBoom = errors.New("boom")

type stubSocialRepo struct {
	Repository

	exists       bool
	existsErr    error
	followErr    error
	unfollowErr  error
	followers    int
	following    int
	pinsCount    int
	countErr     error
	isFollowing  bool
	isFollowingE error

	followed   string
	unfollowed string
}

func (s *stubSocialRepo) UserExists(context.Context, string) (bool, error) {
	return s.exists, s.existsErr
}

func (s *stubSocialRepo) Follow(_ context.Context, _, target string) error {
	s.followed = target
	return s.followErr
}

func (s *stubSocialRepo) Unfollow(_ context.Context, _, target string) error {
	s.unfollowed = target
	return s.unfollowErr
}

func (s *stubSocialRepo) CountFollowers(context.Context, string) (int, error) {
	return s.followers, s.countErr
}

func (s *stubSocialRepo) CountFollowing(context.Context, string) (int, error) {
	return s.following, s.countErr
}

func (s *stubSocialRepo) CountPins(context.Context, string) (int, error) {
	return s.pinsCount, s.countErr
}

func (s *stubSocialRepo) IsFollowing(_ context.Context, _, _ string) (bool, error) {
	return s.isFollowing, s.isFollowingE
}

func newSocialHarness(repo *stubSocialRepo, viewer string) *gin.Engine {
	h := NewHandler(repo)
	r := gin.New()
	r.POST("/follow/:id", func(c *gin.Context) {
		c.Set("user_id", viewer)
		h.Follow(c)
	})
	r.DELETE("/follow/:id", func(c *gin.Context) {
		c.Set("user_id", viewer)
		h.Unfollow(c)
	})
	r.GET("/stats/:id", func(c *gin.Context) {
		c.Set("user_id", viewer)
		h.Stats(c)
	})
	return r
}

func socialError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	return body["error"]
}

func TestFollowGuard(t *testing.T) {
	cases := []struct {
		name   string
		method string
		target string
		exists bool
		status int
		msg    string
	}{
		{"follow self", http.MethodPost, "viewer-1", true, http.StatusBadRequest, "you cannot follow yourself"},
		{"unfollow self", http.MethodDelete, "viewer-1", true, http.StatusBadRequest, "you cannot follow yourself"},
		{"follow missing", http.MethodPost, "ghost", false, http.StatusNotFound, "user not found"},
		{"unfollow missing", http.MethodDelete, "ghost", false, http.StatusNotFound, "user not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newSocialHarness(&stubSocialRepo{exists: tc.exists}, "viewer-1")
			w := httptest.NewRecorder()
			route := "/follow/" + tc.target
			if tc.method == http.MethodDelete {
				r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, route, nil))
			} else {
				r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, route, nil))
			}
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.status, w.Body.String())
			}
			if msg := socialError(t, w); msg != tc.msg {
				t.Errorf("error = %q, want %q", msg, tc.msg)
			}
		})
	}
}

func TestFollowUnfollowSuccess(t *testing.T) {
	repo := &stubSocialRepo{exists: true}
	r := newSocialHarness(repo, "viewer-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/follow/other-1", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("follow status = %d, want 204", w.Code)
	}
	if repo.followed != "other-1" {
		t.Errorf("followed = %q, want other-1", repo.followed)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/follow/other-1", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("unfollow status = %d, want 204", w.Code)
	}
	if repo.unfollowed != "other-1" {
		t.Errorf("unfollowed = %q, want other-1", repo.unfollowed)
	}
}

func TestStatsFanOut(t *testing.T) {
	repo := &stubSocialRepo{exists: true, followers: 4, following: 2, pinsCount: 7, isFollowing: true}
	r := newSocialHarness(repo, "viewer-9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/other-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var stats map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats["followers"] != float64(4) || stats["following"] != float64(2) || stats["pins_count"] != float64(7) {
		t.Errorf("counts = %v, want 4/2/7", stats)
	}
	if stats["is_following"] != true {
		t.Errorf("is_following = %v, want true", stats["is_following"])
	}
}

func TestStatsSkipsFollowingCheckForSelfOrAnonymous(t *testing.T) {
	repo := &stubSocialRepo{exists: true, isFollowingE: errSocialBoom}
	r := newSocialHarness(repo, "other-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/other-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("self stats status = %d, want 200", w.Code)
	}

	r = newSocialHarness(&stubSocialRepo{exists: true, isFollowingE: errSocialBoom}, "")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/other-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("anonymous stats status = %d, want 200", w.Code)
	}
}

func TestStatsMissingUser(t *testing.T) {
	r := newSocialHarness(&stubSocialRepo{exists: false}, "viewer-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/ghost", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestStatsCountError(t *testing.T) {
	r := newSocialHarness(&stubSocialRepo{exists: true, countErr: errSocialBoom}, "viewer-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/other-1", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
