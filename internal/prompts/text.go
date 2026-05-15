package prompts

// TruncateText shortens s to maxRunes runes, appending "..." if truncated.
func TruncateText(s string, maxRunes int) string {
	if maxRunes < 0 {
		maxRunes = 0
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
