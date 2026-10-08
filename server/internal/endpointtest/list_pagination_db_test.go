package endpointtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
)

// TestDBFavoritesListPaginates covers the saved-pins list: the total counts
// every match rather than the page, and walking pages covers each saved pin
// exactly once.
func TestDBFavoritesListPaginates(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	user := seedDBUser(t, ctx, pool, "user")
	categoryID := dbCategoryID(t, ctx, pool)
	repo := favorites.NewRepository(pool)

	const saved = 5
	for i := 0; i < saved; i++ {
		pinID := seedDBPin(t, ctx, pool, user, categoryID)
		if err := repo.Save(ctx, user, pinID); err != nil {
			t.Fatalf("save favorite %d: %v", i, err)
		}
	}
	// A pin the user owns but never saved, so "total" cannot accidentally mean
	// "everything this user owns".
	_ = seedDBPin(t, ctx, pool, user, categoryID)

	t.Run("TotalCountsEveryMatch", func(t *testing.T) {
		page, total, err := repo.List(ctx, user, 2, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if total != saved {
			t.Errorf("total = %d, want %d", total, saved)
		}
		if len(page) != 2 {
			t.Errorf("page size = %d, want 2", len(page))
		}
	})

	t.Run("PagesCoverEveryFavoriteOnce", func(t *testing.T) {
		seen := map[string]bool{}
		for offset := 0; ; offset += 2 {
			page, total, err := repo.List(ctx, user, 2, offset)
			if err != nil {
				t.Fatalf("offset %d: List: %v", offset, err)
			}
			if total != saved {
				t.Fatalf("offset %d: total = %d, want %d", offset, total, saved)
			}
			if len(page) == 0 {
				break
			}
			if len(page) > 2 {
				t.Fatalf("offset %d: page larger than limit: %d", offset, len(page))
			}
			for _, e := range page {
				if seen[e.Pin.ID] {
					t.Fatalf("offset %d: %s returned twice", offset, e.Pin.ID)
				}
				seen[e.Pin.ID] = true
			}
			if offset > saved+2 {
				t.Fatal("paging did not terminate")
			}
		}
		if len(seen) != saved {
			t.Errorf("covered %d favorites across pages, want %d", len(seen), saved)
		}
	})

	t.Run("EmptyPageKeepsTotal", func(t *testing.T) {
		page, total, err := repo.List(ctx, user, 2, 1000)
		if err != nil {
			t.Fatalf("List past the end: %v", err)
		}
		if len(page) != 0 {
			t.Errorf("page past the end = %d rows, want 0", len(page))
		}
		if total != saved {
			t.Errorf("total on an empty page = %d, want %d", total, saved)
		}
	})
}

// TestDBFavoritesListIDsStaysComplete proves ListIDs still answers membership
// for every favorite, which is what useSavedStatus relies on: it caps rows
// rather than paginating, so an older pin is never reported as unsaved.
func TestDBFavoritesListIDsStaysComplete(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	user := seedDBUser(t, ctx, pool, "user")
	categoryID := dbCategoryID(t, ctx, pool)
	repo := favorites.NewRepository(pool)

	const saved = 4
	want := map[string]bool{}
	for i := 0; i < saved; i++ {
		pinID := seedDBPin(t, ctx, pool, user, categoryID)
		if err := repo.Save(ctx, user, pinID); err != nil {
			t.Fatalf("save favorite %d: %v", i, err)
		}
		want[pinID] = true
	}

	ids, err := repo.ListIDs(ctx, user)
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	if len(ids) != saved {
		t.Fatalf("ListIDs returned %d ids, want %d", len(ids), saved)
	}
	for id := range want {
		found := false
		for _, got := range ids {
			if got == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListIDs is missing %s; membership lookups would report it unsaved", id)
		}
	}
}

// TestDBCommentsListPaginates covers the per-pin comment list the same way.
func TestDBCommentsListPaginates(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	user := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, user, dbCategoryID(t, ctx, pool))
	repo := comments.NewRepository(pool)

	const total = 5
	for i := 0; i < total; i++ {
		if _, err := repo.Create(ctx, pinID, user, fmt.Sprintf("comment %d", i)); err != nil {
			t.Fatalf("create comment %d: %v", i, err)
		}
	}

	t.Run("TotalCountsEveryMatch", func(t *testing.T) {
		page, got, err := repo.ListByPin(ctx, pinID, 2, 0)
		if err != nil {
			t.Fatalf("ListByPin: %v", err)
		}
		if got != total {
			t.Errorf("total = %d, want %d", got, total)
		}
		if len(page) != 2 {
			t.Errorf("page size = %d, want 2", len(page))
		}
	})

	t.Run("PagesCoverEveryCommentOnce", func(t *testing.T) {
		seen := map[string]bool{}
		for offset := 0; ; offset += 2 {
			page, _, err := repo.ListByPin(ctx, pinID, 2, offset)
			if err != nil {
				t.Fatalf("offset %d: ListByPin: %v", offset, err)
			}
			if len(page) == 0 {
				break
			}
			for _, c := range page {
				if seen[c.ID.String()] {
					t.Fatalf("offset %d: comment %s returned twice", offset, c.ID.String())
				}
				seen[c.ID.String()] = true
			}
			if offset > total+2 {
				t.Fatal("paging did not terminate")
			}
		}
		if len(seen) != total {
			t.Errorf("covered %d comments across pages, want %d", len(seen), total)
		}
	})
}

// TestDBCollectionPinsPaginate covers the collection pin list.
func TestDBCollectionPinsPaginate(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	user := seedDBUser(t, ctx, pool, "user")
	categoryID := dbCategoryID(t, ctx, pool)
	repo := collections.NewRepository(pool)

	col, err := repo.Create(ctx, user, fmt.Sprintf("paged-%d", len(t.Name())), nil, false)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), col.ID, user, true) })

	const total = 5
	for i := 0; i < total; i++ {
		pinID := seedDBPin(t, ctx, pool, user, categoryID)
		if err := repo.AddPin(ctx, col.ID, pinID); err != nil {
			t.Fatalf("add pin %d: %v", i, err)
		}
	}

	t.Run("TotalCountsEveryMatch", func(t *testing.T) {
		page, got, err := repo.ListPins(ctx, col.ID, 2, 0)
		if err != nil {
			t.Fatalf("ListPins: %v", err)
		}
		if got != total {
			t.Errorf("total = %d, want %d", got, total)
		}
		if len(page) != 2 {
			t.Errorf("page size = %d, want 2", len(page))
		}
	})

	t.Run("PagesCoverEveryPinOnce", func(t *testing.T) {
		seen := map[string]bool{}
		for offset := 0; ; offset += 2 {
			page, _, err := repo.ListPins(ctx, col.ID, 2, offset)
			if err != nil {
				t.Fatalf("offset %d: ListPins: %v", offset, err)
			}
			if len(page) == 0 {
				break
			}
			for _, e := range page {
				if seen[e.Pin.ID] {
					t.Fatalf("offset %d: pin %s returned twice", offset, e.Pin.ID)
				}
				seen[e.Pin.ID] = true
			}
			if offset > total+2 {
				t.Fatal("paging did not terminate")
			}
		}
		if len(seen) != total {
			t.Errorf("covered %d pins across pages, want %d", len(seen), total)
		}
	})
}
