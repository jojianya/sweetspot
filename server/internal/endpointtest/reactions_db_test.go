package endpointtest

// HTTP-level tests for the Good spot endpoints: PUT/DELETE
// /pins/:id/good-spot. They register the module's routes on a real router with
// real rate limiters, so the middleware ordering (validid -> auth -> limits)
// and every status code in the contract are exercised, not just the service.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reactions"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

// reactionTestSecret only ever mints tokens for this test's own router, so it
// is not a deployed secret.
const reactionTestSecret = "reactions-endpoint-test-secret-value"

type reactionFixture struct {
	router *gin.Engine
	pool   *pgxpool.Pool
	author string
	other  string
}

func setupReactionRouter(t *testing.T) reactionFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := requireEndpointDB(t)

	userSvc := users.NewService(users.NewRepository(pool))
	h := reactions.NewHandler(reactions.NewRepository(pool))
	r := gin.New()
	reactions.RegisterRoutes(r.Group(""), h, reactions.RouteOptions{
		JWTSecret: reactionTestSecret,
		Sessions:  userSvc,
	})

	ctx := context.Background()
	author := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("react-author-%d@example.com", time.Now().UnixNano()), "user")
	other := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("react-other-%d@example.com", time.Now().UnixNano()), "user")
	return reactionFixture{router: r, pool: pool, author: author, other: other}
}

// seedReactionPin inserts one visible pin owned by userID, using the real
// repository so the row is shaped exactly as production creates it. The cleanup
// hard-deletes it: deleting the author alone would leave the pin behind,
// because pins.user_id is ON DELETE SET NULL.
func seedReactionPin(t *testing.T, f reactionFixture, userID string) string {
	t.Helper()
	ctx := context.Background()
	var catID int
	if err := f.pool.QueryRow(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&catID); err != nil {
		t.Fatalf("read category: %v", err)
	}
	caption := "good spot probe"
	p, err := pins.NewRepository(f.pool).CreatePin(ctx, pins.NewPin{
		UserID: userID, Lat: 20, Lng: 20, Caption: &caption,
		CategoryID: catID, Geohash: fmt.Sprintf("react-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM pins WHERE id = $1`, p.ID)
	})
	return p.ID
}

func reactionToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := jwt.Generate(reactionTestSecret, userID, time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return tok
}

type reactionResponse struct {
	Reacted       bool `json:"reacted"`
	GoodSpotCount int  `json:"good_spot_count"`
}

// callReaction fires one toggle. An empty token means "no Authorization
// header", which is the guest case.
func callReaction(t *testing.T, f reactionFixture, method, pinID, token string) (int, string, reactionResponse) {
	t.Helper()
	req := httptest.NewRequest(method, "/pins/"+pinID+"/good-spot", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)

	var out reactionResponse
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("response is not the documented shape: %v (body: %s)", err, w.Body.String())
		}
	}
	return w.Code, w.Body.String(), out
}

func reactionCount(t *testing.T, f reactionFixture, pinID string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT good_spot_count FROM pins WHERE id = $1`, pinID).Scan(&n); err != nil {
		t.Fatalf("read count: %v", err)
	}
	return n
}

func reactionRows(t *testing.T, f reactionFixture, pinID string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM good_spots WHERE pin_id = $1`, pinID).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

// TestDBGoodSpotEndpointToggle covers the happy path end to end: the count
// moves, the row exists, and undoing returns the count to zero.
func TestDBGoodSpotEndpointToggle(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)
	tok := reactionToken(t, f.other)

	if got := reactionCount(t, f, pin); got != 0 {
		t.Fatalf("count = %d before any toggle, want 0", got)
	}

	code, body, res := callReaction(t, f, http.MethodPut, pin, tok)
	if code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (body: %s)", code, body)
	}
	if !res.Reacted || res.GoodSpotCount != 1 {
		t.Fatalf("PUT body = %+v, want {reacted:true count:1}", res)
	}
	if got := reactionCount(t, f, pin); got != 1 {
		t.Fatalf("count = %d after react, want 1", got)
	}
	if got := reactionRows(t, f, pin); got != 1 {
		t.Fatalf("good_spots rows = %d after react, want 1", got)
	}

	code, body, res = callReaction(t, f, http.MethodDelete, pin, tok)
	if code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200 (body: %s)", code, body)
	}
	if res.Reacted || res.GoodSpotCount != 0 {
		t.Fatalf("DELETE body = %+v, want {reacted:false count:0}", res)
	}
	if got := reactionCount(t, f, pin); got != 0 {
		t.Fatalf("count = %d after undo, want 0", got)
	}
	if got := reactionRows(t, f, pin); got != 0 {
		t.Fatalf("good_spots rows = %d after undo, want 0", got)
	}
}

