package endpointtest

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// TestDBUpdatePinReturnsPhotoSet covers the database half of the update flow:
// the photo set the response carries is read back inside the update
// transaction, and PinVisible follows the shared hidden-pin rule that makes
// the handler answer 404 for an edit of a hidden pin.
func TestDBUpdatePinReturnsPhotoSet(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	owner := seedDBUser(t, ctx, pool, "user")
	pinID := seedDBPin(t, ctx, pool, owner, dbCategoryID(t, ctx, pool))
	seedDBPhotos(t, ctx, pool, pinID, []string{"seed-a.webp", "seed-b.webp"})

	repo := pins.NewRepository(pool)

	t.Run("CaptionOnlyEditKeepsPhotoSet", func(t *testing.T) {
		caption := "renamed"
		updated, photos, err := repo.UpdatePin(ctx, pinID, pins.UpdatePinPatch{Caption: &caption})
		if err != nil {
			t.Fatalf("UpdatePin: %v", err)
		}
		if updated.Caption == nil || *updated.Caption != "renamed" {
			t.Fatalf("caption = %v, want %q", updated.Caption, "renamed")
		}
		assertPhotoURLs(t, photos, []string{"seed-a.webp", "seed-b.webp"})
	})

	t.Run("PhotoReplacementReturnsNewSet", func(t *testing.T) {
		replacement := []pins.NewPhoto{
			{PhotoURL: "new-a.webp", ThumbnailURL: "new-a-thumb.webp"},
			{PhotoURL: "new-b.webp", ThumbnailURL: "new-b-thumb.webp"},
			{PhotoURL: "new-c.webp", ThumbnailURL: "new-c-thumb.webp"},
		}
		_, photos, err := repo.UpdatePin(ctx, pinID, pins.UpdatePinPatch{Photos: replacement})
		if err != nil {
			t.Fatalf("UpdatePin with photos: %v", err)
		}
		assertPhotoURLs(t, photos, []string{"new-a.webp", "new-b.webp", "new-c.webp"})
		for i, ph := range photos {
			if ph.PinID.String() != pinID {
				t.Errorf("photo %d pin_id = %s, want %s", i, ph.PinID.String(), pinID)
			}
			if ph.ID.String() == "" || ph.CreatedAt.IsZero() {
				t.Errorf("photo %d is missing its row id or created_at: %+v", i, ph)
			}
			if int(ph.Position) != i {
				t.Errorf("photo %d position = %d, want %d", i, ph.Position, i)
			}
		}
	})

	t.Run("HiddenPinIsNotVisible", func(t *testing.T) {
		if visible, err := repo.PinVisible(ctx, pinID); err != nil || !visible {
			t.Fatalf("PinVisible before hide = %v, %v; want true, nil", visible, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE pins SET is_hidden = true WHERE id = $1`, pinID); err != nil {
			t.Fatalf("hide pin: %v", err)
		}
		if visible, err := repo.PinVisible(ctx, pinID); err != nil || visible {
			t.Errorf("PinVisible after hide = %v, %v; want false, nil", visible, err)
		}
	})
}

func assertPhotoURLs(t *testing.T, photos []pins.PinPhoto, want []string) {
	t.Helper()
	if len(photos) != len(want) {
		t.Fatalf("photos = %d, want %d (%+v)", len(photos), len(want), photos)
	}
	for i, ph := range photos {
		if !strings.HasSuffix(ph.PhotoURL, want[i]) {
			t.Errorf("photo %d url = %q, want it to end in %q", i, ph.PhotoURL, want[i])
		}
	}
}

func seedDBPhotos(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pinID string, names []string) {
	t.Helper()
	for i, name := range names {
		if _, err := pool.Exec(ctx, `
			INSERT INTO pin_photos (pin_id, photo_url, thumbnail_url, position)
			VALUES ($1, $2, $3, $4)
		`, pinID, name, name+"-thumb", i); err != nil {
			t.Fatalf("seed photo %s: %v", name, err)
		}
	}
}
