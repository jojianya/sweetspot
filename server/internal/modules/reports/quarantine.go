package reports

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// PinPhotos lists a pin's stored file URLs so the moderation hide path can
// quarantine them after the DB commit. The pins repository satisfies it via
// GetPin (which returns hidden pins with their photos); tests stub it.
type PinPhotos interface {
	GetPin(ctx context.Context, id string) (pins.PinDetail, error)
}

// WithQuarantine wires post-hide file moves onto the handler. When store or
// photos is nil the handler behaves exactly as before (no quarantine).
func (h *Handler) WithQuarantine(store *storage.Local, photos PinPhotos) *Handler {
	h.store = store
	h.photos = photos
	return h
}

// quarantinePinFiles moves every stored file of pinID out of the static root.
// Best-effort by design: the DB hide already committed, so a failure is
// logged and left for the sweep rather than failing the request. It never
// deletes.
func (h *Handler) quarantinePinFiles(ctx context.Context, pinID string) {
	if h.store == nil || h.photos == nil {
		return
	}
	detail, err := h.photos.GetPin(ctx, pinID)
	if err != nil {
		slog.Warn("quarantine: load pin photos", "error", err.Error(), "pin_id", pinID)
		return
	}
	for _, ph := range detail.Photos {
		for _, u := range []string{ph.PhotoURL, ph.ThumbnailURL} {
			if u == "" {
				continue
			}
			if _, err := h.store.Quarantine(u); err != nil {
				slog.Error("quarantine: move file", "error", err.Error(), "url", u, "pin_id", pinID)
			}
		}
	}
}

// SweepURLs quarantines every URL, returning (checked, moved, failed). With
// dryRun it only counts files that would move and moves nothing, so
// operators can read the first-run impact before enabling the real sweep.
// A cancelled context stops the walk: shutdown is not a failure, so counts
// so far return with a nil error.
func SweepURLs(ctx context.Context, urls []string, store *storage.Local, dryRun bool) (checked, moved, failed int, err error) {
	for _, u := range urls {
		select {
		case <-ctx.Done():
			return checked, moved, failed, nil
		default:
		}
		if u == "" {
			continue
		}
		checked++
		if dryRun {
			if store.QuarantineWouldMove(u) {
				moved++
			}
			continue
		}
		m, qerr := store.Quarantine(u)
		if qerr != nil {
			slog.Error("quarantine sweep: move file", "error", qerr.Error(), "url", u)
			failed++
			continue
		}
		if m {
			moved++
		}
	}
	return checked, moved, failed, nil
}

// SweepHiddenPinFiles quarantines stored files of every hidden pin. It heals
// pins hidden before quarantine wiring existed and finishes moves that failed
// at review time (idempotent, safe to rerun). A failure to list never fails
// the caller; per-file failures are logged and counted.
//
// There is no un-hide path: dismissing a report leaves the pin visible and
// nothing ever clears is_hidden, so the sweep never needs a restore step. If
// an un-hide feature is ever added, already-quarantined files stay
// quarantined until that feature moves them back — the sweep alone will not
// re-publish them.
func SweepHiddenPinFiles(ctx context.Context, pool *pgxpool.Pool, store *storage.Local, dryRun bool) (checked, moved, failed int, err error) {
	rows, err := pool.Query(ctx, `
		SELECT pp.photo_url, pp.thumbnail_url
		FROM pin_photos pp
		JOIN pins p ON p.id = pp.pin_id
		WHERE p.is_hidden = true
	`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var photo, thumb string
		if err := rows.Scan(&photo, &thumb); err != nil {
			return checked, moved, failed, err
		}
		urls = append(urls, photo, thumb)
	}
	if err := rows.Err(); err != nil {
		return checked, moved, failed, err
	}
	if !dryRun {
		// Crash leftovers from interrupted copies are never valid final
		// artifacts; drop them so they can't accumulate. Dry-run mutates
		// nothing.
		if n, err := store.CleanStaleTemps(); err != nil {
			slog.Warn("quarantine sweep: clean stale temps", "error", err.Error())
		} else if n > 0 {
			slog.Info("quarantine sweep: cleaned stale temps", "count", n)
		}
	}
	checked, moved, failed, err = SweepURLs(ctx, urls, store, dryRun)
	return checked, moved, failed, err
}
