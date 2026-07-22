package controller

import "strings"

// masker replaces known secret values with *** in strings fermata prints itself
// (plan F16). It reuses the same inputs act masks with: configured secret
// values and any runtime-registered masks (rc.Masks). Note: this cannot mask
// what the user types into the in-container shell, that is documented, not
// pretended away.
type masker struct {
	secrets []string
}

func newMasker(secretMap map[string]string, runtimeMasks []string) *masker {
	seen := make(map[string]struct{})
	vals := make([]string, 0, len(secretMap)+len(runtimeMasks))
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		vals = append(vals, v)
	}
	for _, v := range secretMap {
		add(v)
	}
	for _, v := range runtimeMasks {
		add(v)
	}
	return &masker{secrets: vals}
}

// mask returns s with every known secret value replaced by ***.
func (m *masker) mask(s string) string {
	for _, secret := range m.secrets {
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}
