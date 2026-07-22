package controller

import (
	"fmt"
	"regexp"
	"strings"
)

// excerptLines is how many lines of failing output to show at the pause.
const excerptLines = 8

// actPrefix matches act's per-line decoration, e.g.
//
//	[CI/test]   | AssertionError: expected 45, got 49.8
//
// so the excerpt shows the step's own output rather than act's framing.
var actPrefix = regexp.MustCompile(`^\[[^\]]*\]\s*(\|\s?)?`)

// failureExcerpt pulls the tail of a failed step's output into a short, quoted
// block for the pause banner.
//
// "paused: exitcode '1'" is useless on its own. The assertion, stack trace, or
// compiler error has already scrolled past the setup noise above. Showing the
// last few lines of what the step printed is what makes the pause actionable.
func failureExcerpt(tail string) string {
	if strings.TrimSpace(tail) == "" {
		return ""
	}

	var lines []string
	for _, raw := range strings.Split(tail, "\n") {
		line := strings.TrimRight(actPrefix.ReplaceAllString(raw, ""), " \t\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isNoise(line) {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	// Prefer the headline error over trailing frames. Runtimes print the
	// message first and then pages of stack/object dump, so a plain tail shows
	// "Node.js v20.20.2" instead of "expected 45, got 49.8".
	if i, ok := firstErrorLine(lines); ok {
		lines = lines[i:]
	}

	if len(lines) > excerptLines {
		lines = lines[:excerptLines]
	}

	var b strings.Builder
	b.WriteString("   last output from the step:\n")
	for _, l := range lines {
		// Keep the block visually distinct from fermata's own messages.
		fmt.Fprintf(&b, "   │ %s\n", truncate(l, 100))
	}
	return strings.TrimRight(b.String(), "\n")
}

// errorMarkers are the shapes runtimes and build tools use to announce the
// actual problem, as opposed to the frames that follow it.
var errorMarkers = []string{
	"error:", "error ", "assertionerror", "exception",
	"failed:", "failure:", "fatal:", "panic:", "traceback",
	"cannot ", "unable to", "not found", "no such file",
	"npm err!", "* what went wrong",
}

// firstErrorLine finds where the real error starts, so the excerpt begins at
// the message rather than in the middle of a stack trace.
func firstErrorLine(lines []string) (int, bool) {
	for i, l := range lines {
		low := strings.ToLower(l)
		for _, m := range errorMarkers {
			if strings.Contains(low, m) {
				return i, true
			}
		}
	}
	return 0, false
}

// isNoise drops act's own status chatter, which repeats what the banner already
// says and would crowd out the real error.
func isNoise(line string) bool {
	t := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(t, "⭐"), strings.HasPrefix(t, "🐳"),
		strings.HasPrefix(t, "✅"), strings.HasPrefix(t, "❌"),
		strings.HasPrefix(t, "☁"), strings.HasPrefix(t, "🚀"),
		strings.HasPrefix(t, "🏁"), strings.HasPrefix(t, "❓"),
		strings.HasPrefix(t, "⚙"), strings.HasPrefix(t, "🚧"):
		return true
	case strings.HasPrefix(t, "Success - "), strings.HasPrefix(t, "Failure - "):
		return true
	case strings.HasPrefix(t, "exitcode '"):
		return true // the banner already states this
	}

	// act's git-metadata warnings fire repeatedly when the workflow isn't in a
	// git repo. They say nothing about why the step failed and, being frequent,
	// would push the real error out of a short excerpt.
	for _, noise := range []string{
		"unable to get git ref",
		"unable to get git revision",
		"not located inside a git repository",
		"repository does not exist",
		"not able to contact the cache service",
	} {
		if strings.Contains(t, noise) {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
