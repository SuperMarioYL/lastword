// Package verdictspan implements the verdict-span/v0 format: a typed,
// turn-linked answer record laid over an existing Claude Code transcript.
//
// A span is emitted by deterministic extraction (extract.go) and is only
// considered valid once every pointer resolves to a real turn and the
// answer's diff claims match the transcript's actual edit events — the
// check semantics verify.go will enforce (milestone m2).
package verdictspan

// SchemaVersion identifies the emitted format.
const SchemaVersion = "verdict-span/v0"

// Pointer kinds.
const (
	PointerFinal  = "final"  // the assistant turn the answer lead comes from
	PointerEdit   = "edit"   // the tool event behind a claimed file change
	PointerResult = "result" // the tool_result behind a claimed command outcome
)

// Check names (evaluated by verify, milestone m2).
const (
	CheckPointerResolution = "pointer_resolution"
	CheckAnswerDiff        = "answer_diff"
)

// Span is the top-level verdict-span/v0 document.
type Span struct {
	Schema   string    `json:"schema"`
	Session  string    `json:"session"`
	Answer   Answer    `json:"answer"`
	Pointers []Pointer `json:"pointers"`
	// Checks is nil when the emitter has not evaluated the span; the
	// pointer_resolution and answer_diff checks land with `lastword verify`
	// in milestone m2 and marshal as null until then.
	Checks []Check `json:"checks"`
}

// Answer is what the session actually concluded: the lead line, the files
// it changed, and how its commands ended.
type Answer struct {
	Lead    string   `json:"lead"`
	Changes []Change `json:"changes"`
	Results []Result `json:"results"`
}

// Change is a per-file aggregation of the session's Edit/Write tool events.
type Change struct {
	File    string `json:"file"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// Result is the last observed outcome of one distinct command. Claude Code
// tool_result blocks carry no structured exit code, so OK mirrors is_error
// and ExitCode is set only when the result text prints "Exit code N".
// ExitCode is nil when no code was printed.
type Result struct {
	Kind     string `json:"kind"` // always "command" in v0
	Cmd      string `json:"cmd"`
	OK       bool   `json:"ok"`
	ExitCode *int   `json:"exit_code"`
}

// Pointer links one answer claim to the specific turn that produced it.
// Snippet is a verbatim quote of one to two lines from that turn (hard
// truncated at snippetWidth characters); Provenance names the claim, using
// the JSON path of the answer entry it backs.
type Pointer struct {
	Turn       int    `json:"turn"`
	UUID       string `json:"uuid"`
	Kind       string `json:"kind"`
	Snippet    string `json:"snippet"`
	Provenance string `json:"provenance"`
}

// Check is one machine verdict over a span. Pass is false and Detail
// explains what failed; an unevaluated check does not appear in Checks.
type Check struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}
