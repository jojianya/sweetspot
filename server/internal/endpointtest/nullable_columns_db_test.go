package endpointtest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
)

// Regression tests for fix/nullable-ids.
//
// pins.user_id, comments.user_id and reports.reporter_id are nullable
// (ON DELETE SET NULL in 0003_pins.sql / 0009_comments.sql / 0004_reports.sql).
// fix/model-types turned the model fields into plain strings, and pgx refuses
// to scan a NULL into *string, so any orphan row made list/get queries error
// with "cannot scan NULL into *string". Each test inserts a row that is NULL
// in that column and asserts the real query neither errors nor hides the row,
// and that the JSON renders the column as null (a nil *string marshals to
// null, so the API output for null stays null and for set values stays the
// same string).

// nullableBBox covers the orphan-test pins (created around 37.77 N, 122.43 W).
var nullableBBox = [4]float64{37.7, -122.5, 37.9, -122.3}

// assertJSONNull marshals v and requires key to render as JSON null.
func assertJSONNull(t *testing.T, label string, v any, key string) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", label, err)
	}
	if !strings.Contains(string(b), `"`+key+`":null`) {
		t.Fatalf("%s: %s did not render as null: %s", label, key, b)
	}
}

// seedNullUserPin inserts a visible pin whose user_id is NULL from the start,
// exactly as an author-deleted pin ends up (ON DELETE SET NULL), and returns
// its id.
func seedNullUserPin(t *testing.T, ctx context.Context, pool *pgxpool.Pool, catID int) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO pins (id, user_id, location, geohash, caption, category_id)
		VALUES (gen_random_uuid(), NULL, ST_SetSRID(ST_MakePoint(-122.43, 37.77), 4326)::geography, 'orb', 'orphan pin', $1)
		RETURNING id
	`, catID).Scan(&id)
	if err != nil {
		t.Fatalf("seed NULL-user pin: %v", err)
	}
	return id
}

// TestDBNullablePinsUserID covers the three pins queries that scan pins.user_id:
// ListPins, ListTrending and GetPin.
func TestDBNullablePinsUserID(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	catID := dbCategoryID(t, ctx, pool)
	pinID := seedNullUserPin(t, ctx, pool, catID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pins WHERE id = $1`, pinID)
	})

	repo := pins.NewRepository(pool)

	entries, err := repo.ListPins(ctx, nullableBBox, nil, 20)
	if err != nil {
		t.Fatalf("ListPins with NULL user_id: %v", err)
	}
	for i := range entries {
		if entries[i].ID == pinID {
			assertJSONNull(t, "ListPins", entries[i], "user_id")
			goto trending
		}
	}
	t.Fatalf("ListPins did not return the NULL-user pin %s", pinID)

trending:
	if _, err := repo.ListTrending(ctx, nullableBBox, 20); err != nil {
		t.Fatalf("ListTrending with NULL user_id: %v", err)
	}

	detail, err := repo.GetPin(ctx, pinID)
	if err != nil {
		t.Fatalf("GetPin with NULL user_id: %v", err)
	}
	assertJSONNull(t, "GetPin", detail, "user_id")
}

// TestDBNullableCommentsUserID covers comments.ListByPin and comments.Get:
// a comment whose author was deleted has a NULL user_id.
func TestDBNullableCommentsUserID(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	catID := dbCategoryID(t, ctx, pool)
	pinID := seedNullUserPin(t, ctx, pool, catID)

	var commentID string
	err := pool.QueryRow(ctx, `
		INSERT INTO comments (id, pin_id, user_id, body)
		VALUES (gen_random_uuid(), $1, NULL, 'orphan comment')
		RETURNING id
	`, pinID).Scan(&commentID)
	if err != nil {
		t.Fatalf("seed NULL-user comment: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pins WHERE id = $1`, pinID) // cascades comments
	})

	repo := comments.NewRepository(pool)

	cmts, total, err := repo.ListByPin(ctx, pinID, 20, 0)
	if err != nil {
		t.Fatalf("comments ListByPin with NULL user_id: %v", err)
	}
	if total < 1 {
		t.Fatalf("ListByPin total = %d, want >= 1", total)
	}
	for i := range cmts {
		if cmts[i].ID == commentID {
			assertJSONNull(t, "ListByPin", cmts[i], "user_id")
			goto get
		}
	}
	t.Fatalf("ListByPin did not return the NULL-user comment %s", commentID)

get:
	got, err := repo.Get(ctx, commentID)
	if err != nil {
		t.Fatalf("comments Get with NULL user_id: %v", err)
	}
	assertJSONNull(t, "Get", got, "user_id")
}

