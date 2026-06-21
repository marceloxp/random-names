package core

import (
	"strconv"
	"strings"
	"unicode"
)

// bom is the UTF-8 byte order mark, sometimes present at the start of a file.
const bom = "\uFEFF"

// NormalizeToken converts a raw line from a bootstrap file into the canonical
// stored form: trimmed, BOM-stripped, and Title-cased for a single token
// (first rune uppercase, the rest lowercase), Unicode-aware.
//
//	"MARCELO" -> "Marcelo", "ARAÚJO" -> "Araújo", "ASSUNÇÃO" -> "Assunção"
//
// Returns "" for blank lines so callers can skip them.
func NormalizeToken(line string) string {
	line = strings.TrimPrefix(line, bom)
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	runes := []rune(strings.ToLower(line))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// formatInt renders an integer with thousands separators (e.g. 10695667980 ->
// "10,695,667,980") for human-readable stats output.
func formatInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}

	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}

	if neg {
		return "-" + b.String()
	}
	return b.String()
}
