package database

import "strings"

// EscapeLike escapes the LIKE wildcard characters (\, %, _) so a user search
// string matches literally instead of acting as a pattern. Pair with
// ILIKE '%' || $1 || '%' at the call site.
func EscapeLike(query string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
}
