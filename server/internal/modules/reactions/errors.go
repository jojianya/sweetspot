package reactions

import "errors"

// ErrNotFound reports a pin that does not exist or is hidden. The message
// matches pins.ErrNotFound so the handler answers the same "pin not found" the
// rest of the API uses: a hidden pin must be indistinguishable from a missing
// one, and telling the two apart would confirm that a moderated pin exists.
var ErrNotFound = errors.New("pin not found")

// ErrOwnPin reports a reaction on the caller's own pin. Authors cannot react
// to their own pin, so the UI hides the control and this is the server-side
// backstop for a direct API call.
var ErrOwnPin = errors.New("you cannot react to your own pin")
