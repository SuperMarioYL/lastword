package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) *Session {
	t.Helper()
	s, err := Load(filepath.Join("..", "..", "testdata", "sessions", name))
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return s
}

func TestLoadEdgesFixture(t *testing.T) {
	s := loadFixture(t, "session-edges.jsonl")

	// 16 lines: 1 summary + 2 sidechain skipped, 13 counted turns.
	if len(s.Turns) != 13 {
		t.Fatalf("counted turns = %d, want 13", len(s.Turns))
	}
	if s.ID != "e7b1c4d2-9a30-4e58-b1f0-6c2d8e3a5b77" {
		t.Errorf("session ID = %q, want the recorded sessionId", s.ID)
	}
	for i, turn := range s.Turns {
		if turn.Index != i+1 {
			t.Errorf("turn %d has Index %d, want %d", i, turn.Index, i+1)
		}
	}

	// Turn 3 narrates and carries two Edits on the same file.
	edits := s.Turns[2]
	if edits.Kind != "assistant" || edits.Text != "First pass done." {
		t.Errorf("turn 3 = %s %q, want assistant narration", edits.Kind, edits.Text)
	}
	if len(edits.ToolUses) != 2 || edits.ToolUses[0].Name != "Edit" || edits.ToolUses[1].Name != "Edit" {
		t.Fatalf("turn 3 tool uses = %+v, want two Edits", edits.ToolUses)
	}
	if got := edits.ToolUses[0].Input.FilePath; got != "src/a.go" {
		t.Errorf("edit file_path = %q, want src/a.go", got)
	}

	// Tool results pair by tool_use_id; a failing Bash result keeps its text.
	fail := s.Turns[6]
	if len(fail.ToolResults) != 1 || !fail.ToolResults[0].IsError {
		t.Fatalf("turn 7 = %+v, want one failed tool_result", fail.ToolResults)
	}
	if want := "toolu_0004"; fail.ToolResults[0].ToolUseID != want {
		t.Errorf("tool_use_id = %q, want %q", fail.ToolResults[0].ToolUseID, want)
	}

	// The orphan tool_result is still parsed (it belongs to the transcript);
	// ignoring it is extraction's decision, not the parser's.
	orphan := s.Turns[12]
	if len(orphan.ToolResults) != 1 || orphan.ToolResults[0].ToolUseID != "toolu_ghost" {
		t.Errorf("orphan tool_result missing: %+v", orphan.ToolResults)
	}
}

func TestLoadInlineShapes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	content := `{"type":"user","uuid":"u1","message":{"role":"user","content":"plain string prompt"}}

{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"internal reasoning"},{"type":"text","text":"answer line 1"},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"make check"}},{"type":"future_block","payload":"whatever comes next"}]}}
{"type":"user","uuid":"u2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"Exit code 1"},{"type":"text","text":"make failed"}],"is_error":true}]}}
{"type":"progress","progress":1}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Turns) != 3 {
		t.Fatalf("counted turns = %d, want 3 (progress skipped, blank line skipped)", len(s.Turns))
	}
	if s.Turns[0].Text != "plain string prompt" {
		t.Errorf("string content = %q", s.Turns[0].Text)
	}
	if s.Turns[1].Text != "answer line 1" {
		t.Errorf("text = %q, want thinking and unknown blocks excluded", s.Turns[1].Text)
	}
	if len(s.Turns[1].ToolUses) != 1 || s.Turns[1].ToolUses[0].Input.Command != "make check" {
		t.Errorf("tool uses = %+v", s.Turns[1].ToolUses)
	}
	if got := s.Turns[2].ToolResults[0].Content; got != "Exit code 1\nmake failed" {
		t.Errorf("block-array result content = %q", got)
	}
	// No sessionId anywhere: the file name is the fallback identity.
	if s.ID != "s" {
		t.Errorf("fallback session ID = %q, want file stem", s.ID)
	}
}

func TestLoadMalformedMiddleLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	// A malformed line BEFORE the end must fail the load: only a torn final
	// entry (a write that never completed) is tolerated.
	if err := os.WriteFile(path, []byte("{\"type\":\"user\"}\nnot json at all\n{\"type\":\"assistant\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load succeeded on a malformed middle line, want error")
	}
	if !strings.Contains(err.Error(), "2:") {
		t.Errorf("error = %v, want the offending line number", err)
	}
}

func TestLoadTornFinalLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torn.jsonl")
	// A session still being written: two complete entries, then a final
	// entry cut mid-JSON. The torn entry never completed, so it is skipped
	// and the completed turns load.
	valid1 := `{"type":"user","uuid":"u1","sessionId":"torn","message":{"role":"user","content":"hello"}}`
	valid2 := `{"type":"assistant","uuid":"a1","sessionId":"torn","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`
	if err := os.WriteFile(path, []byte(valid1+"\n"+valid2+"\n"+valid2[:60]), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Turns) != 2 {
		t.Fatalf("counted turns = %d, want the 2 completed entries", len(s.Turns))
	}
	if s.Turns[1].Text != "done" {
		t.Errorf("turn 2 text = %q, want the completed assistant turn", s.Turns[1].Text)
	}
}

func TestLoadBlankFinalLineAfterTorn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "torn-blank.jsonl")
	// The torn line is the last NON-EMPTY line even when blank lines follow.
	if err := os.WriteFile(path, []byte("{\"type\":\"user\",\"uuid\":\"u1\"}\n{\"type\":\"as"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Turns) != 1 {
		t.Fatalf("counted turns = %d, want 1", len(s.Turns))
	}
}
