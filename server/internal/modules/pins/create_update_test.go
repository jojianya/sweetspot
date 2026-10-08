package pins

// Stub-repository characterization of CreatePin and UpdatePin: every
// validation branch with its exact status and message, plus orphan cleanup
// when a later step fails. Mirrors the delete_pin_test.go harness; the photo
// pipeline and storage are real (temp dir), only the repository is stubbed.

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// ptrString returns a pointer to s, for constructing model fields typed
// *string (pins.Pin.UserID is *string since fix/nullable-ids).
func ptrString(s string) *string { return &s }

var errTestBoom = errors.New("boom")

const testOwnerID = "123e4567-e89b-42d3-a456-426614174000"

func validJPEG(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../platform/imaging/testdata/gps-exif.jpg")
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}
	return raw
}

// multipartBody builds a multipart request with text fields and photo files.
// Each file entry is name -> content.
func multipartBody(t *testing.T, method, target string, fields map[string]string, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	for name, content := range files {
		fw, err := w.CreateFormFile("photos", name)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func newCreateHarness(t *testing.T, repo *stubRepo, dir string) *gin.Engine {
	t.Helper()
	h := NewHandler(repo, storage.NewLocal(dir, "http://api.test"), nil, nil)
	r := gin.New()
	r.POST("/pins", func(c *gin.Context) {
		c.Set("user_id", testOwnerID)
		h.CreatePin(c)
	})
	return r
}

func baseCreateRepo() *stubRepo {
	return &stubRepo{userExists: true, categoryExists: true}
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	return body["error"]
}

func TestCreatePinValidation(t *testing.T) {
	jpeg := validJPEG(t)
	big := make([]byte, 10<<20+1)
	cases := []struct {
		name   string
		fields map[string]string
		files  map[string][]byte
		status int
		msg    string
	}{
		{"lat not numeric", map[string]string{"lat": "abc"}, nil, http.StatusBadRequest, "lat must be a number"},
		{"lng out of range", map[string]string{"lat": "10", "lng": "200"}, nil, http.StatusBadRequest, "latitude or longitude out of range"},
		{"category not integer", map[string]string{"lat": "10", "lng": "10", "category_id": "x"}, nil, http.StatusBadRequest, "category_id must be an integer"},
		{"caption too long", map[string]string{"lat": "10", "lng": "10", "category_id": "1", "caption": strings.Repeat("x", 501)}, nil, http.StatusBadRequest, "caption must be at most 500 characters"},
		{"no photos", map[string]string{"lat": "10", "lng": "10", "category_id": "1"}, nil, http.StatusBadRequest, "at least one photo is required"},
		{"too many photos", map[string]string{"lat": "10", "lng": "10", "category_id": "1"},
			map[string][]byte{"1.jpg": jpeg, "2.jpg": jpeg, "3.jpg": jpeg, "4.jpg": jpeg, "5.jpg": jpeg, "6.jpg": jpeg},
			http.StatusBadRequest, "photo count exceeds maximum"},
		{"oversize photo", map[string]string{"lat": "10", "lng": "10", "category_id": "1"}, map[string][]byte{"big.jpg": big}, http.StatusBadRequest, "one or more photos exceed 10MB"},
		{"bad mime", map[string]string{"lat": "10", "lng": "10", "category_id": "1"}, map[string][]byte{"note.txt": []byte("hello")}, http.StatusBadRequest, "photo 1: only jpg and png images are allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newCreateHarness(t, baseCreateRepo(), t.TempDir())
			req := multipartBody(t, http.MethodPost, "/pins", tc.fields, tc.files)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.status, w.Body.String())
			}
			if msg := errorBody(t, w); msg != tc.msg {
				t.Errorf("error = %q, want %q", msg, tc.msg)
			}
		})
	}
}

func TestCreatePinRejectsNonMultipart(t *testing.T) {
	r := newCreateHarness(t, baseCreateRepo(), t.TempDir())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/pins", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if msg := errorBody(t, w); msg != "expected multipart form data" {
		t.Errorf("error = %q", msg)
	}
}

