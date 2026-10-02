package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestEmitEventDisabledByDefault(t *testing.T) {
	SetJSONMode(false)
	out := captureStdout(t, func() {
		EmitEvent(IPCEvent{Type: EventSuccess})
	})
	if out != "" {
		t.Errorf("expected no output with JSON mode off, got %q", out)
	}
}

func TestEmitEventNDJSON(t *testing.T) {
	SetJSONMode(true)
	defer SetJSONMode(false)

	out := captureStdout(t, func() {
		EmitEvent(IPCEvent{Type: EventProgress, File: "a.txt", Bytes: 50, Total: 100, Percent: 50})
	})

	var evt IPCEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("event is not valid JSON: %v (got %q)", err, out)
	}
	if evt.Type != EventProgress || evt.File != "a.txt" || evt.Bytes != 50 || evt.Total != 100 || evt.Percent != 50 {
		t.Errorf("unexpected event fields: %+v", evt)
	}
	if evt.Timestamp == 0 {
		t.Error("expected timestamp to be set")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected newline-delimited output, got %q", out)
	}
}

func TestEmitProgressTracker(t *testing.T) {
	SetJSONMode(true)
	defer SetJSONMode(false)

	var nextCalls []int64
	next := func(n int64) { nextCalls = append(nextCalls, n) }

	out := captureStdout(t, func() {
		track := EmitProgressTracker("b.bin", 200, next)
		track(50)
		track(200)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 events, got %d (%q)", len(lines), out)
	}
	var first IPCEvent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if first.Percent != 25 || first.Bytes != 50 || first.Total != 200 {
		t.Errorf("unexpected progress math: %+v", first)
	}
	if len(nextCalls) != 2 || nextCalls[0] != 50 || nextCalls[1] != 200 {
		t.Errorf("expected wrapped callback passthrough, got %v", nextCalls)
	}
}

func TestEmitProgressTrackerZeroTotal(t *testing.T) {
	SetJSONMode(true)
	defer SetJSONMode(false)

	out := captureStdout(t, func() {
		EmitProgressTracker("empty.txt", 0, nil)(0)
	})
	var evt IPCEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &evt); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if evt.Percent != 0 {
		t.Errorf("expected 0%% for zero total (no div-by-zero), got %v", evt.Percent)
	}
}
