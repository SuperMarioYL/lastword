package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// run executes the root command with the given args and returns stdout,
// stderr and the execution error.
func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCommand("test")
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func TestVerdictJSONOnFixture(t *testing.T) {
	out, _, err := run(t, "verdict", "../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("verdict: %v", err)
	}
	for _, want := range []string{
		`"schema": "verdict-span/v0"`,
		`"session": "7f3d2c8a-1e94-4b67-9c02-5a8d3f6b1e47"`,
		`"file": "src/auth/token.go"`,
		`"cmd": "go test ./..."`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q", want)
		}
	}
}

func TestVerdictTextOnFixture(t *testing.T) {
	out, _, err := run(t, "verdict", "../../testdata/sessions/session-500turn.jsonl", "--format", "text")
	if err != nil {
		t.Fatalf("verdict: %v", err)
	}
	if !strings.Contains(out, "The login flake was a stale refresh-token cache key") {
		t.Errorf("text output missing the answer lead:\n%s", out)
	}
}

func TestVerdictUnknownFormat(t *testing.T) {
	_, _, err := run(t, "verdict", "../../testdata/sessions/session-minimal.jsonl", "--format", "yaml")
	if err == nil || !strings.Contains(err.Error(), "yaml") {
		t.Errorf("err = %v, want unknown-format error naming yaml", err)
	}
}

func TestVerdictMissingFile(t *testing.T) {
	if _, _, err := run(t, "verdict", "no-such-session.jsonl"); err == nil {
		t.Fatal("verdict succeeded on a missing file, want error")
	}
}

func TestVerdictDiscoversNewestSession(t *testing.T) {
	// A tiny projects tree: the newest main session is picked, nested
	// transcripts are ignored, and the discovery note stays off stdout.
	root := t.TempDir()
	newest := filepath.Join(root, "proj-a", "newest.jsonl")
	writeSessionLine(t, newest, time.Now())
	writeSessionLine(t, filepath.Join(root, "proj-a", "older.jsonl"), time.Now().Add(-time.Hour))
	writeSessionLine(t, filepath.Join(root, "proj-a", "nested", "sub.jsonl"), time.Now().Add(time.Minute))

	out, errOut, err := run(t, "verdict", "--projects-dir", root)
	if err != nil {
		t.Fatalf("verdict discovery: %v", err)
	}
	if !strings.Contains(errOut, "discovered session: "+newest) {
		t.Errorf("stderr = %q, want a discovery note for %q", errOut, newest)
	}
	if !strings.Contains(out, `"schema": "verdict-span/v0"`) {
		t.Errorf("stdout = %q, want the span JSON only", out)
	}
}

func TestLatest(t *testing.T) {
	root := t.TempDir()
	newest := filepath.Join(root, "proj-b", "newest.jsonl")
	writeSessionLine(t, newest, time.Now())
	writeSessionLine(t, filepath.Join(root, "proj-b", "older.jsonl"), time.Now().Add(-time.Hour))

	out, _, err := run(t, "latest", "--projects-dir", root)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got := strings.TrimSpace(out); got != newest {
		t.Errorf("latest = %q, want %q", got, newest)
	}
}

func TestLatestEmptyTree(t *testing.T) {
	if _, _, err := run(t, "latest", "--projects-dir", t.TempDir()); err == nil {
		t.Fatal("latest succeeded on an empty tree, want error")
	}
}

func TestVerifyTextOnFixture(t *testing.T) {
	out, _, err := run(t, "verify", "../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, want := range []string{
		"LastWord — verify verdict-span/v0",
		"[pass] pointer_resolution — 14/14 pointers resolve",
		"[pass] answer_diff — 7 files claimed, 7 edited, 0 unclaimed",
		"verdict: PASS",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("verify report missing %q:\n%s", want, out)
		}
	}
}

func TestVerifyJSONFillsChecks(t *testing.T) {
	out, _, err := run(t, "verify", "--format", "json", "../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, want := range []string{
		`"name": "pointer_resolution"`,
		`"name": "answer_diff"`,
		`"pass": true`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("verify JSON missing %q", want)
		}
	}
}

func TestVerifyUnknownFormat(t *testing.T) {
	if _, _, err := run(t, "verify", "--format", "yaml", "../../testdata/sessions/session-minimal.jsonl"); err == nil || !strings.Contains(err.Error(), "yaml") {
		t.Errorf("err = %v, want unknown-format error naming yaml", err)
	}
}

func TestVerifyMissingFile(t *testing.T) {
	if _, _, err := run(t, "verify", "no-such-session.jsonl"); err == nil {
		t.Fatal("verify succeeded on a missing file, want error")
	}
}

func TestVerifyDiscoversNewestSession(t *testing.T) {
	root := t.TempDir()
	newest := filepath.Join(root, "proj-a", "newest.jsonl")
	writeSessionLine(t, newest, time.Now())

	out, errOut, err := run(t, "verify", "--projects-dir", root)
	if err != nil {
		t.Fatalf("verify discovery: %v", err)
	}
	if !strings.Contains(errOut, "discovered session: "+newest) {
		t.Errorf("stderr = %q, want a discovery note for %q", errOut, newest)
	}
	if !strings.Contains(out, "verdict: PASS") {
		t.Errorf("stdout = %q, want the check report", out)
	}
}

func TestViewWritesHTML(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "lastword.html")
	out, _, err := run(t, "view", "../../testdata/sessions/session-500turn.jsonl", "--out", outPath)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if got := strings.TrimSpace(out); !strings.HasSuffix(got, "lastword.html") {
		t.Errorf("view stdout = %q, want the written path", got)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("viewer file not written: %v", err)
	}
	html := string(data)
	for _, want := range []string{`id="turn-500"`, "The login flake was a stale refresh-token cache key", "<!doctype html>"} {
		if !strings.Contains(html, want) {
			t.Errorf("viewer HTML missing %q", want)
		}
	}
}

// writeSessionLine creates a minimal one-entry session file with the given
// modification time.
func writeSessionLine(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","uuid":"u1","sessionId":"` + filepath.Base(path) + `","message":{"role":"user","content":"hi"}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}
