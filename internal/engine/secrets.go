package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadSecrets builds the secret map for a run, mirroring act's inputs.
//
//   - file: a dotenv-style file of KEY=value lines (act's --secret-file).
//   - pairs: "KEY=value", or bare "KEY" to read the value from the
//     environment (act's --secret behavior).
//
// Later sources win, so an explicit --secret overrides the file.
func LoadSecrets(file string, pairs []string) (map[string]string, error) {
	secrets := map[string]string{}

	if file != "" {
		fromFile, err := parseSecretFile(file)
		if err != nil {
			return nil, err
		}
		for k, v := range fromFile {
			secrets[k] = v
		}
	}

	for _, p := range pairs {
		k, v, err := parseSecretPair(p)
		if err != nil {
			return nil, err
		}
		secrets[k] = v
	}

	return secrets, nil
}

// parseSecretPair handles "KEY=value" and bare "KEY" (value from environment).
func parseSecretPair(p string) (string, string, error) {
	if k, v, ok := strings.Cut(p, "="); ok {
		k = strings.TrimSpace(k)
		if k == "" {
			return "", "", fmt.Errorf("invalid --secret %q: empty name", p)
		}
		return k, v, nil
	}

	k := strings.TrimSpace(p)
	if k == "" {
		return "", "", fmt.Errorf("invalid --secret: empty name")
	}
	v, ok := os.LookupEnv(k)
	if !ok {
		return "", "", fmt.Errorf("--secret %s: no value given and %s is not set in the environment", k, k)
	}
	return k, v, nil
}

// parseSecretFile reads dotenv-style KEY=value lines. Blank lines and lines
// starting with # are ignored; surrounding single or double quotes on the
// value are stripped.
func parseSecretFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secret file %q not found.\n"+
				"  it should contain one KEY=value per line, e.g.\n"+
				"    MY_TOKEN=abc123\n"+
				"  or pass secrets directly: --secret MY_TOKEN=abc123", path)
		}
		return nil, fmt.Errorf("read secret file %q: %w", path, err)
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, lineNo)
		}
		k = strings.TrimSpace(k)
		if k == "" {
			return nil, fmt.Errorf("%s:%d: empty secret name", path, lineNo)
		}
		out[k] = unquote(strings.TrimSpace(v))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}
	return out, nil
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
