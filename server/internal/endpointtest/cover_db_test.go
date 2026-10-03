package endpointtest

// DB-backed characterization of every cover-photo query: pins list,
// trending, collections (lateral + scalar variants), favorites and the
// social feed. Each test seeds pins with two photos (asserting ORDER BY
// position picks the first), a single backfilled photo (thumbnail == full
// URL) and a photo-less pin (cover falls back to ''). The refactor must
// keep every asserted cover identical.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
)

const (
	coverThumbA = "https://media.example/cov-a-thumb.webp"
	coverFullA  = "https://media.example/cov-a-full.webp"
	coverThumbB = "https://media.example/cov-b-thumb.webp"
	coverFullB  = "https://media.example/cov-b-full.webp"
)

// coverWorld is a bbox covering everything the seeds create (lat/lng
// 10..12). It stays well clear of [-180, 180]: an exactly-180-degree
// envelope trips PostGIS "Antipodal edge detected", and a near-global box
// inverts under geography intersection and matches nothing.
var coverWorld = [4]float64{9, 9, 13, 13}

type coverPhoto struct {
	full  string
	thumb string
}

// seedCoverPin creates a pin with the given photos in position order and
// returns its id. Photos insert positionally, so callers control ORDER BY
// position deterministically.
func seedCoverPin(t *testing.T, ctx context.Context, repo pins.Repository, userID string, categoryID int, lat, lng float64, caption string, photos []coverPhoto) string {
	t.Helper()
	full := make([]string, len(photos))
	thumb := make([]string, len(photos))
	for i, p := range photos {
		full[i] = p.full
		thumb[i] = p.thumb
	}
	p, err := repo.CreatePin(ctx, pins.NewPin{
		UserID:        userID,
		Lat:           lat,
		Lng:           lng,
		Caption:       &caption,
		CategoryID:    categoryID,
		PhotoURLs:     full,
		ThumbnailURLs: thumb,
		Geohash:       "covtest",
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}
	return p.ID.String()
}

func coverSetup(t *testing.T) (ctx context.Context, pool *pgxpool.Pool, author string, categoryID int) {
	t.Helper()
	pool = requireEndpointDB(t)
	ctx = context.Background()
	author = seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("cov-%d@example.com", time.Now().UnixNano()), "user")
	if err := pool.QueryRow(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return ctx, pool, author, categoryID
}

// seedCoverTrio creates the shared fixture: pin A with two photos (first wins),
// pin B with one backfilled photo (thumb == full), pin C with no photos.
func seedCoverTrio(t *testing.T, ctx context.Context, repo pins.Repository, author string, categoryID int) (a, b, c string) {
	t.Helper()
	a = seedCoverPin(t, ctx, repo, author, categoryID, 10, 10, "two photos",
		[]coverPhoto{{coverFullA, coverThumbA}, {coverFullB, coverThumbB}})
	b = seedCoverPin(t, ctx, repo, author, categoryID, 11, 11, "backfilled",
		[]coverPhoto{{coverFullB, coverFullB}})
	c = seedCoverPin(t, ctx, repo, author, categoryID, 12, 12, "no photos", nil)
	return a, b, c
}

func coverByID[T any](entries []T, id func(T) string) map[string]T {
	out := make(map[string]T, len(entries))
	for _, e := range entries {
		out[id(e)] = e
	}
	return out
}

func TestDBCoverPinsList(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	repo := pins.NewRepository(pool)
	a, b, c := seedCoverTrio(t, ctx, repo, author, categoryID)

	got, err := repo.ListPins(ctx, coverWorld, nil, 50)
	if err != nil {
		t.Fatalf("ListPins: %v", err)
	}
	byID := coverByID(got, func(e pins.PinListEntry) string { return e.ID.String() })
	if len(byID) < 3 {
		t.Fatalf("ListPins returned %d pins, want at least our 3", len(byID))
	}
	for id, want := range map[string]string{a: coverThumbA, b: coverFullB, c: ""} {
		entry, ok := byID[id]
		if !ok {
			t.Errorf("ListPins missing pin %s", id)
			continue
		}
		if entry.CoverURL != want {
			t.Errorf("cover for %s = %q, want %q", id, entry.CoverURL, want)
		}
	}
}

func TestDBCoverTrending(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	repo := pins.NewRepository(pool)
	a, b, c := seedCoverTrio(t, ctx, repo, author, categoryID)

	got, err := repo.ListTrending(ctx, coverWorld, 50)
	if err != nil {
		t.Fatalf("ListTrending: %v", err)
	}
	byID := coverByID(got, func(e pins.TrendingPin) string { return e.ID.String() })
	for id, want := range map[string]string{a: coverThumbA, b: coverFullB, c: ""} {
		entry, ok := byID[id]
		if !ok {
			t.Errorf("trending missing pin %s", id)
			continue
		}
		if entry.CoverURL != want {
			t.Errorf("trending cover for %s = %q, want %q", id, entry.CoverURL, want)
		}
	}
}

func TestDBCoverCollectionPins(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	pinsRepo := pins.NewRepository(pool)
	colRepo := collections.NewRepository(pool)
	a, b, c := seedCoverTrio(t, ctx, pinsRepo, author, categoryID)

	col, err := colRepo.Create(ctx, author, "covers", nil, false)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	t.Cleanup(func() { _ = colRepo.Delete(context.Background(), col.ID.String()) })
	for _, pid := range []string{a, b, c} {
		if err := colRepo.AddPin(ctx, col.ID.String(), pid); err != nil {
			t.Fatalf("add pin: %v", err)
		}
	}

	got, err := colRepo.ListPins(ctx, col.ID.String())
	if err != nil {
		t.Fatalf("ListPins: %v", err)
	}
	byID := coverByID(got, func(e pins.PinListEntry) string { return e.ID.String() })
	for id, want := range map[string]string{a: coverThumbA, b: coverFullB, c: ""} {
		entry, ok := byID[id]
		if !ok {
			t.Errorf("collection pins missing %s", id)
			continue
		}
		if entry.CoverURL != want {
			t.Errorf("collection pin cover for %s = %q, want %q", id, entry.CoverURL, want)
		}
	}
}

func TestDBCoverCollectionList(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	pinsRepo := pins.NewRepository(pool)
	colRepo := collections.NewRepository(pool)
	a, _, _ := seedCoverTrio(t, ctx, pinsRepo, author, categoryID)

	full, err := colRepo.Create(ctx, author, "with pins", nil, false)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	t.Cleanup(func() { _ = colRepo.Delete(context.Background(), full.ID.String()) })
	if err := colRepo.AddPin(ctx, full.ID.String(), a); err != nil {
		t.Fatalf("add pin: %v", err)
	}
	empty, err := colRepo.Create(ctx, author, "empty", nil, false)
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	t.Cleanup(func() { _ = colRepo.Delete(context.Background(), empty.ID.String()) })

	got, err := colRepo.ListByUser(ctx, author)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	byID := coverByID(got, func(e collections.Collection) string { return e.ID.String() })
	if byID[full.ID.String()].CoverURL != coverThumbA {
		t.Errorf("scalar cover = %q, want first thumbnail %q", byID[full.ID.String()].CoverURL, coverThumbA)
	}
	if byID[empty.ID.String()].CoverURL != "" {
		t.Errorf("empty collection cover = %q, want empty", byID[empty.ID.String()].CoverURL)
	}
}

func TestDBCoverFavorites(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	pinsRepo := pins.NewRepository(pool)
	favRepo := favorites.NewRepository(pool)
	a, _, _ := seedCoverTrio(t, ctx, pinsRepo, author, categoryID)

	if err := favRepo.Save(ctx, author, a); err != nil {
		t.Fatalf("save favorite: %v", err)
	}

	got, err := favRepo.List(ctx, author)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("favorites = %d entries, want 1", len(got))
	}
	if got[0].CoverURL != coverThumbA {
		t.Errorf("favorite cover = %q, want first thumbnail %q", got[0].CoverURL, coverThumbA)
	}
}

func TestDBCoverFeed(t *testing.T) {
	ctx, pool, author, categoryID := coverSetup(t)
	pinsRepo := pins.NewRepository(pool)
	socialRepo := social.NewRepository(pool)
	a, _, _ := seedCoverTrio(t, ctx, pinsRepo, author, categoryID)

	follower := seedDBUserWithRole(t, ctx, pool, fmt.Sprintf("covf-%d@example.com", time.Now().UnixNano()), "user")
	if err := socialRepo.Follow(ctx, follower, author); err != nil {
		t.Fatalf("follow: %v", err)
	}

	got, err := socialRepo.Feed(ctx, follower, 50)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	byID := coverByID(got, func(e pins.PinListEntry) string { return e.ID.String() })
	entry, ok := byID[a]
	if !ok {
		t.Fatalf("feed missing pin %s", a)
	}
	if entry.CoverURL != coverThumbA {
		t.Errorf("feed cover = %q, want first thumbnail %q", entry.CoverURL, coverThumbA)
	}
}
