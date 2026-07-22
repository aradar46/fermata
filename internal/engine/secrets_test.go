package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecrets_Pairs(t *testing.T) {
	got, err := LoadSecrets("", []string{"A=1", "B=with=equals"})
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if got["A"] != "1" {
		t.Errorf("A = %q", got["A"])
	}
	// Only the first "=" separates name from value.
	if got["B"] != "with=equals" {
		t.Errorf("B = %q, want %q", got["B"], "with=equals")
	}
}

func TestLoadSecrets_BareKeyReadsEnvironment(t *testing.T) {
	t.Setenv("FERMATA_TEST_SECRET", "from-env")

	got, err := LoadSecrets("", []string{"FERMATA_TEST_SECRET"})
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if got["FERMATA_TEST_SECRET"] != "from-env" {
		t.Errorf("got %q, want %q", got["FERMATA_TEST_SECRET"], "from-env")
	}
}

func TestLoadSecrets_BareKeyMissingFromEnvIsAnError(t *testing.T) {
	os.Unsetenv("FERMATA_DEFINITELY_UNSET")

	if _, err := LoadSecrets("", []string{"FERMATA_DEFINITELY_UNSET"}); err == nil {
		t.Error("expected an error when a bare --secret is not in the environment")
	}
}

func TestLoadSecrets_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	body := "" +
		"# a comment\n" +
		"\n" +
		"PLAIN=value\n" +
		"QUOTED=\"quoted value\"\n" +
		"SINGLE='single quoted'\n" +
		"  SPACED  =  padded  \n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSecrets(path, nil)
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	for k, want := range map[string]string{
		"PLAIN":  "value",
		"QUOTED": "quoted value",
		"SINGLE": "single quoted",
		"SPACED": "padded",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}

func TestLoadSecrets_ExplicitOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.env")
	if err := os.WriteFile(path, []byte("K=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSecrets(path, []string{"K=from-flag"})
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if got["K"] != "from-flag" {
		t.Errorf("K = %q, want the --secret value to win", got["K"])
	}
}

func TestLoadSecrets_MissingFileIsAnError(t *testing.T) {
	if _, err := LoadSecrets(filepath.Join(t.TempDir(), "nope.env"), nil); err == nil {
		t.Error("expected an error for a missing secret file")
	}
}

func TestLoadSecrets_MalformedLineIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.env")
	if err := os.WriteFile(path, []byte("NOT_A_PAIR\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadSecrets(path, nil); err == nil {
		t.Error("expected an error for a line without '='")
	}
}

func TestLoadSecrets_EmptyInputsGiveEmptyMap(t *testing.T) {
	got, err := LoadSecrets("", nil)
	if err != nil {
		t.Fatalf("LoadSecrets: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}