func TestCreatePinRejectsUnknownUser(t *testing.T) {
	repo := baseCreateRepo()
	repo.userExists = false
	r := newCreateHarness(t, repo, t.TempDir())
	// Fully valid fields: input validation passes, so the request reaches the
	// account check and the unknown user answers 401.
	req := multipartBody(t, http.MethodPost, "/pins",
		map[string]string{"lat": "10", "lng": "10", "category_id": "1"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestCreatePinRejectsUnknownCategory(t *testing.T) {
	repo := baseCreateRepo()
	repo.categoryExists = false
	r := newCreateHarness(t, repo, t.TempDir())
	req := multipartBody(t, http.MethodPost, "/pins",
		map[string]string{"lat": "10", "lng": "10", "category_id": "9"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if msg := errorBody(t, w); msg != "category not found" {
		t.Errorf("error = %q", msg)
	}
}

func TestCreatePinSuccess(t *testing.T) {
	repo := baseCreateRepo()
	r := newCreateHarness(t, repo, t.TempDir())
	req := multipartBody(t, http.MethodPost, "/pins",
		map[string]string{"lat": "10", "lng": "20", "category_id": "1", "caption": "hi"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}
	if !repo.created {
		t.Error("expected the pin to reach the repository")
	}
	var body struct {
		Photos []map[string]any `json:"photos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Photos) != 1 {
		t.Errorf("photos = %d, want 1", len(body.Photos))
	}
}

// TestCreatePinCleansUpFilesOnInsertFailure proves staged files are removed
// when the database insert fails after the photos hit disk.
func TestCreatePinCleansUpFilesOnInsertFailure(t *testing.T) {
	dir := t.TempDir()
	repo := baseCreateRepo()
	repo.createErr = errTestBoom
	r := newCreateHarness(t, repo, dir)
	req := multipartBody(t, http.MethodPost, "/pins",
		map[string]string{"lat": "10", "lng": "10", "category_id": "1"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d orphan files left on disk", len(entries))
	}
}

func newUpdateHarness(t *testing.T, repo *stubRepo, dir string, roles users.RoleReader) *gin.Engine {
	t.Helper()
	h := NewHandler(repo, storage.NewLocal(dir, "http://api.test"), nil, roles)
	r := gin.New()
	r.PATCH("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", testOwnerID)
		h.UpdatePin(c)
	})
	return r
}

func baseUpdateRepo() *stubRepo {
	return &stubRepo{
		detail:         PinDetail{Pin: Pin{UserID: ptrString(testOwnerID)}, Photos: []PinPhoto{}},
		categoryExists: true,
		visible:        true,
	}
}

func TestUpdatePinValidation(t *testing.T) {
	jpeg := validJPEG(t)
	cases := []struct {
		name   string
		fields map[string]string
		files  map[string][]byte
		status int
		msg    string
	}{
		{"caption too long", map[string]string{"caption": strings.Repeat("x", 501)}, nil, http.StatusBadRequest, "caption must be at most 500 characters"},
		{"category not integer", map[string]string{"caption": "ok", "category_id": "x"}, nil, http.StatusBadRequest, "category_id must be an integer"},
		{"too many photos", map[string]string{"caption": "ok"},
			map[string][]byte{"1.jpg": jpeg, "2.jpg": jpeg, "3.jpg": jpeg, "4.jpg": jpeg, "5.jpg": jpeg, "6.jpg": jpeg},
			http.StatusBadRequest, "photo count exceeds maximum"},
		{"bad photo", map[string]string{"caption": "ok"}, map[string][]byte{"note.txt": []byte("hello")}, http.StatusBadRequest, "photo 1: only jpg and png images are allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newUpdateHarness(t, baseUpdateRepo(), t.TempDir(), nil)
			req := multipartBody(t, http.MethodPatch, "/pins/pin-1", tc.fields, tc.files)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.status, w.Body.String())
			}
			if msg := errorBody(t, w); msg != tc.msg {
				t.Errorf("error = %q, want %q", msg, tc.msg)
			}
		})
	}
}

func TestUpdatePinNotFound(t *testing.T) {
	r := newUpdateHarness(t, &stubRepo{getErr: ErrNotFound}, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/gone", map[string]string{"caption": "x"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestUpdatePinForbiddenWithoutRoles(t *testing.T) {
	// UpdatePin must answer 403 for a non-owner even when no RoleReader is
	// wired, the way DeletePin does — never panic. The 403 now comes from the
	// repository's UPDATE predicate, reached with isModerator=false.
	r := newUpdateHarness(t, baseUpdateRepoOther(), t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1", map[string]string{"caption": "x"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestUpdatePinForbidden(t *testing.T) {
	// A wired RoleReader reporting a non-moderator must not widen access:
	// ownership is enforced by the UPDATE predicate, not by the handler.
	r := newUpdateHarness(t, baseUpdateRepoOther(), t.TempDir(), stubRoles{role: users.RoleUser})
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1", map[string]string{"caption": "x"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if msg := errorBody(t, w); msg != "you can only edit your own pins" {
		t.Errorf("error = %q", msg)
	}
}

func TestUpdatePinModeratorMayEditAnotherPin(t *testing.T) {
	// The mirror of TestUpdatePinForbidden: the predicate's moderator
	// alternative lets an admin edit a pin they do not own.
	r := newUpdateHarness(t, baseUpdateRepoOther(), t.TempDir(), stubRoles{role: users.RoleAdmin})
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1", map[string]string{"caption": "x"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestUpdatePinUnknownCategory(t *testing.T) {
	repo := baseUpdateRepo()
	repo.categoryExists = false
	r := newUpdateHarness(t, repo, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1",
		map[string]string{"caption": "x", "category_id": "9"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if msg := errorBody(t, w); msg != "category not found" {
		t.Errorf("error = %q", msg)
	}
}

func TestUpdatePinSuccessWithoutPhotos(t *testing.T) {
	repo := baseUpdateRepo()
	r := newUpdateHarness(t, repo, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1",
		map[string]string{"caption": "new", "category_id": "2"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if !repo.updated {
		t.Error("expected the patch to reach the repository")
	}
	if repo.updatePhotos != 0 {
		t.Errorf("photos replaced = %d, want 0", repo.updatePhotos)
	}
}

// TestUpdatePinPhotosOnlyKeepsCaption proves an absent caption field is not
// conflated with an explicit empty caption, which would clear the pin text.
func TestUpdatePinPhotosOnlyKeepsCaption(t *testing.T) {
	repo := baseUpdateRepo()
	r := newUpdateHarness(t, repo, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1", map[string]string{}, map[string][]byte{"new.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if repo.lastPatch.Caption != nil {
		t.Fatalf("caption patch = %v, want nil when field is absent", *repo.lastPatch.Caption)
	}
}

// TestUpdatePinCleansUpNewFilesOnFailure proves staged replacements are
// removed when the database update fails after the photos hit disk.
func TestUpdatePinCleansUpNewFilesOnFailure(t *testing.T) {
	dir := t.TempDir()
	repo := baseUpdateRepo()
	repo.updateErr = errTestBoom
	r := newUpdateHarness(t, repo, dir, nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1",
		map[string]string{"caption": "new"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d orphan files left on disk", len(entries))
	}
}

// TestUpdatePinSweepsStalePhotosOnSuccess proves the replaced photo set is
// deleted from disk once the swap succeeds.
func TestUpdatePinSweepsStalePhotosOnSuccess(t *testing.T) {
	dir := t.TempDir()
	photoURL := stagePhoto(t, dir, "stale-full.webp")
	thumbURL := stagePhoto(t, dir, "stale-thumb.webp")
	repo := baseUpdateRepo()
	repo.detail.Photos = []PinPhoto{{PhotoURL: photoURL, ThumbnailURL: thumbURL}}
	r := newUpdateHarness(t, repo, dir, nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1",
		map[string]string{"caption": "new"},
		map[string][]byte{"a.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	for _, name := range []string{"stale-full.webp", "stale-thumb.webp"} {
		if onDisk(t, dir, name) {
			t.Errorf("%s still on disk after a successful photo swap", name)
		}
	}
}

// TestUpdatePinReturnsPhotos proves the response carries the pin's photo set
// after the update, so a client that replaced its photos does not have to
// refetch the pin to learn the new URLs. CreatePin answers the same way.
func TestUpdatePinReturnsPhotos(t *testing.T) {
	repo := baseUpdateRepo()
	repo.updateResultPhotos = []PinPhoto{
		{PhotoURL: "http://api.test/new-full.webp", ThumbnailURL: "http://api.test/new-thumb.webp", Position: 0},
		{PhotoURL: "http://api.test/new2-full.webp", ThumbnailURL: "http://api.test/new2-thumb.webp", Position: 1},
	}
	r := newUpdateHarness(t, repo, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1",
		map[string]string{"caption": "new"},
		map[string][]byte{"a.jpg": validJPEG(t), "b.jpg": validJPEG(t)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var body struct {
		Pin    Pin              `json:"pin"`
		Photos []map[string]any `json:"photos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Photos) != 2 {
		t.Fatalf("photos = %d, want 2 (body: %s)", len(body.Photos), w.Body.String())
	}
	for i, ph := range body.Photos {
		if ph["photo_url"] != repo.updateResultPhotos[i].PhotoURL {
			t.Errorf("photo %d url = %v, want %q", i, ph["photo_url"], repo.updateResultPhotos[i].PhotoURL)
		}
		if ph["thumbnail_url"] != repo.updateResultPhotos[i].ThumbnailURL {
			t.Errorf("photo %d thumbnail = %v, want %q", i, ph["thumbnail_url"], repo.updateResultPhotos[i].ThumbnailURL)
		}
	}
}

// TestUpdatePinHiddenPinIsNotFound proves a soft-hidden pin cannot be edited:
// the shared VisiblePinExists rule makes it a 404, not a silent success. The
// UPDATE predicate repeats the rule, and TestDBUpdatePinAuthorization covers
// that second gate against real SQL.
func TestUpdatePinHiddenPinIsNotFound(t *testing.T) {
	repo := baseUpdateRepo()
	repo.visible = false
	r := newUpdateHarness(t, repo, t.TempDir(), nil)
	req := multipartBody(t, http.MethodPatch, "/pins/pin-1", map[string]string{"caption": "new"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
	if repo.updated {
		t.Error("a hidden pin must not reach the repository update")
	}
}

func baseUpdateRepoOther() *stubRepo {
	return &stubRepo{
		detail:         PinDetail{Pin: Pin{UserID: ptrString("223e4567-e89b-42d3-a456-426614174000")}, Photos: []PinPhoto{}},
		categoryExists: true,
		visible:        true,
	}
}
