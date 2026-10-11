package verdictspan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SuperMarioYL/lastword/internal/transcript"
)

// truncationMark is appended to snippets that were cut at the width cap;
// verify treats it as the documented truncation marker.
const truncationMark = "…"

// Verify evaluates a span against the session it was extracted from, using
// the check semantics fixed by docs/verdict-span-v0.md:
//
//   - pointer_resolution — every pointer's turn and uuid resolve to a real
//     counted turn, and its snippet appears verbatim in that turn's content.
//     A snippet ending in the truncation marker was cut from the start of
//     its source, so it must appear at a line boundary of the turn; an
//     empty snippet is vacuous containment and cannot be violated.
//   - answer_diff — the claimed changes equal the transcript's actual
//     Edit/Write aggregation: every claimed file edited, every edited file
//     claimed, and the added/removed counts equal.
//
// Both checks are always reported once a span is evaluated; a command that
// evaluates them exits non-zero when any check fails.
func Verify(s *transcript.Session, span *Span) []Check {
	return []Check{verifyPointers(s, span), verifyDiff(s, span)}
}

// verifyPointers checks pointer resolution for every claim.
func verifyPointers(s *transcript.Session, span *Span) Check {
	byIndex := make(map[int]*transcript.Turn, len(s.Turns))
	for i := range s.Turns {
		byIndex[s.Turns[i].Index] = &s.Turns[i]
	}
	c := Check{Name: CheckPointerResolution, Pass: true}
	var problems []string
	resolved := 0
	for _, p := range span.Pointers {
		turn := byIndex[p.Turn]
		if turn == nil || turn.UUID != p.UUID {
			problems = append(problems, fmt.Sprintf("%s: turn %d does not resolve", p.Provenance, p.Turn))
			continue
		}
		if !snippetInTurn(p.Snippet, turn) {
			problems = append(problems, fmt.Sprintf("%s: snippet not found in turn %d", p.Provenance, p.Turn))
			continue
		}
		resolved++
	}
	if len(problems) > 0 {
		c.Pass = false
		sort.Strings(problems)
		c.Detail = fmt.Sprintf("%d/%d pointers resolve; %s", resolved, len(span.Pointers), strings.Join(problems, "; "))
		return c
	}
	c.Detail = fmt.Sprintf("%d/%d pointers resolve", resolved, len(span.Pointers))
	return c
}

// verifyDiff checks the claimed changes against the transcript's own
// Edit/Write aggregation.
func verifyDiff(s *transcript.Session, span *Span) Check {
	actual, _ := aggregateChanges(s.Turns)
	want := make(map[string]Change, len(actual))
	for _, c := range actual {
		want[c.File] = c
	}
	got := make(map[string]Change, len(span.Answer.Changes))
	for _, c := range span.Answer.Changes {
		got[c.File] = c
	}

	c := Check{Name: CheckAnswerDiff, Pass: true}
	var problems []string
	for file, claim := range got {
		w, ok := want[file]
		if !ok {
			problems = append(problems, "claimed but never edited: "+file)
			continue
		}
		if w.Added != claim.Added || w.Removed != claim.Removed {
			problems = append(problems, fmt.Sprintf("%s: claimed +%d/-%d, transcript shows +%d/-%d",
				file, claim.Added, claim.Removed, w.Added, w.Removed))
		}
	}
	unclaimed := 0
	for file := range want {
		if _, ok := got[file]; !ok {
			unclaimed++
			problems = append(problems, "edited but unclaimed: "+file)
		}
	}
	if len(problems) > 0 {
		c.Pass = false
		sort.Strings(problems)
	}
	detail := fmt.Sprintf("%d files claimed, %d edited, %d unclaimed", len(got), len(want), unclaimed)
	if len(problems) > 0 {
		detail += "; " + strings.Join(problems, "; ")
	}
	c.Detail = detail
	return c
}

// turnContent is everything a turn "says": its text plus every tool payload
// and result it carries — the universe a pointer snippet must quote from.
// Empty parts are dropped so a quote of the first payload is also a prefix
// of the whole content.
func turnContent(t *transcript.Turn) string {
	var parts []string
	if t.Text != "" {
		parts = append(parts, t.Text)
	}
	for _, u := range t.ToolUses {
		for _, field := range []string{u.Input.OldString, u.Input.NewString, u.Input.Content, u.Input.Command} {
			if field != "" {
				parts = append(parts, field)
			}
		}
	}
	for _, r := range t.ToolResults {
		if r.Content != "" {
			parts = append(parts, r.Content)
		}
	}
	return strings.Join(parts, "\n")
}

// snippetInTurn reports whether snippet is a verbatim quote from the turn.
// A snippet cut at the width cap carries the truncation marker and must
// appear at a line boundary of the turn's content (it quotes the start of
// its source); a full snippet may appear anywhere verbatim; an empty
// snippet is vacuous — it asserts nothing, so it cannot be violated.
func snippetInTurn(snippet string, t *transcript.Turn) bool {
	if snippet == "" {
		return true
	}
	content := turnContent(t)
	if strings.HasSuffix(snippet, truncationMark) {
		return containsAtLineStart(content, strings.TrimSuffix(snippet, truncationMark))
	}
	return strings.Contains(content, snippet)
}

// containsAtLineStart reports whether s occurs in content at the start of a
// line — position zero or right after a newline. Extraction always quotes
// from the start of a line, so a truncated quote must land on one too.
func containsAtLineStart(content, s string) bool {
	for idx := 0; ; {
		i := strings.Index(content[idx:], s)
		if i < 0 {
			return false
		}
		pos := idx + i
		if pos == 0 || content[pos-1] == '\n' {
			return true
		}
		idx = pos + 1
	}
}