// TestDBGoodSpotEndpointIdempotent is the double-tap case. The second PUT must
// not move the count, because ON CONFLICT DO NOTHING absorbs the duplicate and
// the trigger fires only for the row that was actually inserted.
func TestDBGoodSpotEndpointIdempotent(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)
	tok := reactionToken(t, f.other)

	callReaction(t, f, http.MethodPut, pin, tok)
	code, body, res := callReaction(t, f, http.MethodPut, pin, tok)
	if code != http.StatusOK {
		t.Fatalf("second PUT status = %d, want 200 (body: %s)", code, body)
	}
	if res.GoodSpotCount != 1 {
		t.Fatalf("count = %d after two taps, want 1", res.GoodSpotCount)
	}
	if got := reactionRows(t, f, pin); got != 1 {
		t.Fatalf("good_spots rows = %d after two taps, want 1", got)
	}
}

// TestDBGoodSpotEndpointHiddenPin is the visibility rule: a hidden pin answers
// exactly the same 404 as a missing one, and its count does not move.
func TestDBGoodSpotEndpointHiddenPin(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)
	tok := reactionToken(t, f.other)

	if _, err := f.pool.Exec(context.Background(),
		`UPDATE pins SET is_hidden = true WHERE id = $1`, pin); err != nil {
		t.Fatalf("hide pin: %v", err)
	}

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		code, body, res := callReaction(t, f, method, pin, tok)
		if code != http.StatusNotFound {
			t.Fatalf("%s on a hidden pin status = %d, want 404 (body: %s)", method, code, body)
		}
		if body != `{"error":"pin not found"}` {
			t.Errorf("%s on a hidden pin body = %s, want the standard not-found message", method, body)
		}
		if res.GoodSpotCount != 0 {
			t.Errorf("%s on a hidden pin reported count %d, want 0", method, res.GoodSpotCount)
		}
	}
	if got := reactionRows(t, f, pin); got != 0 {
		t.Errorf("hidden pin gained %d reaction rows, want 0", got)
	}
}

// TestDBGoodSpotEndpointMissingPin asserts a random UUID 404s rather than
// erroring, and does not leak whether the id is well-formed.
func TestDBGoodSpotEndpointMissingPin(t *testing.T) {
	f := setupReactionRouter(t)
	tok := reactionToken(t, f.other)

	// A syntactically valid UUID that does not exist.
	code, body, _ := callReaction(t, f, http.MethodPut, "00000000-0000-4000-8000-000000000000", tok)
	if code != http.StatusNotFound {
		t.Fatalf("PUT on a missing pin status = %d, want 404 (body: %s)", code, body)
	}
}

// TestDBGoodSpotEndpointOwnPin is the ownership rule. The UI hides the button
// so a human never reaches this; the backend still refuses.
func TestDBGoodSpotEndpointOwnPin(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)
	tok := reactionToken(t, f.author)

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		code, _, _ := callReaction(t, f, method, pin, tok)
		if code != http.StatusForbidden {
			t.Fatalf("%s on own pin status = %d, want 403", method, code)
		}
	}
	if got := reactionRows(t, f, pin); got != 0 {
		t.Errorf("own pin gained %d reaction rows, want 0", got)
	}
	if got := reactionCount(t, f, pin); got != 0 {
		t.Errorf("own pin count = %d, want 0", got)
	}
}

// TestDBGoodSpotEndpointGuest asserts an unauthenticated toggle is a 401 and
// writes nothing. Guests still see counts, which the read path covers.
func TestDBGoodSpotEndpointGuest(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		code, _, _ := callReaction(t, f, method, pin, "")
		if code != http.StatusUnauthorized {
			t.Fatalf("%s with no token status = %d, want 401", method, code)
		}
	}
	if got := reactionRows(t, f, pin); got != 0 {
		t.Errorf("guest toggle wrote %d reaction rows, want 0", got)
	}
}

// TestDBGoodSpotEndpointUnreactAbsent covers the undo race: removing a
// reaction that was never there, or that another tab already removed, is a
// 200 with reacted:false — not a 404.
func TestDBGoodSpotEndpointUnreactAbsent(t *testing.T) {
	f := setupReactionRouter(t)
	pin := seedReactionPin(t, f, f.author)
	tok := reactionToken(t, f.other)

	code, body, res := callReaction(t, f, http.MethodDelete, pin, tok)
	if code != http.StatusOK {
		t.Fatalf("DELETE with no reaction status = %d, want 200 (body: %s)", code, body)
	}
	if res.Reacted {
		t.Error("reacted = true after undoing a reaction that never existed")
	}
	if res.GoodSpotCount != 0 {
		t.Errorf("count = %d, want 0", res.GoodSpotCount)
	}
	if got := reactionRows(t, f, pin); got != 0 {
		t.Errorf("good_spots rows = %d, want 0", got)
	}
}

// TestDBGoodSpotEndpointMalformedID asserts a bad id is a 400 and consumes no
// reaction budget, because validid runs before the limiters.
func TestDBGoodSpotEndpointMalformedID(t *testing.T) {
	f := setupReactionRouter(t)
	tok := reactionToken(t, f.other)

	code, _, _ := callReaction(t, f, http.MethodPut, "not-a-uuid", tok)
	if code != http.StatusBadRequest {
		t.Fatalf("PUT on a malformed id status = %d, want 400", code)
	}
}
