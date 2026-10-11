package verdictspan

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/lastword/internal/transcript"
)

// fixtureSession loads the bundled 500-turn session.
func fixtureSession(t *testing.T) *transcript.Session {
	t.Helper()
	s, err := transcript.Load("../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func findCheck(t *testing.T, checks []Check, name string) Check {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q missing from %+v", name, checks)
	return Check{}
}

func TestVerifyFixtureClean(t *testing.T) {
	s := fixtureSession(t)
	checks := Verify(s, Extract(s))
	if len(checks) != 2 {
		t.Fatalf("checks = %+v, want pointer_resolution and answer_diff", checks)
	}
	pr := findCheck(t, checks, CheckPointerResolution)
	if !pr.Pass || pr.Detail != "14/14 pointers resolve" {
		t.Errorf("pointer_resolution = %+v, want pass with 14/14", pr)
	}
	ad := findCheck(t, checks, CheckAnswerDiff)
	if !ad.Pass || ad.Detail != "7 files claimed, 7 edited, 0 unclaimed" {
		t.Errorf("answer_diff = %+v, want pass with 7/7/0", ad)
	}
}

func TestVerifyEmptySpan(t *testing.T) {
	// A session with no claims: both checks pass on nothing to violate.
	b := &turnBuilder{}
	b.add("user", "hello", nil, nil)
	s := &transcript.Session{ID: "s1", Turns: b.turns}
	checks := Verify(s, Extract(s))
	for _, c := range checks {
		if !c.Pass {
			t.Errorf("%s = %+v, want pass on a span with no claims", c.Name, c)
		}
	}
}

// mutate applies fn to the extracted fixture span and verifies it against
// the untouched fixture session, returning the named check.
func mutate(t *testing.T, check string, fn func(span *Span)) Check {
	t.Helper()
	s := fixtureSession(t)
	span := Extract(s)
	fn(span)
	return findCheck(t, Verify(s, span), check)
}

func TestVerifyFailsOnPhantomTurn(t *testing.T) {
	c := mutate(t, CheckPointerResolution, func(span *Span) { span.Pointers[0].Turn = 9999 })
	if c.Pass || !strings.Contains(c.Detail, "turn 9999 does not resolve") {
		t.Errorf("pointer_resolution = %+v, want failure naming turn 9999", c)
	}
}

func TestVerifyFailsOnWrongUUID(t *testing.T) {
	c := mutate(t, CheckPointerResolution, func(span *Span) { span.Pointers[0].UUID = "00000000-dead-beef-0000-000000000000" })
	if c.Pass || !strings.Contains(c.Detail, "does not resolve") {
		t.Errorf("pointer_resolution = %+v, want failure on the uuid mismatch", c)
	}
}

func TestVerifyFailsOnFabricatedSnippet(t *testing.T) {
	c := mutate(t, CheckPointerResolution, func(span *Span) { span.Pointers[0].Snippet = "text this transcript never contained" })
	if c.Pass || !strings.Contains(c.Detail, "snippet not found in turn 500") {
		t.Errorf("pointer_resolution = %+v, want failure naming the turn", c)
	}
}

func TestVerifyFailsOnInflatedCount(t *testing.T) {
	c := mutate(t, CheckAnswerDiff, func(span *Span) { span.Answer.Changes[0].Added += 100 })
	if c.Pass || !strings.Contains(c.Detail, "claimed +106/-6") {
		t.Errorf("answer_diff = %+v, want failure quoting both counts", c)
	}
}

func TestVerifyFailsOnUnclaimedFile(t *testing.T) {
	c := mutate(t, CheckAnswerDiff, func(span *Span) {
		span.Answer.Changes = append(span.Answer.Changes, Change{File: "ghost.go", Added: 1})
	})
	if c.Pass || !strings.Contains(c.Detail, "claimed but never edited: ghost.go") {
		t.Errorf("answer_diff = %+v, want failure naming the phantom file", c)
	}
}

func TestVerifyFailsOnDroppedFile(t *testing.T) {
	c := mutate(t, CheckAnswerDiff, func(span *Span) { span.Answer.Changes = span.Answer.Changes[1:] })
	if c.Pass || !strings.Contains(c.Detail, "edited but unclaimed: src/auth/token.go") {
		t.Errorf("answer_diff = %+v, want failure naming the dropped file", c)
	}
}

func TestVerifyTruncatedSnippetAtLineStart(t *testing.T) {
	// A silent turn that edits two files with long payloads: the second
	// file's quote is a prefix of ITS payload but not of the whole turn
	// content. The truncation rule must still resolve it — the quote
	// appears at a line boundary of the turn.
	b := &turnBuilder{}
	b.add("user", "go", nil, nil)
	long1 := strings.Repeat("one ", 60) // 240 runes
	long2 := strings.Repeat("two ", 60)
	b.add("assistant", "", []transcript.ToolUse{
		editUse("e1", "first.go", long1, long1),
		editUse("e2", "second.go", long2, long2),
	}, nil)
	s := &transcript.Session{ID: "s1", Turns: b.turns}
	span := Extract(s)
	// Both edit snippets were cut (payloads exceed the cap) and carry the
	// truncation marker.
	for _, p := range span.Pointers {
		if p.Kind == PointerEdit && !strings.HasSuffix(p.Snippet, truncationMark) {
			t.Fatalf("edit snippet = %q, want the truncation marker on a long payload", p.Snippet)
		}
	}
	checks := Verify(s, span)
	if c := findCheck(t, checks, CheckPointerResolution); !c.Pass {
		t.Errorf("pointer_resolution = %+v, want the truncated multi-payload quotes to resolve", c)
	}
}

func TestVerifyTruncatedSnippetMissingFromTurn(t *testing.T) {
	// A truncated quote that appears at no line boundary of the turn must
	// still fail: swap one truncated snippet for a cut of unrelated text.
	s := fixtureSession(t)
	span := Extract(s)
	fake := strings.Repeat("narration ", 13) + "…"
	span.Pointers[0].Snippet = fake
	c := findCheck(t, Verify(s, span), CheckPointerResolution)
	if c.Pass || !strings.Contains(c.Detail, "snippet not found") {
		t.Errorf("pointer_resolution = %+v, want failure on a fabricated truncated quote", c)
	}
}

func TestVerifyEmptySnippetIsVacuous(t *testing.T) {
	// The recorded demo carries an empty result snippet (gofmt prints
	// nothing). An empty quote asserts nothing, so it cannot be violated.
	s := fixtureSession(t)
	span := Extract(s)
	for i := range span.Pointers {
		if span.Pointers[i].Provenance == "answer.results[gofmt -w .]" {
			span.Pointers[i].Snippet = ""
		}
	}
	if c := findCheck(t, Verify(s, span), CheckPointerResolution); !c.Pass {
		t.Errorf("pointer_resolution = %+v, want the empty gofmt snippet to pass", c)
	}
}
