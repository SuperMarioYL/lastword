package render

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/SuperMarioYL/lastword/internal/transcript"
	"github.com/SuperMarioYL/lastword/internal/verdictspan"
)

func fixtureSpan(t *testing.T) *verdictspan.Span {
	t.Helper()
	s, err := transcript.Load("../../testdata/sessions/session-500turn.jsonl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return verdictspan.Extract(s)
}

func TestWriteJSONRoundTrip(t *testing.T) {
	span := fixtureSpan(t)
	var buf bytes.Buffer
	if err := WriteJSON(&buf, span); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"schema": "verdict-span/v0"`) {
		t.Errorf("output missing the schema marker:\n%s", buf.String())
	}
	var back verdictspan.Span
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(span, &back) {
		t.Errorf("JSON round-trip changed the span:\n got %+v\nwant %+v", &back, span)
	}
}

func TestWriteTextPanel(t *testing.T) {
	span := fixtureSpan(t)
	var buf bytes.Buffer
	if err := WriteText(&buf, span); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"verdict-span/v0",
		"The login flake was a stale refresh-token cache key",
		"src/auth/token.go",
		"[ok] go test ./...",
		"[FAIL (exit 2)] npm --prefix web run build",
		"#500  final   answer.lead",
		"checks: not evaluated",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text panel missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteTextEmptySession(t *testing.T) {
	span := &verdictspan.Span{Schema: verdictspan.SchemaVersion, Session: "s"}
	var buf bytes.Buffer
	if err := WriteText(&buf, span); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"(no assistant text in this session)", "(no Edit/Write tool events)", "(no Bash commands ran)"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty panel missing %q\n---\n%s", want, out)
		}
	}
}
