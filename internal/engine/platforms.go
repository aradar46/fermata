package engine

import (
	"fmt"
	"strings"
)

// DefaultUbuntuImage is act's standard "medium" image: a size/compatibility
// compromise that runs most actions. Full-parity images are ~20GB compressed.
const DefaultUbuntuImage = "catthehacker/ubuntu:act-latest"

// defaultPlatforms maps the runs-on labels fermata supports out of the box.
func defaultPlatforms() map[string]string {
	return map[string]string{
		"ubuntu-latest": DefaultUbuntuImage,
		"ubuntu-24.04":  DefaultUbuntuImage,
		"ubuntu-22.04":  DefaultUbuntuImage,
		"ubuntu-20.04":  DefaultUbuntuImage,
	}
}

// ResolvePlatforms builds the runs-on label → container image map, applying
// user overrides on top of the defaults.
//
// Overrides matter more than they look. Two cases the defaults cannot serve:
//
//   - Jobs needing tooling the standard image lacks (the Android SDK is the
//     usual one), where the user has an image that has it.
//   - Self-hosted labels. A job with `runs-on: [self-hosted, linux, android]`
//     matches nothing in the defaults, so nothing runs at all. Mapping any
//     one of its labels to an image makes the job runnable.
//
// Each spec is "label=image". Labels are compared case-insensitively, as act
// does.
func ResolvePlatforms(specs []string) (map[string]string, error) {
	platforms := defaultPlatforms()

	for _, spec := range specs {
		label, image, ok := strings.Cut(spec, "=")
		label = strings.ToLower(strings.TrimSpace(label))
		image = strings.TrimSpace(image)

		if !ok || label == "" {
			return nil, fmt.Errorf("invalid --platform %q: expected label=image, e.g. "+
				"-P ubuntu-latest=catthehacker/ubuntu:full-latest", spec)
		}
		if image == "" {
			// act treats an empty image as "skip this platform"; be explicit
			// rather than silently producing a job that never runs.
			return nil, fmt.Errorf("invalid --platform %q: no image given for label %q", spec, label)
		}
		platforms[label] = image
	}

	return platforms, nil
}
