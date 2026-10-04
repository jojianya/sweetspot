package users

import "testing"

// TestValidateUsername pins the character allowlist for new usernames.
//
// The rejects matter more than the accepts: spaces and quotes break the
// rendered name and any naive string handling, angle brackets and slashes invite
// injection into markup and URLs, and non-ASCII letters are how a homograph
// account impersonates an existing one while looking identical.
func TestValidateUsername(t *testing.T) {
	valid := []string{
		"alice",
		"Alice",
		"ALICE99",
		"user_name",
		"user.name",
		"user-name",
		"a.b_c-d",
		"___",
		"123",
		strings30,
	}
	for _, name := range valid {
		if err := ValidateUsername(name); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", name, err)
		}
	}

	invalid := []struct {
		name string
		why  string
	}{
		{"", "empty"},
		{"ab", "too short"},
		{strings31, "too long"},
		{"alice smith", "space"},
		{"alice\tsmith", "tab"},
		{"alice\nsmith", "newline"},
		{"<script>", "angle brackets"},
		{"alice<b>", "angle bracket"},
		{`alice"`, "double quote"},
		{`alice'`, "single quote"},
		{"`alice`", "backtick"},
		{"alice/bob", "slash"},
		{"alice\\bob", "backslash"},
		{"alice@home", "at sign"},
		{"alice:admin", "colon"},
		{"alice;drop", "semicolon"},
		{"../../etc", "path traversal"},
		{"%00alice", "percent encoding"},
		{"caf\u00e9", "non-ASCII letter"},
		{"\u0430lice", "Cyrillic a homoglyph"},
		{"alice\u200bname", "zero-width space"},
		{"user name", "space"},
		{"\u7528\u6237", "CJK"},
		{"emoji\U0001F600name", "emoji"},
	}
	for _, tc := range invalid {
		if err := ValidateUsername(tc.name); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want an error (%s)", tc.name, tc.why)
		}
	}
}

// The length rule still applies alongside the character rule, and both messages
// are the ones the handler already used for length so nothing else moved.
func TestValidateUsernameLengthBoundaries(t *testing.T) {
	if err := ValidateUsername("ab"); err == nil || err.Error() != "username must be between 3 and 30 characters" {
		t.Errorf("short name error = %v, want the length message", err)
	}
	if err := ValidateUsername(strings31); err == nil || err.Error() != "username must be between 3 and 30 characters" {
		t.Errorf("long name error = %v, want the length message", err)
	}
}

// A rename is checked the same way as a registration, and an unchanged field
// (absent from the form) is never validated — otherwise every profile save
// would re-validate a legacy username the account already carries.
func TestValidateProfileFieldsUsernameRules(t *testing.T) {
	t.Run("accepts a valid rename", func(t *testing.T) {
		patch, msg := validateProfileFields([]string{" new_name "}, nil, false)
		if msg != "" {
			t.Fatalf("msg = %q, want empty", msg)
		}
		if patch.username == nil || *patch.username != "new_name" {
			t.Fatalf("username = %v, want new_name", patch.username)
		}
	})

	t.Run("rejects an invalid rename", func(t *testing.T) {
		for _, bad := range []string{"bad name", "<script>", "аlice"} {
			if _, msg := validateProfileFields([]string{bad}, nil, false); msg == "" {
				t.Errorf("validateProfileFields(%q) = no error, want 400 message", bad)
			}
		}
	})

	t.Run("ignores the username field when absent", func(t *testing.T) {
		patch, msg := validateProfileFields(nil, []string{`{"instagram":"@x"}`}, false)
		if msg != "" {
			t.Fatalf("msg = %q, want empty", msg)
		}
		if patch.username != nil {
			t.Fatalf("username = %v, want nil when the field is absent", *patch.username)
		}
	})

	t.Run("still rejects an empty username", func(t *testing.T) {
		if _, msg := validateProfileFields([]string{"   "}, nil, false); msg != "username cannot be empty" {
			t.Errorf("msg = %q, want the empty message", msg)
		}
	})
}

const (
	strings30 = "abcdefghijabcdefghijabcdefghij"
	strings31 = "abcdefghijabcdefghijabcdefghijk"
)
