package endpointtest

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// The list endpoints gained limit/offset. These cover the handler half — the
// default, the enforced maximum, and rejection of nonsense — against the
// recording stubs below, which echo the limit they were handed.

type recordingFavoriteRepo struct {
	mockFavoriteRepo
	lastLimit  int
	lastOffset int
}

func (r *recordingFavoriteRepo) List(_ context.Context, _ string, limit, offset int) ([]favorites.Entry, int, error) {
	r.lastLimit, r.lastOffset = limit, offset
	return []favorites.Entry{}, 0, nil
}

type recordingCommentRepo struct {
	mockCommentRepo
	lastLimit  int
	lastOffset int
}

func (r *recordingCommentRepo) ListByPin(_ context.Context, _ string, limit, offset int) ([]comments.Comment, int, error) {
	r.lastLimit, r.lastOffset = limit, offset
	return []comments.Comment{}, 0, nil
}

type recordingCollectionRepo struct {
	mockCollectionRepo
	lastLimit  int
	lastOffset int
}

func (r *recordingCollectionRepo) ListPins(_ context.Context, _ string, limit, offset int) ([]pins.PinListEntry, int, error) {
	r.lastLimit, r.lastOffset = limit, offset
	return []pins.PinListEntry{}, 0, nil
}

// The defaults and maxima are the ones each handler declares; they are
// unexported, so they are repeated here as literals and the mismatch is caught
// by the "at max" and "default" cases below.
const (
	favDefault, favMax         = 50, 200
	commentDefault, commentMax = 50, 200
	colPinsDefault, colPinsMax = 50, 200
)

func TestListEndpointsApplyPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		path       string
		favorites  bool // /favorites lives on the other router
		wantLimit  int
		wantOffset int
	}{
		// No params: the documented default applies to each endpoint.
		{"favorites default", "/favorites", true, favDefault, 0},
		{"comments default", "/pins/" + testUUID1 + "/comments", false, commentDefault, 0},
		{"collection pins default", "/collections/" + testUUID2, false, colPinsDefault, 0},
		// Explicit values pass through.
		{"favorites explicit", "/favorites?limit=5&offset=10", true, 5, 10},
		{"comments explicit", "/pins/" + testUUID1 + "/comments?limit=1&offset=3", false, 1, 3},
		{"collection pins explicit", "/collections/" + testUUID2 + "?limit=7&offset=2", false, 7, 2},
		// At the maximum: allowed, not clamped or rejected.
		{"favorites at max", "/favorites?limit=" + strconv.Itoa(favMax), true, favMax, 0},
		{"comments at max", "/pins/" + testUUID1 + "/comments?limit=" + strconv.Itoa(commentMax), false, commentMax, 0},
		{"collection pins at max", "/collections/" + testUUID2 + "?limit=" + strconv.Itoa(colPinsMax), false, colPinsMax, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fav := &recordingFavoriteRepo{}
			cmt := &recordingCommentRepo{}
			col := &recordingCollectionRepo{}

			// favorites and collections are not on setupFeaturesRouter, so
			// wire the two routers and query the one under test.
			fr, _ := setupFeaturesRouter(t, &stubPinRepo{}, cmt, &mockSocialRepo{}, col, newUsersSvc())
			r := fr
			if tc.favorites {
				r = setupRouter(newUsersSvc(), &mockReportRepo{}, fav, nil,
					storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)
			}

			w := doJSON(t, r, http.MethodGet, tc.path, "", authHeaders(newToken(t, testUUID1)))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
			}

			gotLimit, gotOffset := -1, -1
			switch {
			case fav.lastLimit > 0:
				gotLimit, gotOffset = fav.lastLimit, fav.lastOffset
			case cmt.lastLimit > 0:
				gotLimit, gotOffset = cmt.lastLimit, cmt.lastOffset
			case col.lastLimit > 0:
				gotLimit, gotOffset = col.lastLimit, col.lastOffset
			}
			if gotLimit != tc.wantLimit || gotOffset != tc.wantOffset {
				t.Errorf("limit/offset = %d/%d, want %d/%d", gotLimit, gotOffset, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestListEndpointsRejectInvalidPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	paths := []string{
		"/favorites",
		"/pins/" + testUUID1 + "/comments",
		"/collections/" + testUUID2,
	}
	bad := []string{
		"?limit=0",
		"?limit=-1",
		"?limit=abc",
		"?limit=" + strconv.Itoa(favMax+1), // one above the maximum
		"?offset=-1",
		"?offset=abc",
	}

	for _, path := range paths {
		isFavorites := path == "/favorites"
		for _, query := range bad {
			t.Run(path+query, func(t *testing.T) {
				fav := &recordingFavoriteRepo{}
				cmt := &recordingCommentRepo{}
				col := &recordingCollectionRepo{}
				fr, _ := setupFeaturesRouter(t, &stubPinRepo{}, cmt, &mockSocialRepo{}, col, newUsersSvc())
				r := fr
				if isFavorites {
					r = setupRouter(newUsersSvc(), &mockReportRepo{}, fav, nil,
						storage.NewLocal(t.TempDir(), "http://test.local"), auth.SameSiteStrict)
				}

				w := doJSON(t, r, http.MethodGet, path+query, "", authHeaders(newToken(t, testUUID1)))
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
				}
				if fav.lastLimit != 0 || cmt.lastLimit != 0 || col.lastLimit != 0 {
					t.Error("a rejected request must not reach the repository")
				}
			})
		}
	}
}
