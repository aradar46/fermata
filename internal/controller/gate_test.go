package controller

import (
	"bytes"
	"testing"
)

func TestGatedWriter_PassesThroughWhenOpen(t *testing.T) {
	var buf bytes.Buffer
	g := newGatedWriter(&buf)

	if _, err := g.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q, want %q", buf.String(), "hello")
	}
}

func TestGatedWriter_BuffersWhenClosedThenFlushesOnOpen(t *testing.T) {
	var buf bytes.Buffer
	g := newGatedWriter(&buf)

	g.Close()
	// Writes while closed (REPL owns the terminal) must not reach the sink yet.
	n, err := g.Write([]byte("quiesced-1\n"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len("quiesced-1\n") {
		t.Errorf("short write while closed: got %d", n)
	}
	if _, err := g.Write([]byte("quiesced-2\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("output leaked while closed: %q", buf.String())
	}

	// Reopening (continue) flushes everything buffered, in order.
	g.Open()
	want := "quiesced-1\nquiesced-2\n"
	if buf.String() != want {
		t.Errorf("after Open got %q, want %q", buf.String(), want)
	}

	// Subsequent writes pass through again.
	buf.Reset()
	if _, err := g.Write([]byte("live")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.String() != "live" {
		t.Errorf("post-open passthrough got %q, want %q", buf.String(), "live")
	}
}
