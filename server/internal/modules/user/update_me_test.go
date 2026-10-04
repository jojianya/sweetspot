package users

// Stub-service characterization of UpdateMe: username rules, the socials
// parser bounds, avatar replacement and rollback on failure. Imaging and
// storage are real (temp dir); only the service is stubbed.

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

func init() { gin.SetMode(gin.TestMode) }

type stubUserService struct {
	Service

	current User
	getErr  error

	updatedPatch UpdateProfilePatch
	updateResult User
	updateErr    error
}

func (s *stubUserService) GetByID(context.Context, string) (User, error) {
	return s.current, s.getErr
}

func (s *stubUserService) UpdateProfile(_ context.Context, _ string, patch UpdateProfilePatch) (User, error) {
	s.updatedPatch = patch
	if s.updateErr != nil {
		return User{}, s.updateErr
	}
	if s.updateResult.ID != "" {
		return s.updateResult, nil
	}
	return s.current, nil
}

func validAvatar(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../platform/imaging/testdata/gps-exif.jpg")
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}
	return raw
}

func updateMeRequest(t *testing.T, fields map[string]string, avatar []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if avatar != nil {
		fw, err := w.CreateFormFile("avatar", "avatar.jpg")
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := fw.Write(avatar); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/users/me", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func newUpdateMeHarness(svc *stubUserService, dir string) *gin.Engine {
	h := NewHandler(svc, storage.NewLocal(dir, "http://api.test"))
	r := gin.New()
	r.PATCH("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.UpdateMe(c)
	})
	return r
}

func baseUserService() *stubUserService {
	return &stubUserService{
		current:      User{ID: "user-1", Username: "alice", Socials: map[string]any{}},
		updateResult: User{ID: "user-1", Username: "alice", Socials: map[string]any{}},
	}
}

func updateMeError(t *testing.T, r *gin.Engine, req *http.Request) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	return w.Code, body["error"]
}

func TestUpdateMeValidation(t *testing.T) {
	bigAvatar := make([]byte, (5<<20)+1)
	cases := []struct {
		name   string
		fields map[string]string
		avatar []byte
		status int
		msg    string
	}{
		{"empty username", map[string]string{"username": "  "}, nil, http.StatusBadRequest, "username cannot be empty"},
		{"short username", map[string]string{"username": "ab"}, nil, http.StatusBadRequest, "username must be between 3 and 30 characters"},
		{"long username", map[string]string{"username": strings.Repeat("x", 31)}, nil, http.StatusBadRequest, "username must be between 3 and 30 characters"},
		{"socials not JSON", map[string]string{"socials": "nope"}, nil, http.StatusBadRequest, "socials must be a JSON object"},
		{"socials not object", map[string]string{"socials": "[1,2]"}, nil, http.StatusBadRequest, "socials must be a JSON object"},
		{"avatar too big", map[string]string{"username": "alice2"}, bigAvatar, http.StatusBadRequest, "avatar exceeds 5MB"},
		{"avatar bad mime", map[string]string{"username": "alice2"}, []byte("not an image"), http.StatusBadRequest, "only jpg and png images are allowed"},
		{"nothing to update", map[string]string{}, nil, http.StatusBadRequest, "nothing to update"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newUpdateMeHarness(baseUserService(), t.TempDir())
			code, msg := updateMeError(t, r, updateMeRequest(t, tc.fields, tc.avatar))
			if code != tc.status {
				t.Fatalf("status = %d, want %d (error: %s)", code, tc.status, msg)
			}
			if tc.msg != "" && msg != tc.msg {
				t.Errorf("error = %q, want %q", msg, tc.msg)
			}
			if tc.msg == "" && msg == "" {
				t.Errorf("expected a non-empty error message")
			}
		})
	}

	t.Run("socials too many keys", func(t *testing.T) {
		var keys []string
		for i := 0; i < 21; i++ {
			keys = append(keys, `"k`+strings.Repeat("x", i)+`":"v"`)
		}
		r := newUpdateMeHarness(baseUserService(), t.TempDir())
		code, msg := updateMeError(t, r, updateMeRequest(t,
			map[string]string{"socials": "{" + strings.Join(keys, ",") + "}"}, nil))
		if code != http.StatusBadRequest || msg != "too many socials (max 20)" {
			t.Errorf("status = %d, error = %q", code, msg)
		}
	})
}

func TestUpdateMeSocialsKeyBounds(t *testing.T) {
	longKey := strings.Repeat("k", 65)
	r := newUpdateMeHarness(baseUserService(), t.TempDir())
	code, msg := updateMeError(t, r, updateMeRequest(t,
		map[string]string{"socials": `{"` + longKey + `":"v"}`}, nil))
	if code != http.StatusBadRequest || msg != "social name too long" {
		t.Errorf("status = %d, error = %q", code, msg)
	}

	badVal := updateMeRequest(t, map[string]string{"socials": `{"a":[1]}`}, nil)
	code, msg = updateMeError(t, newUpdateMeHarness(baseUserService(), t.TempDir()), badVal)
	if code != http.StatusBadRequest || msg != "socials values must be strings, booleans, or numbers" {
		t.Errorf("status = %d, error = %q", code, msg)
	}

	longVal := updateMeRequest(t,
		map[string]string{"socials": `{"a":"` + strings.Repeat("v", 501) + `"}`}, nil)
	code, msg = updateMeError(t, newUpdateMeHarness(baseUserService(), t.TempDir()), longVal)
	if code != http.StatusBadRequest || msg != "social value too long" {
		t.Errorf("status = %d, error = %q", code, msg)
	}
}

func TestUpdateMeUsernameTakenRollsBackAvatar(t *testing.T) {
	dir := t.TempDir()
	svc := baseUserService()
	svc.updateErr = ErrUsernameTaken
	r := newUpdateMeHarness(svc, dir)
	code, msg := updateMeError(t, r, updateMeRequest(t,
		map[string]string{"username": "taken"}, validAvatar(t)))
	if code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (error: %s)", code, msg)
	}
	if msg != "username already taken" {
		t.Errorf("error = %q", msg)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d orphan avatar files left on disk", len(entries))
	}
}

func TestUpdateMeSuccess(t *testing.T) {
	svc := baseUserService()
	r := newUpdateMeHarness(svc, t.TempDir())
	req := updateMeRequest(t, map[string]string{"username": "alice2"}, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if svc.updatedPatch.Username == nil || *svc.updatedPatch.Username != "alice2" {
		t.Errorf("username patch = %+v", svc.updatedPatch.Username)
	}
}
