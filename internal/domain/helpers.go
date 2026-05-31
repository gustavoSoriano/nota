package domain

import "strings"

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func trimHashPrefix(s string) string {
	return strings.TrimLeft(strings.TrimSpace(s), "# ")
}

func truncated(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
