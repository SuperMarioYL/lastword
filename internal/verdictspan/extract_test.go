package verdictspan

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SuperMarioYL/lastword/internal/transcript"
)

// mk is a shorthand for building inline sessions in tests.
type turnBuilder struct {
	turns []transcript.Turn
}

func (b *turnBuilder) add(kind, text string, uses []transcript.ToolUse, results []transcript.ToolResult) *transcript.Turn {
	b.turns = append(b.turns, transcript.Turn{
		Index:       len(b.turns) + 1,
		Kind:        kind,
		Text:        text,
		ToolUses:    uses,
		ToolResults: results,
	})
	return &b.turns[len(b.turns)-1]
}

func editUse(id, file, old, new string) transcript.ToolUse {
	return transcript.ToolUse{ID: id, Name: "Edit", Input: transcript.ToolInput{
		FilePath: file, OldString: old, NewString: new,
	}}
}

func writeUse(id, file, content string) transcript.ToolUse {
	return transcript.ToolUse{ID: id, Name: "Write", Input: transcript.ToolInput{
		FilePath: file, Content: content,
	}}
}

func bashUse(id, cmd string) transcript.ToolUse {
	return transcript.ToolUse{ID: id, Name: "Bash", Input: transcript.ToolInput{Command: cmd}}
}

func resultFor(id, content string, isErr bool) transcript.ToolResult {
	return transcript.ToolResult{ToolUseID: id, Content: content, IsError: isErr}
}

func TestExtractLead(t *testing.T) {
	// The final assistant turn is tool-only: the lead falls back to the
	// last assistant turn that actually says something.
	b := &turnBuilder{}
	b.add("user", "fix it", nil, nil)
	b.add("assistant", "First paragraph of the wrap-up.\n\nSecond paragraph, ignored for the lead.", nil, nil)
	b.add("assistant", "", []transcript.ToolUse{bashUse("t1", "echo done")}, nil)
	b.add("user", "", nil, []transcript.ToolResult{resultFor("t1", "done", false)})

	span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})
	want := "First paragraph of the wrap-up."
	if span.Answer.Lead != want {
		t.Errorf("lead = %q, want %q", span.Answer.Lead, want)
	}
	final := span.Pointers[0]
	if final.Kind != PointerFinal || final.Turn != 2 {
		t.Errorf("final pointer = %+v, want kind final at turn 2", final)
	}
	if final.Snippet != want {
		t.Errorf("final snippet = %q, want the lead's first line", final.Snippet)
	}
}

func TestExtractLeadNoAssistantText(t *testing.T) {
	b := &turnBuilder{}
	b.add("user", "hello", nil, nil)
	span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})
	if span.Answer.Lead != "" {
		t.Errorf("lead = %q, want empty", span.Answer.Lead)
	}
	for _, p := range span.Pointers {
		if p.Kind == PointerFinal {
			t.Errorf("unexpected final pointer for a session with no assistant text: %+v", p)
		}
	}
}

