package apigen

import (
	"strings"
)

func conjoin(conj string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 { // "a and b" not "a, and b"
		return items[0] + " " + conj + " " + items[1]
	}

	sep := ", "
	pieces := []string{items[0]}
	for _, item := range items[1 : len(items)-1] {
		pieces = append(pieces, sep, item)
	}
	pieces = append(pieces, sep, conj, " ", items[len(items)-1])

	return strings.Join(pieces, "")
}
