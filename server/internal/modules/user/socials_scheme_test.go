package users

// Server-side half of the social-link scheme rule. The client already
// neutralises dangerous hrefs (client/src/lib/utils/profile.ts:37 prefixes
// https:// for anything that is not http(s)); these tests pin the server so a
// stored value is safe regardless of which client reads it back.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// socialsResult is one UpdateMe round trip: the status, the error message
// (empty on success), and the socials patch that reached the service.
type socialsResult struct {
	code  int
	msg   string
	patch map[string]any
}

// socialsSave performs a profile update carrying only `socials`.
//
// It reads the raw response rather than reusing updateMeError, whose
// map[string]string decode cannot hold the 200 body's nested socials object.
func socialsSave(t *testing.T, raw string) socialsResult {
	t.Helper()
	svc := baseUserService()
	r := newUpdateMeHarness(svc, t.TempDir())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, updateMeRequest(t, map[string]string{"socials": raw}, nil))

	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (body: %s)", err, w.Body.String())
	}
	out := socialsResult{code: w.Code}
	if rawErr, ok := body["error"]; ok {
		var s string
		if err := json.Unmarshal(rawErr, &s); err == nil {
			out.msg = s
		}
	}
	if svc.updatedPatch.Socials != nil {
		out.patch = *svc.updatedPatch.Socials
	}
	return out
}

func TestParseSocialsAcceptsKnownGoodValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{
			name: "plain handles",
			raw:  `{"instagram":"alice","twitter":"@alice"}`,
			want: map[string]any{"instagram": "alice", "twitter": "@alice"},
		},
		{
			name: "https website",
			raw:  `{"website":"https://alice.example/a"}`,
			want: map[string]any{"website": "https://alice.example/a"},
		},
		{
			name: "http website",
			raw:  `{"website":"http://plain.example"}`,
			want: map[string]any{"website": "http://plain.example"},
		},
		{
			name: "bare host website",
			raw:  `{"website":"alice.example"}`,
			want: map[string]any{"website": "alice.example"},
		},
		{
			// A colon inside a handle is not a scheme, and profile.ts builds
			// "https://instagram.com/alice:example" from it — inert, so it is
			// not this rule's business to reject.
			name: "bare host with a port",
			raw:  `{"website":"alice.example:8080"}`,
			want: map[string]any{"website": "alice.example:8080"},
		},
		{
			name: "https with a port and path",
			raw:  `{"website":"https://alice.example:8080/a"}`,
			want: map[string]any{"website": "https://alice.example:8080/a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := socialsSave(t, tc.raw)
			if got.code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (error: %s)", got.code, got.msg)
			}
			for k, v := range tc.want {
				if got.patch[k] != v {
					t.Errorf("socials[%q] = %#v, want %#v", k, got.patch[k], v)
				}
			}
		})
	}
}

func TestParseSocialsRejectsDangerousSchemes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"javascript", `{"website":"javascript:alert(1)"}`, "javascript"},
		{"data", `{"website":"data:text/html,<script>alert(1)</script>"}`, "data"},
		{"vbscript", `{"website":"vbscript:msgbox(1)"}`, "vbscript"},
		{"file", `{"website":"file:///etc/passwd"}`, "file"},
		{"mixed case", `{"website":"JaVaScRiPt:alert(1)"}`, "javascript"},
		{"upper case data", `{"website":"DATA:text/html,x"}`, "data"},
		{"instagram handle with scheme", `{"instagram":"javascript:alert(1)"}`, "javascript"},
		{"twitter handle with scheme", `{"twitter":"javascript:alert(1)"}`, "javascript"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := socialsSave(t, tc.raw)
			if got.code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", got.code)
			}
			if got.patch != nil {
				t.Errorf("a rejected payload reached the service: %#v", got.patch)
			}
			if !strings.Contains(got.msg, tc.want) {
				t.Errorf("error = %q, want it to name the %q scheme", got.msg, tc.want)
			}
			// The message must never reflect the rejected value itself.
			if strings.Contains(got.msg, "alert(1)") || strings.Contains(got.msg, "<script>") {
				t.Errorf("error reflects the rejected payload: %q", got.msg)
			}
		})
	}
}

// TestParseSocialsLeavesUnknownKeysAlone pins the passthrough: only the three
// rendered keys are scheme-checked, so an unrelated key never blocks a save.
func TestParseSocialsLeavesUnknownKeysAlone(t *testing.T) {
	got := socialsSave(t, `{"anything":"javascript:alert(1)"}`)
	if got.code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (error: %s)", got.code, got.msg)
	}
	if got.patch["anything"] != "javascript:alert(1)" {
		t.Errorf("unknown key was altered: %#v", got.patch)
	}
}

// TestUpdateMeKeepsExistingValidProfileSaving is the back-compat case. The
// client sends the whole socials object on every save
// (client/src/lib/utils/profile.ts: socialsChanged spreads `current`), and the
// repository replaces the column wholesale, so an already-stored valid profile
// must keep saving unchanged.
func TestUpdateMeKeepsExistingValidProfileSaving(t *testing.T) {
	svc := baseUserService()
	svc.current = User{
		ID: "user-1", Username: "alice",
		Socials: map[string]any{
			"instagram": "alice",
			"twitter":   "@alice",
			"website":   "https://alice.example",
			"extra":     "kept",
		},
	}
	r := newUpdateMeHarness(svc, t.TempDir())
	// The full object, exactly as socialsChanged would send it.
	raw := `{"instagram":"alice","twitter":"@alice","website":"https://alice.example","extra":"kept"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, updateMeRequest(t, map[string]string{"socials": raw}, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if svc.updatedPatch.Socials == nil {
		t.Fatal("socials patch was nil, want the stored profile")
	}
	got := *svc.updatedPatch.Socials
	for k, v := range svc.current.Socials {
		if got[k] != v {
			t.Errorf("socials[%q] = %#v, want %#v", k, got[k], v)
		}
	}
}

// TestUpdateMeSocialsSchemeRejected covers the 400 the handler actually
// returns, not just the parser's error.
func TestUpdateMeSocialsSchemeRejected(t *testing.T) {
	got := socialsSave(t, `{"website":"javascript:alert(1)"}`)
	if got.code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", got.code)
	}
	if !strings.Contains(got.msg, "javascript") {
		t.Errorf("error = %q, want it to name the javascript scheme", got.msg)
	}
}
