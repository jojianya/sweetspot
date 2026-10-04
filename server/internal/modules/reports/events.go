package reports

import "context"

// Publisher receives pin-removal notifications after the DB transaction has
// committed. It carries only plain data (ID + WKT location) so reports stays
// free of a pins import; the realtime broker implements this method
// structurally. Implementations must be best-effort: log publish failures,
// never fail the removal.
type Publisher interface {
	PublishRemoval(ctx context.Context, id string, location string)
}
