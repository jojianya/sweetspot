package users

import (
	"errors"
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

// usernameAllowed is the set of characters a username may contain: ASCII
// letters, digits, underscore, dot and hyphen.
//
// It is an allowlist rather than a denylist on purpose. A username is rendered
// as text all over the app (profile headers, comments, admin tables) and is
// also a login identifier, so the useful property is that *no* character outside
// this set can ever be stored: no spaces, quotes, angle brackets, slashes, or
// non-ASCII letters that look like ASCII ones (Cyrillic "а" for "a"), which are
// how homograph accounts and display-name spoofing are built.
const usernameAllowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-"

// ValidateUsername reports whether a username may be stored: 3-30 characters,
// all drawn from usernameAllowed.
//
// It applies only where a username is *chosen* — registration and a profile
// rename. Login and username lookup deliberately do not call it, so an account
// that predates this rule keeps working; this only stops new ones from being
// created or renamed into a shape nobody wants.
func ValidateUsername(username string) error {
	if n := utf8.RuneCountInString(username); n < usernameMinRunes || n > usernameMaxRunes {
		return errors.New("username must be between 3 and 30 characters")
	}
	for _, r := range username {
		if !strings.ContainsRune(usernameAllowed, r) {
			return errors.New("username may only contain letters, numbers, and _ . -")
		}
	}
	return nil
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
		if err := ValidateUsername(username); err != nil {
			return nil, err.Error()
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
