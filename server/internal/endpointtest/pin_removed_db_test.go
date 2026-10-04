package endpointtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
)

// seedDBReport inserts a pending report for pinID by reporterID and returns
// the report id. Cleanup removes the row.
func seedDBReport(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pinID, reporterID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO reports (id, pin_id, reporter_id, reason, status)
		VALUES (gen_random_uuid(), $1, $2, 'spam', 'pending')
		RETURNING id
	`, pinID, reporterID).Scan(&id)
	if err != nil {
		t.Fatalf("seed report: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reports WHERE id = $1`, id)
	})
	return id
}

// TestDBReviewApproveReturnsLocationAfterCommit proves the publish-after-commit
// wiring at the DB layer: ReviewReport commits the hide + resolution in one
// transaction and only then returns the pin location the handler publishes.
// The returned location is the data the pin_removed event carries.
func TestDBReviewApproveReturnsLocationAfterCommit(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	owner := seedDBUser(t, ctx, pool, "user")
	reporter := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, owner, dbCategoryID(t, ctx, pool))
	reportID := seedDBReport(t, ctx, pool, pinID, reporter)

	repo := reports.NewRepository(pool)
	rep, loc, err := repo.ReviewReport(ctx, reportID, "approve", owner)
	if err != nil {
		t.Fatalf("ReviewReport approve: %v", err)
	}
	if loc == nil || *loc == "" {
		t.Fatalf("expected non-empty pin location after commit, got %v", loc)
	}
	// The payload must parse with exactly the logic realtime.matchesRemoved
	// uses for bbox filtering — WKT "POINT(lng lat)", not raw EWKB hex.
	var lng, lat float64
	if _, err := fmt.Sscanf(*loc, "POINT(%f %f)", &lng, &lat); err != nil {
		t.Fatalf("pin_removed location %q does not parse like matchesRemoved expects: %v", *loc, err)
	}
	if rep.Status != reports.StatusActioned {
		t.Errorf("expected status actioned, got %q", rep.Status)
	}
	if !dbIsHidden(t, ctx, pool, pinID) {
		t.Error("expected pin to be hidden after approve commit")
	}
}

// TestDBReviewDismissReturnsNilLocation proves a dismiss resolution carries
// no removal payload, so the handler publishes nothing.
func TestDBReviewDismissReturnsNilLocation(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	owner := seedDBUser(t, ctx, pool, "user")
	reporter := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, owner, dbCategoryID(t, ctx, pool))
	reportID := seedDBReport(t, ctx, pool, pinID, reporter)

	repo := reports.NewRepository(pool)
	rep, loc, err := repo.ReviewReport(ctx, reportID, "dismiss", owner)
	if err != nil {
		t.Fatalf("ReviewReport dismiss: %v", err)
	}
	if loc != nil {
		t.Errorf("expected nil location on dismiss (no removal), got %v", *loc)
	}
	if rep.Status != reports.StatusReviewed {
		t.Errorf("expected status reviewed, got %q", rep.Status)
	}
	if dbIsHidden(t, ctx, pool, pinID) {
		t.Error("expected pin to stay visible after dismiss")
	}
}

// TestDBReviewUpdateErrorSurfaces covers the UPDATE-reports error path in
// ReviewReport: resolving with a nonexistent resolved_by UUID violates the
// reports.resolved_by foreign key, so the UPDATE fails and the error must
// surface (previously the stale outer err was checked instead of the UPDATE
// result, silently swallowing the failure and committing).
func TestDBReviewUpdateErrorSurfaces(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	owner := seedDBUser(t, ctx, pool, "user")
	reporter := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, owner, dbCategoryID(t, ctx, pool))
	reportID := seedDBReport(t, ctx, pool, pinID, reporter)

	repo := reports.NewRepository(pool)
	_, _, err := repo.ReviewReport(ctx, reportID, "dismiss", "99999999-9999-9999-9999-999999999999")
	if err == nil {
		t.Fatal("expected foreign-key error from UPDATE reports, got nil")
	}
	// The failed review must not have resolved the report.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM reports WHERE id = $1`, reportID).Scan(&status); err != nil {
		t.Fatalf("read report status: %v", err)
	}
	if status != reports.StatusPending {
		t.Errorf("expected report to stay pending after failed UPDATE, got %q", status)
	}
}
