package users

import (
	"strings"
	"unicode/utf8"
)

// profileUpdateInput carries the validated profile fields. Avatar bytes and
// storage stay with the caller: they need the multipart handle, imaging and
// the store.
type profileUpdateInput struct {
	username *string
	socials  *map[string]any
}

// validateProfileFields checks the username/socials form values in handler
// order, returning the exact message the handler responds with (all 400s).
// hasAvatar mirrors whether an avatar file was uploaded, for the
// nothing-to-update check.
func validateProfileFields(usernameVals, socialsVals []string, hasAvatar bool) (*profileUpdateInput, string) {
	var patch profileUpdateInput
	if len(usernameVals) > 0 {
		username := strings.TrimSpace(usernameVals[0])
		if username == "" {
			return nil, "username cannot be empty"
		}
		if n := utf8.RuneCountInString(username); n < usernameMinRunes || n > usernameMaxRunes {
			return nil, "username must be between 3 and 30 characters"
		}
		patch.username = &username
	}

	if len(socialsVals) > 0 {
		socials, parseErr := parseSocials(socialsVals[0])
		if parseErr != nil {
			return nil, parseErr.Error()
		}
		patch.socials = &socials
	}

	if patch.username == nil && patch.socials == nil && !hasAvatar {
		return nil, "nothing to update"
	}

	return &patch, ""
}
