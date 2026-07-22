package controller

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// historyLimit caps the persisted command history.
const historyLimit = 200

// history persists REPL commands across sessions, so a paused step feels like
// a shell rather than a one-off prompt: the commands you typed last time are
// still there.
type history struct {
	path  string
	items []string
}

// historyPath returns where to persist REPL history, following XDG when set.
// An empty return means "don't persist", never an error, because failing to
// save history must never interfere with debugging.
func historyPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "fermata", "history")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "fermata", "history")
}

func loadHistory() *history {
	h := &history{path: historyPath()}
	if h.path == "" {
		return h
	}

	f, err := os.Open(h.path)
	if err != nil {
		return h // no history yet, or unreadable: start empty
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			h.items = append(h.items, line)
		}
	}
	return h
}

// add records a command, skipping blanks and immediate repeats.
func (h *history) add(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	if n := len(h.items); n > 0 && h.items[n-1] == cmd {
		return
	}
	h.items = append(h.items, cmd)
	if len(h.items) > historyLimit {
		h.items = h.items[len(h.items)-historyLimit:]
	}
}

// save writes the history back. Errors are deliberately ignored: a debugger
// must not fail because a history file could not be written.
func (h *history) save() {
	if h.path == "" || len(h.items) == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, item := range h.items {
		_, _ = w.WriteString(item + "\n")
	}
	_ = w.Flush()
}

// recent returns up to n most recent commands, newest last.
func (h *history) recent(n int) []string {
	if len(h.items) <= n {
		return h.items
	}
	return h.items[len(h.items)-n:]
}