func TestExtractChanges(t *testing.T) {
	b := &turnBuilder{}
	b.add("user", "go", nil, nil)
	// Two edits on the same file in one turn, one earlier edit on another
	// file, one Write, and noise that must not count (Read, empty path).
	b.add("assistant", "", []transcript.ToolUse{
		{Name: "Read", ID: "r1", Input: transcript.ToolInput{FilePath: "other.go"}},
		editUse("e0", "first.go", "a\nb", "x\ny\nz"),
	}, nil)
	b.add("user", "", nil, []transcript.ToolResult{resultFor("e0", "updated", false)})
	b.add("assistant", "second pass", []transcript.ToolUse{
		editUse("e1", "second.go", "one\nline\nthree", "one\nline\nthree\nfour\nfive"),
		editUse("e2", "second.go", "old", "new"),
		writeUse("w1", "fresh.go", "l1\nl2\nl3"),
		editUse("e3", "", "x", "y"), // no file: skipped
	}, nil)

	span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})
	want := []Change{
		{File: "first.go", Added: 3, Removed: 2},
		{File: "second.go", Added: 6, Removed: 4}, // 4+1 added, 3+1 removed
		{File: "fresh.go", Added: 3, Removed: 0},
	}
	if !reflect.DeepEqual(span.Answer.Changes, want) {
		t.Errorf("changes = %+v, want %+v", span.Answer.Changes, want)
	}
	// Edit pointers anchor each file's latest event: first.go at turn 2,
	// second.go and fresh.go at turn 4.
	byProv := map[string]Pointer{}
	for _, p := range span.Pointers {
		if p.Kind == PointerEdit {
			byProv[p.Provenance] = p
		}
	}
	for prov, wantTurn := range map[string]int{
		"answer.changes[first.go]":  2,
		"answer.changes[second.go]": 4,
		"answer.changes[fresh.go]":  4,
	} {
		if p, ok := byProv[prov]; !ok || p.Turn != wantTurn {
			t.Errorf("%s = %+v, want a pointer at turn %d", prov, p, wantTurn)
		}
	}
	// A narrated edit turn quotes the narration for every event it holds;
	// a silent turn quotes the payload it wrote.
	if sn := byProv["answer.changes[second.go]"].Snippet; sn != "second pass" {
		t.Errorf("narrated edit snippet = %q, want the turn text", sn)
	}
	if sn := byProv["answer.changes[fresh.go]"].Snippet; sn != "second pass" {
		t.Errorf("write in a narrated turn = %q, want the narration", sn)
	}
	if sn := byProv["answer.changes[first.go]"].Snippet; sn != "x\ny" {
		t.Errorf("silent edit snippet = %q, want the first lines of new_string", sn)
	}
}

func TestExtractResults(t *testing.T) {
	b := &turnBuilder{}
	b.add("user", "go", nil, nil)
	b.add("assistant", "", []transcript.ToolUse{
		bashUse("c1", "make check"),
		bashUse("c2", "go build ./..."),
		bashUse("c3", ""), // empty command: skipped
	}, nil)
	b.add("user", "", nil, []transcript.ToolResult{
		resultFor("c1", "Exit code 1\nmake: *** [check] Error 1", true),
		resultFor("c2", "ok", false),
		resultFor("ghost", "no matching use", false), // orphan: skipped
	})
	// Same command re-run with trailing whitespace: same identity, and the
	// later outcome replaces the earlier one.
	b.add("assistant", "", []transcript.ToolUse{bashUse("c4", "make check ")}, nil)
	b.add("user", "", nil, []transcript.ToolResult{resultFor("c4", "all checks passed", false)})
	// A different command failing without a printed exit code.
	b.add("assistant", "", []transcript.ToolUse{bashUse("c5", "npm run build")}, nil)
	b.add("user", "", nil, []transcript.ToolResult{resultFor("c5", "build failed, no code printed", true)})

	span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})
	want := []Result{
		{Kind: "command", Cmd: "make check", OK: true, ExitCode: nil},
		{Kind: "command", Cmd: "go build ./...", OK: true, ExitCode: nil},
		{Kind: "command", Cmd: "npm run build", OK: false, ExitCode: nil},
	}
	if !reflect.DeepEqual(span.Answer.Results, want) {
		t.Errorf("results = %+v, want %+v", span.Answer.Results, want)
	}
	if span.Answer.Results[0].ExitCode != nil {
		t.Errorf("retried command kept the failed run's exit code: %+v", span.Answer.Results[0])
	}
	// The result pointer anchors the LATEST outcome of "make check" (turn 5).
	for _, p := range span.Pointers {
		if p.Kind == PointerResult && p.Provenance == "answer.results[make check]" {
			if p.Turn != 5 || p.Snippet != "all checks passed" {
				t.Errorf("make check pointer = %+v, want turn 5 quoting the passing run", p)
			}
		}
	}
}

func TestExtractFailedExitCode(t *testing.T) {
	b := &turnBuilder{}
	b.add("assistant", "", []transcript.ToolUse{bashUse("c1", "go test ./...")}, nil)
	b.add("user", "", nil, []transcript.ToolResult{
		resultFor("c1", "Exit code 1\n--- FAIL: TestX", true),
	})
	span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})
	res := span.Answer.Results[0]
	if res.OK || res.ExitCode == nil || *res.ExitCode != 1 {
		t.Errorf("result = %+v, want failed with exit code 1", res)
	}
}

