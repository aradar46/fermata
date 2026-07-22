package engine

import "testing"

func TestResolvePlatforms_DefaultsCoverCommonUbuntuLabels(t *testing.T) {
	got, err := ResolvePlatforms(nil)
	if err != nil {
		t.Fatalf("ResolvePlatforms: %v", err)
	}
	for _, label := range []string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04"} {
		if got[label] == "" {
			t.Errorf("no default image for %q", label)
		}
	}
}

func TestResolvePlatforms_OverrideReplacesDefault(t *testing.T) {
	got, err := ResolvePlatforms([]string{"ubuntu-latest=myorg/android-ci:17"})
	if err != nil {
		t.Fatalf("ResolvePlatforms: %v", err)
	}
	if got["ubuntu-latest"] != "myorg/android-ci:17" {
		t.Errorf("ubuntu-latest = %q, want the override", got["ubuntu-latest"])
	}
	// Untouched defaults must survive.
	if got["ubuntu-22.04"] != DefaultUbuntuImage {
		t.Errorf("override should not clear other defaults")
	}
}

// A job with `runs-on: [self-hosted, linux, android]` matches no default, so
// nothing runs at all. Mapping one of its labels is the only way in.
func TestResolvePlatforms_MapsSelfHostedLabel(t *testing.T) {
	got, err := ResolvePlatforms([]string{"self-hosted=myorg/runner:latest"})
	if err != nil {
		t.Fatalf("ResolvePlatforms: %v", err)
	}
	if got["self-hosted"] != "myorg/runner:latest" {
		t.Errorf("self-hosted = %q", got["self-hosted"])
	}
}

func TestResolvePlatforms_LabelsAreCaseInsensitive(t *testing.T) {
	got, err := ResolvePlatforms([]string{"Ubuntu-Latest=img:1"})
	if err != nil {
		t.Fatalf("ResolvePlatforms: %v", err)
	}
	if got["ubuntu-latest"] != "img:1" {
		t.Errorf("label should be lowercased, got map %v", got)
	}
}

func TestResolvePlatforms_ImageMayContainEquals(t *testing.T) {
	// Only the first "=" separates label from image.
	got, err := ResolvePlatforms([]string{"ubuntu-latest=registry:5000/img?a=b"})
	if err != nil {
		t.Fatalf("ResolvePlatforms: %v", err)
	}
	if got["ubuntu-latest"] != "registry:5000/img?a=b" {
		t.Errorf("got %q", got["ubuntu-latest"])
	}
}

func TestResolvePlatforms_RejectsMalformed(t *testing.T) {
	for _, spec := range []string{"noequals", "=noimage", "label=", ""} {
		if _, err := ResolvePlatforms([]string{spec}); err == nil {
			t.Errorf("expected an error for %q", spec)
		}
	}
}

func TestParseMatrix_Empty(t *testing.T) {
	got, err := ParseMatrix(nil)
	if err != nil {
		t.Fatalf("ParseMatrix: %v", err)
	}
	if got != nil {
		t.Errorf("no specs should yield a nil filter (act runs all legs), got %v", got)
	}
}

func TestParseMatrix_SingleAndRepeatedKeys(t *testing.T) {
	got, err := ParseMatrix([]string{"python:3.12", "python:3.11", "os:ubuntu-latest"})
	if err != nil {
		t.Fatalf("ParseMatrix: %v", err)
	}
	if !got["python"]["3.12"] || !got["python"]["3.11"] {
		t.Errorf("repeated keys should OR their values, got %v", got["python"])
	}
	if !got["os"]["ubuntu-latest"] {
		t.Errorf("os filter missing, got %v", got)
	}
}

func TestParseMatrix_RejectsMalformed(t *testing.T) {
	for _, spec := range []string{"nocolon", ":novalue", "key:", ""} {
		if _, err := ParseMatrix([]string{spec}); err == nil {
			t.Errorf("expected an error for %q", spec)
		}
	}
}
