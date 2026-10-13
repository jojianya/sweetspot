package reactions

import (
	"errors"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// ErrNotFound is pins.ErrNotFound. A hidden pin must be indistinguishable
// from a missing one, and telling the two apart would confirm that a
// moderated pin exists — so the two cases share one sentinel value.
var ErrNotFound = pins.ErrNotFound

// ErrOwnPin reports a reaction on the caller's own pin. Authors cannot react
// to their own pin, so the UI hides the control and this is the server-side
// backstop for a direct API call.
var ErrOwnPin = errors.New("you cannot react to your own pin")
