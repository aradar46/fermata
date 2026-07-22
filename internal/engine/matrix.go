package engine

import (
	"fmt"
	"strings"
)

// ParseMatrix builds act's matrix filter from "key:value" specs.
//
// A matrix job fans out into one container per combination. Debugging is a
// single-cell activity. You want the leg that failed, not all twelve at once,
// which would also swamp a laptop. Repeating a key ORs its values:
//
//	--matrix python:3.12                 -> only the 3.12 legs
//	--matrix python:3.12 --matrix os:ubuntu-latest
//
// The "key:value" form matches act's own --matrix flag so act users don't have
// to learn a second syntax.
func ParseMatrix(specs []string) (map[string]map[string]bool, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	matrix := make(map[string]map[string]bool)
	for _, spec := range specs {
		key, value, ok := strings.Cut(spec, ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid --matrix %q: expected key:value, e.g. "+
				"--matrix python:3.12", spec)
		}

		if matrix[key] == nil {
			matrix[key] = make(map[string]bool)
		}
		matrix[key][value] = true
	}

	return matrix, nil
}