// TestDBNullableReportsReporterID covers reports.ListReports: a report whose
// reporter was deleted has a NULL reporter_id.
func TestDBNullableReportsReporterID(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	catID := dbCategoryID(t, ctx, pool)
	pinID := seedNullUserPin(t, ctx, pool, catID)

	var reportID string
	err := pool.QueryRow(ctx, `
		INSERT INTO reports (id, pin_id, reporter_id, reason)
		VALUES (gen_random_uuid(), $1, NULL, 'orphan report')
		RETURNING id
	`, pinID).Scan(&reportID)
	if err != nil {
		t.Fatalf("seed NULL-reporter report: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pins WHERE id = $1`, pinID) // cascades reports
	})

	reps, err := reports.NewRepository(pool).ListReports(ctx, nil, 20, 0)
	if err != nil {
		t.Fatalf("ListReports with NULL reporter_id: %v", err)
	}
	for i := range reps {
		if reps[i].ID == reportID {
			assertJSONNull(t, "ListReports", reps[i], "reporter_id")
			return
		}
	}
	t.Fatalf("ListReports did not return the NULL-reporter report %s", reportID)
}

// TestDBNullableOrphanPinInLists covers favorites.List and collections.ListPins:
// an orphan pin saved or collected by a real user still shows up, with
// user_id null, instead of erroring on the scan.
func TestDBNullableOrphanPinInLists(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	catID := dbCategoryID(t, ctx, pool)
	pinID := seedNullUserPin(t, ctx, pool, catID)
	userID := seedDBUserWithRole(t, ctx, pool, "orphan-fan-"+time.Now().Format("150405.000000000")+"@example.com", "user")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pins WHERE id = $1`, pinID) // cascades favorites, collection_pins
	})

	favs := favorites.NewRepository(pool)
	if err := favs.Save(ctx, userID, pinID); err != nil {
		t.Fatalf("save orphan pin: %v", err)
	}
	entries, total, err := favs.List(ctx, userID, 20, 0)
	if err != nil {
		t.Fatalf("favorites List with NULL pin user_id: %v", err)
	}
	if total < 1 {
		t.Fatalf("favorites total = %d, want >= 1", total)
	}
	for i := range entries {
		if entries[i].Pin.ID == pinID {
			assertJSONNull(t, "favorites.List", entries[i], "user_id")
			goto collections
		}
	}
	t.Fatalf("favorites.List did not return the orphan pin %s", pinID)

collections:
	colRepo := collections.NewRepository(pool)
	col, err := colRepo.Create(ctx, userID, "Orphan Pins", nil, false)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if err := colRepo.AddPin(ctx, col.ID, pinID); err != nil {
		t.Fatalf("add orphan pin to collection: %v", err)
	}
	cps, ctotal, err := colRepo.ListPins(ctx, col.ID, 20, 0)
	if err != nil {
		t.Fatalf("collections ListPins with NULL pin user_id: %v", err)
	}
	if ctotal < 1 {
		t.Fatalf("collections total = %d, want >= 1", ctotal)
	}
	for i := range cps {
		if cps[i].Pin.ID == pinID {
			assertJSONNull(t, "collections.ListPins", cps[i], "user_id")
			return
		}
	}
	t.Fatalf("collections ListPins did not return the orphan pin %s", pinID)
}