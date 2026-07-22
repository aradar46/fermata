package controller

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// editedSince reports whether any source file under root was modified after t,
// and returns one example path.
//
// This exists to catch the worst failure mode in the retry loop: without a
// bind mount the container holds a copy of the repo taken at checkout, so a
// user can fix their bug, hit retry, and watch it re-run the OLD code and fail
// with the identical error. Detecting the edit lets us explain that instead of
// letting them think the tool is broken.
func editedSince(root string, t time.Time) (string, bool) {
	if root == "" {
		return "", false
	}

	var found string
	// A bounded walk: this runs at a human-interactive moment, and repos can be
	// large, so stop as soon as we have an answer and skip the usual noise.
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are not interesting here
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		if info.ModTime().After(t) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})

	return found, found != ""
}

// skipDir filters directories whose churn says nothing about the user editing
// source: VCS metadata, dependencies, and build output.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", "target",
		".gradle", ".idea", ".vscode", "__pycache__", ".venv", "venv":
		return true
	}
	return strings.HasPrefix(name, ".terraform")
}
