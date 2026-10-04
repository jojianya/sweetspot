package endpointtest

import (
	"context"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
)

// TestDBHiddenPinIsInvisibleToSharedChecks pins down the centralized
// pins.VisiblePinExists rule: once a pin is hidden, favorites, reports and
// collections all treat it as gone, and favorites stops returning it from
// ListIDs (it used to keep serving stale ids for deleted pins).
func TestDBHiddenPinIsInvisibleToSharedChecks(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	owner := seedDBUser(t, ctx, pool, "user")
	reporter := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, owner, dbCategoryID(t, ctx, pool))

	favRepo := favorites.NewRepository(pool)
	if err := favRepo.Save(ctx, reporter, pinID); err != nil {
		t.Fatalf("save favorite: %v", err)
	}

	// Sanity: everything reports the pin as existing while it is visible.
	if exists, err := favRepo.PinExists(ctx, pinID); err != nil || !exists {
		t.Fatalf("favorites.PinExists = %v, %v; want true, nil", exists, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE pins SET is_hidden = true WHERE id = $1`, pinID); err != nil {
		t.Fatalf("hide pin: %v", err)
	}

	if exists, err := favRepo.PinExists(ctx, pinID); err != nil || exists {
		t.Errorf("favorites.PinExists after hide = %v, %v; want false, nil", exists, err)
	}
	reportRepo := reports.NewRepository(pool)
	if exists, err := reportRepo.PinExists(ctx, pinID); err != nil || exists {
		t.Errorf("reports.PinExists after hide = %v, %v; want false, nil", exists, err)
	}
	colRepo := collections.NewRepository(pool)
	if exists, err := colRepo.PinExists(ctx, pinID); err != nil || exists {
		t.Errorf("collections.PinExists after hide = %v, %v; want false, nil", exists, err)
	}
	commentRepo := comments.NewRepository(pool)
	if exists, err := commentRepo.PinExistsVisible(ctx, pinID); err != nil || exists {
		t.Errorf("comments.PinExistsVisible after hide = %v, %v; want false, nil", exists, err)
	}

	ids, err := favRepo.ListIDs(ctx, reporter)
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	for _, id := range ids {
		if id == pinID {
			t.Errorf("ListIDs still returns hidden pin %s", pinID)
		}
	}

	entries, _, err := favRepo.List(ctx, reporter, 50, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List returned %d entries for hidden pin, want 0", len(entries))
	}
}