func TestExtract500TurnFixture(t *testing.T) {
	s, err := transcript.Load("../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Turns) != 500 {
		t.Fatalf("fixture has %d counted turns, want 500", len(s.Turns))
	}
	span := Extract(s)

	if span.Schema != SchemaVersion || span.Session != s.ID {
		t.Errorf("span header = %s %s", span.Schema, span.Session)
	}
	if want := "The login flake was a stale refresh-token cache key: rotating a session changed the in-memory key but never invalidated the old entry, so roughly one in five refreshes read a token the issuer had already rejected."; span.Answer.Lead != want {
		t.Errorf("lead = %q", span.Answer.Lead)
	}
	if len(span.Answer.Changes) != 7 {
		t.Errorf("changes = %d files, want 7", len(span.Answer.Changes))
	}
	first := span.Answer.Changes[0]
	if first.File != "src/auth/token.go" || first.Added != 6 || first.Removed != 6 {
		t.Errorf("first change = %+v, want src/auth/token.go +6 -6", first)
	}

	// go test ./... failed first, passed on the retry: the span claims the
	// last outcome only.
	byCmd := map[string]Result{}
	for _, r := range span.Answer.Results {
		byCmd[r.Cmd] = r
	}
	if r := byCmd["go test ./..."]; !r.OK {
		t.Errorf("go test outcome = %+v, want the final passing run", r)
	}
	if r := byCmd["npm --prefix web run build"]; r.OK || r.ExitCode == nil || *r.ExitCode != 2 {
		t.Errorf("npm build outcome = %+v, want failed with exit code 2", r)
	}

	// 1 final + 7 edit + 6 result pointers, final anchored at turn 500.
	if len(span.Pointers) != 1+7+6 {
		t.Errorf("pointers = %d, want 14", len(span.Pointers))
	}
	if p := span.Pointers[0]; p.Kind != PointerFinal || p.Turn != 500 {
		t.Errorf("final pointer = %+v, want turn 500", p)
	}
	if span.Checks != nil {
		t.Errorf("checks = %+v, want nil until verify (m2)", span.Checks)
	}
}

func TestExtractMinimalFixture(t *testing.T) {
	s, err := transcript.Load("../../testdata/sessions/session-minimal.jsonl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	span := Extract(s)
	if want := "Added public/status.html with a plain 200 OK page."; span.Answer.Lead != want {
		t.Errorf("lead = %q, want %q", span.Answer.Lead, want)
	}
	if len(span.Answer.Changes) != 0 || len(span.Answer.Results) != 0 {
		t.Errorf("changes/results = %d/%d, want 0/0", len(span.Answer.Changes), len(span.Answer.Results))
	}
	if len(span.Pointers) != 1 {
		t.Errorf("pointers = %d, want the final pointer only", len(span.Pointers))
	}
}

func TestSnippetTruncatesAtRuneBoundary(t *testing.T) {
	// A byte-indexed cut used to split multi-byte runes: 118 ASCII bytes
	// then CJK put the old 120-BYTE cap inside a rune, and encoding/json
	// coerced the broken tail to U+FFFD — corrupting the "verbatim" quote.
	// The cap now counts runes, so the quote stays valid UTF-8 and a true
	// prefix of the source.
	for name, lead := range map[string]string{
		"ascii-cjk-boundary": "a" + strings.Repeat("x", 117) + "这个问题的结论是修复了缓存键失效逻辑",
		"cjk-only":           strings.Repeat("这个问题的根本原因是会话刷新时缓存键失效了，需要修正失效逻辑。", 8),
	} {
		t.Run(name, func(t *testing.T) {
			b := &turnBuilder{}
			b.add("user", "总结", nil, nil)
			b.add("assistant", lead, nil, nil)
			span := Extract(&transcript.Session{ID: "s1", Turns: b.turns})

			snip := span.Pointers[0].Snippet
			if !utf8.ValidString(snip) {
				t.Fatalf("snippet is not valid UTF-8: %q", snip)
			}
			if !strings.HasSuffix(snip, "…") {
				t.Fatalf("long snippet = %q, want the truncation marker", snip)
			}
			core := strings.TrimSuffix(snip, "…")
			if got := utf8.RuneCountInString(core); got != snippetWidth {
				t.Errorf("snippet core = %d runes, want %d", got, snippetWidth)
			}
			if !strings.HasPrefix(lead, core) {
				t.Errorf("snippet core is not a prefix of the lead:\ncore %q\nlead %q", core, lead)
			}
		})
	}
}
