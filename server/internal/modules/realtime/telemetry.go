package realtime

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// malformedLocationDrops counts events dropped for an unparsable
// location, and malformedLocationLastLog gates how often that is written out.
// There is no metrics registry in this service, so the count rides along on a
// rate-limited structured log line: one warning per interval, each carrying the
// running total, so a recurrence is visible without a bad publisher being able
// to flood the logs.
var (
	malformedLocationDrops   atomic.Int64
	malformedLocationLastLog atomic.Int64
)

// malformedLocationLogIntervalSecs is the minimum gap, in seconds, between
// warnings. Kept in seconds because that is the unit both timestamps use.
const malformedLocationLogIntervalSecs = 60

// noteMalformedLocation records and (rate-limited) reports a drop.
func noteMalformedLocation(kind string, ev interface {
	GetID() string
	GetLocation() string
}) {
	total := malformedLocationDrops.Add(1)

	now := time.Now().Unix()
	last := malformedLocationLastLog.Load()
	if now-last < malformedLocationLogIntervalSecs {
		return
	}
	// Only the goroutine that wins the swap writes, so a burst of drops in the
	// same interval still produces exactly one line.
	if !malformedLocationLastLog.CompareAndSwap(last, now) {
		return
	}
	slog.Warn("realtime: dropped event with unparsable location",
		"kind", kind,
		"pin_id", ev.GetID(),
		"location", ev.GetLocation(),
		"drops_since_last_log", total,
	)
}
