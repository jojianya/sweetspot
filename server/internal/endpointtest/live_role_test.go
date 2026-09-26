package endpointtest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

// Regression tests for the stale-role vulnerability: the role used to be baked
// into the JWT at login, so demoting a user left them authorized until their
// token expired (30 days). These assert that a role change takes effect on the
// caller's very next request, without re-issuing a token.

// commentOwnedBy builds a comment authored by userID.
func commentOwnedBy(userID string) comments.Comment {
	return comments.Comment{ID: uuidOf(testUUID3), PinID: uuidOf(testUUID1), UserID: uuidOf(userID), Body: "nice"}
}

// reset clears the call counters so a single test can assert on several
// independent requests.
func (m *mockCommentRepo) reset() {
	m.hideCalls, m.deleteCalls = 0, 0
}

// hiddenPinOwnedBy builds a hidden pin owned by ownerID, so the viewer is never
// the author and the only thing that can grant access is a moderation role.
func hiddenPinOwnedBy(ownerID string) pins.PinDetail {
	return pins.PinDetail{
		Pin:      pins.Pin{ID: uuidOf(testUUID3), UserID: uuidOf(ownerID), IsHidden: true},
		Photos:   []pins.PinPhoto{},
		Category: nil,
	}
}

// TestRoleChangeAppliesToExistingToken is the core regression. It is
// deliberately structured so the only variable between the three requests is
// the role stored in the user service: the token is minted once and never
// re-issued.
func TestRoleChangeAppliesToExistingToken(t *testing.T) {
	usersSvc := newUsersSvc()
	setRole(usersSvc, testUUID1, users.RoleUser)

	// The pin is owned by testUUID3, so testUUID1 is never its author.
	r, _ := setupFeaturesRouter(t, &stubPinRepo{pinDetail: hiddenPinOwnedBy(testUUID3)}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, usersSvc)
	tok := newToken(t, testUUID1)

	// Baseline: a hidden pin is invisible to a plain user.
	w := doJSON(t, r, http.MethodGet, "/pins/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusNotFound {
		t.Fatalf("plain user should get 404 for a hidden pin, got %d", w.Code)
	}

	// Promote in the database only.
	setRole(usersSvc, testUUID1, users.RoleAdmin)

	w = doJSON(t, r, http.MethodGet, "/pins/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusOK {
		t.Fatalf("promoted admin should see the hidden pin, got %d (%s)", w.Code, w.Body.String())
	}

	// Demote again, still without touching the token. This is the assertion the
	// old code failed: the role claim baked into the token still said "admin".
	setRole(usersSvc, testUUID1, users.RoleUser)

	w = doJSON(t, r, http.MethodGet, "/pins/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusNotFound {
		t.Fatalf("demoted admin should be denied again, got %d (%s) — role was read from the token", w.Code, w.Body.String())
	}
}

// TestPinAuthorStillSeesOwnHiddenPin guards the other side of canViewHidden:
// authorship must keep working when the caller is not a moderator at all. This
// would break if the fix had replaced the whole check with a role read.
func TestPinAuthorStillSeesOwnHiddenPin(t *testing.T) {
	usersSvc := newUsersSvc()
	setRole(usersSvc, testUUID1, users.RoleUser)

	r, _ := setupFeaturesRouter(t, &stubPinRepo{pinDetail: hiddenPinOwnedBy(testUUID1)}, &mockCommentRepo{}, &mockSocialRepo{}, &mockCollectionRepo{}, usersSvc)
	tok := newToken(t, testUUID1)

	w := doJSON(t, r, http.MethodGet, "/pins/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusOK {
		t.Fatalf("author should see their own hidden pin, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestCommentModerationFollowsLiveRole covers the comments path, which
// soft-deletes for moderators and hard-deletes for the author. The mock records
// which of the two ran, so this asserts the decision came from the database
// role rather than the token.
func TestCommentModerationFollowsLiveRole(t *testing.T) {
	usersSvc := newUsersSvc()
	setRole(usersSvc, testUUID1, users.RoleUser)

	// The comment belongs to testUUID3, so testUUID1 is never its author.
	commentRepo := &mockCommentRepo{get: commentOwnedBy(testUUID3)}
	r, _ := setupFeaturesRouter(t, &stubPinRepo{}, commentRepo, &mockSocialRepo{}, &mockCollectionRepo{}, usersSvc)
	tok := newToken(t, testUUID1)

	// Plain user: neither author nor moderator, so forbidden.
	w := doJSON(t, r, http.MethodDelete, "/comments/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusForbidden {
		t.Fatalf("plain user should get 403, got %d (%s)", w.Code, w.Body.String())
	}

	// Promoted: moderators soft-delete to keep an audit trail.
	setRole(usersSvc, testUUID1, users.RoleAdmin)
	commentRepo.reset()

	w = doJSON(t, r, http.MethodDelete, "/comments/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusNoContent {
		t.Fatalf("promoted admin should get 204, got %d (%s)", w.Code, w.Body.String())
	}
	if commentRepo.hideCalls != 1 {
		t.Fatalf("expected the moderator soft-delete path (1 hide), got %d", commentRepo.hideCalls)
	}

	// Demoted again, same token. Must fall back to the plain-user answer.
	setRole(usersSvc, testUUID1, users.RoleUser)
	commentRepo.reset()

	w = doJSON(t, r, http.MethodDelete, "/comments/"+testUUID3, "", authHeaders(tok))
	if w.Code != http.StatusForbidden {
		t.Fatalf("demoted admin should get 403 again, got %d (%s) — role was read from the token", w.Code, w.Body.String())
	}
	if commentRepo.hideCalls != 0 || commentRepo.deleteCalls != 0 {
		t.Fatalf("demoted caller reached a delete path: %d hides, %d deletes", commentRepo.hideCalls, commentRepo.deleteCalls)
	}
}

// TestTokenCarriesNoRoleClaim locks in the other half of the fix. If a role
// claim reappears in the JWT, every future caller is one refactor away from
// trusting it again — and this test fails instead.
func TestTokenCarriesNoRoleClaim(t *testing.T) {
	tok, err := jwt.Generate(testSecret, testUUID1, time.Hour)
	if err != nil {
		t.Fatalf("jwt.Generate: %v", err)
	}
	claims, err := jwt.Validate(testSecret, tok)
	if err != nil {
		t.Fatalf("jwt.Validate: %v", err)
	}
	if claims.UserID != testUUID1 {
		t.Fatalf("expected user_id %s, got %s", testUUID1, claims.UserID)
	}
	// Claims has no Role field, so a claim cannot reach the server through the
	// typed path even if the field were re-added. Assert on the wire format
	// too: a token that advertises "role" invites a future caller to trust it,
	// so the payload must not carry one.
	payload := decodePayload(t, tok)
	if v, present := payload["role"]; present {
		t.Fatalf("token must not advertise a role claim, got %v", v)
	}
	if payload["user_id"] != testUUID1 {
		t.Fatalf("expected user_id %s in payload, got %v", testUUID1, payload["user_id"])
	}
}

// decodePayload reads the claims segment of a JWT and returns it as a map,
// bypassing the Claims type so it can see fields the server does not model.
func decodePayload(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims segment: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	return payload
}
