package envgate

import "strings"

// Truthy reports whether a user-facing environment gate is enabled.
func Truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true":
		return true
	default:
		return false
	}
}
