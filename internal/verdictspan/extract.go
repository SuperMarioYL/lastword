package verdictspan

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/SuperMarioYL/lastword/internal/transcript"
)

// snippetWidth is the hard cap on a pointer snippet; anything longer is cut
// and marked with an ellipsis so the quote stays verbatim and compact.
const snippetWidth = 120

var exitCodeRe = regexp.MustCompile(`Exit code (\d+)`)

// Extract derives a verdict span from a parsed session using deterministic
// rules only — no model calls, no heuristics beyond the format itself:
//
//   - answer.lead is the leading paragraph of the last assistant turn that
//     carries text (a tool-only final turn has no paragraph to quote);
//   - changes aggregate every Edit/Write tool event per file, in
//     first-appearance order — Edit counts the lines of new/old strings,
//     Write counts the lines of the written content and knows nothing about
//     what the file held before;
//   - results hold the LAST observed outcome of each distinct Bash command
//     (compared after trimming surrounding whitespace; a command re-run
//     later in the session replaces its earlier outcome).
//
// Every claim gets a pointer to the turn that produced it: the final
// assistant turn, the last edit event per file, and the tool_result turn of
// each command's last run. Checks are left to verify (milestone m2).
func Extract(s *transcript.Session) *Span {
	span := &Span{
		Schema:  SchemaVersion,
		Session: s.ID,
		Answer:  Answer{Changes: []Change{}, Results: []Result{}},
	}

	lead, finalTurn := extractLead(s.Turns)
	span.Answer.Lead = lead
	if finalTurn != nil {
		span.Pointers = append(span.Pointers, Pointer{
			Turn:       finalTurn.Index,
			UUID:       finalTurn.UUID,
			Kind:       PointerFinal,
			Snippet:    snippet(lead),
			Provenance: "answer.lead",
		})
	}

	// tool_use id -> the turn that issued it, for pairing results with
	// their commands.
	usesByID := make(map[string]useSite)
	for i := range s.Turns {
		for _, use := range s.Turns[i].ToolUses {
			usesByID[use.ID] = useSite{turn: &s.Turns[i], use: use}
		}
	}

	// File changes: first-appearance order, summed line counts, pointer to
	// the last edit event per file.
	changeAt := make(map[string]int)     // file -> index into Answer.Changes
	lastEdit := make(map[string]useSite) // file -> latest Edit/Write event
	for i := range s.Turns {
		for _, use := range s.Turns[i].ToolUses {
			file := use.Input.FilePath
			if file == "" {
				continue
			}
			var added, removed int
			switch use.Name {
			case "Edit":
				added, removed = lineCount(use.Input.NewString), lineCount(use.Input.OldString)
			case "Write":
				added, removed = lineCount(use.Input.Content), 0
			default:
				continue
			}
			idx, seen := changeAt[file]
			if !seen {
				span.Answer.Changes = append(span.Answer.Changes, Change{File: file})
				idx = len(span.Answer.Changes) - 1
				changeAt[file] = idx
			}
			span.Answer.Changes[idx].Added += added
			span.Answer.Changes[idx].Removed += removed
			lastEdit[file] = useSite{turn: &s.Turns[i], use: use}
		}
	}
	for _, change := range span.Answer.Changes {
		site := lastEdit[change.File]
		span.Pointers = append(span.Pointers, Pointer{
			Turn:       site.turn.Index,
			UUID:       site.turn.UUID,
			Kind:       PointerEdit,
			Snippet:    editSnippet(site),
			Provenance: "answer.changes[" + change.File + "]",
		})
	}

	// Command results: the last outcome of each distinct Bash command, in
	// first-appearance order, pointed at the tool_result turn of that run.
	resultAt := make(map[string]int) // cmd -> index into Answer.Results
	lastRun := make(map[string]resultSite)
	for i := range s.Turns {
		for _, res := range s.Turns[i].ToolResults {
			site, ok := usesByID[res.ToolUseID]
			if !ok || site.use.Name != "Bash" {
				continue
			}
			cmd := strings.TrimSpace(site.use.Input.Command)
			if cmd == "" {
				continue
			}
			outcome := Result{
				Kind:     "command",
				Cmd:      cmd,
				OK:       !res.IsError,
				ExitCode: parseExitCode(res.Content),
			}
			idx, seen := resultAt[cmd]
			if !seen {
				span.Answer.Results = append(span.Answer.Results, outcome)
				resultAt[cmd] = len(span.Answer.Results) - 1
			} else {
				span.Answer.Results[idx] = outcome
			}
			lastRun[cmd] = resultSite{turn: &s.Turns[i], result: res}
		}
	}
	for _, res := range span.Answer.Results {
		site := lastRun[res.Cmd]
		span.Pointers = append(span.Pointers, Pointer{
			Turn:       site.turn.Index,
			UUID:       site.turn.UUID,
			Kind:       PointerResult,
			Snippet:    snippet(site.result.Content),
			Provenance: "answer.results[" + res.Cmd + "]",
		})
	}

	return span
}

// useSite is a tool_use block together with the turn that issued it.
type useSite struct {
	turn *transcript.Turn
	use  transcript.ToolUse
}

// resultSite is a tool_result block together with the turn that carries it.
type resultSite struct {
	turn   *transcript.Turn
	result transcript.ToolResult
}

// extractLead returns the leading paragraph of the last assistant turn that
// carries text, plus that turn.
func extractLead(turns []transcript.Turn) (string, *transcript.Turn) {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Kind != "assistant" {
			continue
		}
		text := strings.TrimSpace(turns[i].Text)
		if text == "" {
			continue
		}
		paragraph, _, _ := strings.Cut(text, "\n\n")
		return strings.TrimSpace(paragraph), &turns[i]
	}
	return "", nil
}

// lineCount counts the lines of a diff payload: an empty payload is zero
// lines, anything else is its newline-separated segment count after one
// trailing newline is dropped (git-style: a final newline terminates the
// last line rather than starting a new one).
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSuffix(s, "\n"), "\n"))
}

// parseExitCode reads an exit code out of result free text. Bash failures
// usually print "Exit code N"; successes and some failures print none, in
// which case there is nothing to report and the field stays null.
func parseExitCode(content string) *int {
	m := exitCodeRe.FindStringSubmatch(content)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	return &n
}

// snippet quotes the first two non-empty lines of a turn's content, hard
// truncated at snippetWidth characters.
func snippet(text string) string {
	var kept []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		kept = append(kept, l)
		if len(kept) == 2 {
			break
		}
	}
	s := strings.Join(kept, "\n")
	if len(s) > snippetWidth {
		return s[:snippetWidth] + "…"
	}
	return s
}

// editSnippet quotes the edit turn itself: its narration when the assistant
// said something, otherwise the first lines of the payload it wrote.
func editSnippet(site useSite) string {
	if strings.TrimSpace(site.turn.Text) != "" {
		return snippet(site.turn.Text)
	}
	if site.use.Name == "Edit" {
		return snippet(site.use.Input.NewString)
	}
	return snippet(site.use.Input.Content)
}
