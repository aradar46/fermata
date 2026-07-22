package controller

import (
	"strings"
	"testing"
)

func TestDiagnose_AndroidSDKMissing(t *testing.T) {
	// The real Gradle output from a local run of an Android deploy workflow.
	out := `FAILURE: Build failed with an exception.
* What went wrong:
Could not determine the dependencies of task ':app:bundleReleaseResources'.
> SDK location not found. Define a valid SDK location with an ANDROID_HOME environment variable or by setting the sdk.dir path in your project's local properties file.`

	d, ok := diagnose(out)
	if !ok {
		t.Fatal("expected the missing Android SDK to be recognised")
	}
	if !strings.Contains(d.Cause, "Android SDK") {
		t.Errorf("cause = %q", d.Cause)
	}
	if d.Hint == "" {
		t.Error("a diagnosis should tell the user what to do")
	}
}

func TestDiagnose_KnownLocalGaps(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"cache service", "The runner was not able to contact the cache service. Caching will be skipped", "cache"},
		{"tls", "x509: certificate signed by unknown authority", "certificate"},
		{"disk", "write /tmp/x: no space left on device", "space"},
		{"oidc", "Unable to get ACTIONS_ID_TOKEN_REQUEST_URL for oidc", "OIDC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, ok := diagnose(tt.out)
			if !ok {
				t.Fatalf("expected %s to be recognised", tt.name)
			}
			if !strings.Contains(strings.ToLower(d.Cause), strings.ToLower(tt.want)) {
				t.Errorf("cause = %q, want it to mention %q", d.Cause, tt.want)
			}
		})
	}
}

// A wrong guess is worse than silence: ordinary failures must not be
// misattributed to the environment.
func TestDiagnose_StaysSilentOnOrdinaryFailures(t *testing.T) {
	for _, out := range []string{
		"npm ERR! missing script: buidl",
		"error: expected ';' at end of declaration",
		"Tests failed: 3 assertions did not pass",
		"",
	} {
		if d, ok := diagnose(out); ok {
			t.Errorf("diagnose(%q) should stay silent, got %q", out, d.Cause)
		}
	}
}

func TestGatedWriter_TailRetainsRecentOutput(t *testing.T) {
	var sink strings.Builder
	g := newGatedWriter(&sink)

	if _, err := g.Write([]byte("SDK location not found")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Tail(), "SDK location not found") {
		t.Errorf("tail = %q", g.Tail())
	}
}

func TestGatedWriter_TailIsBounded(t *testing.T) {
	var sink strings.Builder
	g := newGatedWriter(&sink)

	big := strings.Repeat("x", tailLimit*2)
	if _, err := g.Write([]byte(big)); err != nil {
		t.Fatal(err)
	}
	if len(g.Tail()) > tailLimit {
		t.Errorf("tail grew to %d bytes, limit is %d", len(g.Tail()), tailLimit)
	}
}

// Output captured while the gate is closed (REPL owns the terminal) must still
// be available for diagnosis.
func TestGatedWriter_TailCapturedWhileClosed(t *testing.T) {
	var sink strings.Builder
	g := newGatedWriter(&sink)

	g.Close()
	if _, err := g.Write([]byte("no space left on device")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(g.Tail(), "no space left on device") {
		t.Errorf("tail should capture output written while closed, got %q", g.Tail())
	}
}
