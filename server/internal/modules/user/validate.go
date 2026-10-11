package users

import (
	"errors"
	"fmt"
	"regexp"
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

// socialLinkKeys are the socials whose values the client renders as anchors
// (client/src/lib/utils/profile.ts, socialLinks). Unknown keys are passthrough
// data that nothing ever links, so they need no scheme check.
var socialLinkKeys = map[string]bool{"website": true, "instagram": true, "twitter": true}

// uriSchemeRe matches a leading URI scheme per RFC 3986:
// ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":". A value with no match
// carries no scheme at all, which is the plain-handle form.
var uriSchemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*:`)

// hostPortRe matches only a port, optionally followed by a path. It separates
// "alice.example:8080" — a bare host with a port, which the client renders as
// https://alice.example:8080 — from "javascript:alert(1)", whose colon opens a
// URI scheme. Without this, every host:port value is misread as a scheme.
var hostPortRe = regexp.MustCompile(`^[0-9]+(/[^:]*)?$`)

// validateSocialLink accepts exactly the two shapes the client knows how to
// render: a plain handle or host with no scheme ("alice", "@alice",
// "alice.example", "alice.example:8080"), or an explicit http(s) URL. Every
// other scheme is rejected, matched case-insensitively so "JaVaScRiPt:" cannot
// slip past.
//
// Why this exists: socialLinks() drops these values straight into an anchor
// href, and an href like "javascript:…" executes when the link is clicked.
// The client's own guard (profile.ts:37) prefixes https:// for anything that
// is not already http(s), which neutralises it there; this is the server-side
// half, so a stored value is safe no matter which client reads it back.
//
// Only the scheme is echoed in the error, never the value, so the response
// does not reflect the payload it just rejected.
func validateSocialLink(key, value string) error {
	if !socialLinkKeys[strings.ToLower(key)] {
		return nil
	}
	v := strings.TrimSpace(value)
	loc := uriSchemeRe.FindStringIndex(v)
	if loc == nil {
		return nil // no colon anywhere: a plain handle or host
	}
	if hostPortRe.MatchString(v[loc[1]:]) {
		return nil // host:port, not a scheme
	}
	if scheme := strings.ToLower(v[:loc[1]-1]); scheme != "http" && scheme != "https" {
		return fmt.Errorf("socials %s must be a plain handle or an http(s) URL, not a %s link", key, scheme)
	}
	return nil
}
