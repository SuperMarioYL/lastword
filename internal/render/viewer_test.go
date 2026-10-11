package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/SuperMarioYL/lastword/internal/transcript"
	"github.com/SuperMarioYL/lastword/internal/verdictspan"
)

func fixtureTurns(t *testing.T) []transcript.Turn {
	t.Helper()
	s, err := transcript.Load("../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s.Turns
}

func TestWriteViewerFixture(t *testing.T) {
	span := fixtureSpan(t)
	turns := fixtureTurns(t)
	var buf bytes.Buffer
	if err := WriteViewer(&buf, span, turns); err != nil {
		t.Fatalf("WriteViewer: %v", err)
	}
	out := buf.String()

	// The verdict panel: lead, changes, results.
	for _, want := range []string{
		"The login flake was a stale refresh-token cache key",
		"src/auth/token.go",
		"[FAIL (exit 2)]",
		"go test ./...",
		"answer.results[go test ./...]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("viewer missing %q", want)
		}
	}

	// Every pointer links to an anchor, and every anchor exists.
	for _, p := range span.Pointers {
		link := fmt.Sprintf(`href="#turn-%d"`, p.Turn)
		anchor := fmt.Sprintf(`id="turn-%d"`, p.Turn)
		if !strings.Contains(out, link) {
			t.Errorf("viewer missing link %s for %s", link, p.Provenance)
		}
		if !strings.Contains(out, anchor) {
			t.Errorf("viewer missing anchor %s", anchor)
		}
	}
	// One anchor per counted turn.
	if !strings.Contains(out, `id="turn-1"`) || !strings.Contains(out, `id="turn-500"`) {
		t.Errorf("viewer missing the first or last turn anchor")
	}
}

// TestWriteViewerSelfContained pins the no-CDN contract: the file must not
// reference any external resource.
func TestWriteViewerSelfContained(t *testing.T) {
	span := fixtureSpan(t)
	var buf bytes.Buffer
	if err := WriteViewer(&buf, span, fixtureTurns(t)); err != nil {
		t.Fatalf("WriteViewer: %v", err)
	}
	out := strings.ToLower(buf.String())
	for _, banned := range []string{`src="http`, `href="http`, "<link", "@import", "<script", "url(http"} {
		if strings.Contains(out, banned) {
			t.Errorf("viewer references an external resource: %q", banned)
		}
	}
}

func TestWriteViewerEscapesTurnContent(t *testing.T) {
	// Transcript text is untrusted input into an HTML page: it must be
	// escaped, not injected.
	b := &transcript.Session{ID: "s1"}
	b.Turns = []transcript.Turn{
		{Index: 1, UUID: "u1", Kind: "user", Text: "<script>alert(1)</script>"},
	}
	span := verdictspan.Extract(b)
	var buf bytes.Buffer
	if err := WriteViewer(&buf, span, b.Turns); err != nil {
		t.Fatalf("WriteViewer: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("viewer injected raw turn text into the HTML")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("viewer did not escape the turn text:\n%s", out)
	}
}

func TestWriteViewerEmptySession(t *testing.T) {
	span := &verdictspan.Span{Schema: verdictspan.SchemaVersion, Session: "s", Answer: verdictspan.Answer{Changes: []verdictspan.Change{}, Results: []verdictspan.Result{}}}
	var buf bytes.Buffer
	if err := WriteViewer(&buf, span, nil); err != nil {
		t.Fatalf("WriteViewer: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"(no assistant text in this session)", "(no Edit/Write tool events)", "(no Bash commands ran)", "(no counted turns)"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty viewer missing %q", want)
		}
	}
}
