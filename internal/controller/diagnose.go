package controller

import "strings"

// diagnosis is a short, honest explanation for a failure whose cause is the
// local container rather than the user's code.
type diagnosis struct {
	Cause string // what went wrong
	Hint  string // what the user can do about it
}

// diagnose recognises failures caused by the gap between GitHub's hosted
// runners and act's container image (plan F4). Those look like workflow bugs
// but aren't, and users lose real time chasing them, so name the cause at the
// pause instead of leaving them to work it out from a Gradle stack trace.
//
// Matching is deliberately conservative: a wrong guess is worse than silence.
func diagnose(out string) (diagnosis, bool) {
	l := strings.ToLower(out)

	switch {
	case strings.Contains(l, "sdk location not found"),
		strings.Contains(l, "android_home") && strings.Contains(l, "not"):
		return diagnosis{
			Cause: "the Android SDK is not installed in this container image",
			Hint:  "GitHub's ubuntu runners ship it; catthehacker/ubuntu:act-latest does not. Install it in a step, or run this job on GitHub.",
		}, true

	case strings.Contains(l, "not able to contact the cache service"),
		strings.Contains(l, "actions/cache") && strings.Contains(l, "fail"):
		return diagnosis{
			Cause: "GitHub's cache service is not reachable locally",
			Hint:  "caching is skipped when running locally; this is expected and usually harmless.",
		}, true

	case strings.Contains(l, "unable to get local issuer certificate"),
		strings.Contains(l, "x509: certificate"):
		return diagnosis{
			Cause: "TLS certificate verification failed inside the container",
			Hint:  "often a corporate proxy; the container does not inherit your host's CA bundle.",
		}, true

	case strings.Contains(l, "no space left on device"):
		return diagnosis{
			Cause: "the Docker volume ran out of space",
			Hint:  "try `docker system prune` to reclaim space.",
		}, true

	case strings.Contains(l, "oidc"),
		strings.Contains(l, "id-token"):
		return diagnosis{
			Cause: "this step needs GitHub's OIDC token service",
			Hint:  "that only exists on GitHub's runners — `skip` this step locally.",
		}, true
	}

	return diagnosis{}, false
}
