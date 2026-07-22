package controller

import (
	"io"
	"strings"
	"testing"
	"time"
)

func recvLine(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a line")
		return ""
	}
}

func TestStdinMux_DeliversLines(t *testing.T) {
	m := newStdinMux(strings.NewReader("one\ntwo\n"))
	defer m.Close()

	if got := recvLine(t, m.Lines()); got != "one\n" {
		t.Errorf("first line = %q", got)
	}
	if got := recvLine(t, m.Lines()); got != "two\n" {
		t.Errorf("second line = %q", got)
	}
}

func TestStdinMux_EmitsEmptyStringAtEOF(t *testing.T) {
	m := newStdinMux(strings.NewReader("only\n"))
	defer m.Close()

	if got := recvLine(t, m.Lines()); got != "only\n" {
		t.Errorf("line = %q", got)
	}
	// EOF is signalled as an empty string so the REPL can resume instead of
	// spinning on a closed stdin.
	if got := recvLine(t, m.Lines()); got != "" {
		t.Errorf("expected empty string at EOF, got %q", got)
	}
}

func TestStdinMux_FlushesPartialLineAtEOF(t *testing.T) {
	m := newStdinMux(strings.NewReader("no-newline"))
	defer m.Close()

	if got := recvLine(t, m.Lines()); got != "no-newline" {
		t.Errorf("partial line = %q", got)
	}
}

// The regression this whole type exists for: while the shell has borrowed
// stdin, bytes must go to the shell and must NOT surface as REPL lines.
func TestStdinMux_BorrowRoutesBytesToBorrowerNotREPL(t *testing.T) {
	pr, pw := io.Pipe()
	m := newStdinMux(pr)
	defer m.Close()

	shellIn := m.borrowStdin()

	go func() {
		_, _ = pw.Write([]byte("shell-bytes\n"))
	}()

	buf := make([]byte, len("shell-bytes\n"))
	done := make(chan struct{})
	var n int
	var err error
	go func() {
		n, err = io.ReadFull(shellIn, buf)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("borrower never received the bytes")
	}
	if err != nil {
		t.Fatalf("borrower read: %v", err)
	}
	if got := string(buf[:n]); got != "shell-bytes\n" {
		t.Errorf("borrower got %q", got)
	}

	// Nothing should have leaked to the REPL line channel.
	select {
	case leaked := <-m.Lines():
		t.Errorf("bytes leaked to REPL while borrowed: %q", leaked)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestStdinMux_LinesResumeAfterRelease(t *testing.T) {
	pr, pw := io.Pipe()
	m := newStdinMux(pr)
	defer m.Close()

	shellIn := m.borrowStdin()
	go func() { _, _ = pw.Write([]byte("during\n")) }()
	buf := make([]byte, len("during\n"))
	if _, err := io.ReadFull(shellIn, buf); err != nil {
		t.Fatalf("borrower read: %v", err)
	}

	m.release()

	go func() { _, _ = pw.Write([]byte("after\n")) }()
	if got := recvLine(t, m.Lines()); got != "after\n" {
		t.Errorf("after release got %q, want %q", got, "after\n")
	}
}
