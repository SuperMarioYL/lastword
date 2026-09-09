package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touch(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestLatest(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Local()
	touch(t, filepath.Join(root, "proj-a", "old.jsonl"), now.Add(-2*time.Hour))
	touch(t, filepath.Join(root, "proj-a", "newer.jsonl"), now.Add(-1*time.Hour))
	// Nested transcripts (subagents) must never win, however fresh.
	touch(t, filepath.Join(root, "proj-a", "nested", "sub.jsonl"), now.Add(time.Minute))
	touch(t, filepath.Join(root, "proj-b", "newest.jsonl"), now)
	touch(t, filepath.Join(root, "loose.txt"), now) // not a session file

	got, err := Latest(root)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if want := filepath.Join(root, "proj-b", "newest.jsonl"); got != want {
		t.Errorf("Latest = %q, want %q", got, want)
	}
}

func TestLatestTieBreaksByName(t *testing.T) {
	root := t.TempDir()
	same := time.Now().Local().Truncate(time.Second)
	touch(t, filepath.Join(root, "proj", "beta.jsonl"), same)
	touch(t, filepath.Join(root, "proj", "alpha.jsonl"), same)

	got, err := Latest(root)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if want := filepath.Join(root, "proj", "alpha.jsonl"); got != want {
		t.Errorf("Latest = %q, want the lexicographically smaller %q", got, want)
	}
}

func TestLatestEmpty(t *testing.T) {
	root := t.TempDir()
	if _, err := Latest(root); err == nil {
		t.Fatal("Latest succeeded on an empty projects dir, want error")
	}
}
