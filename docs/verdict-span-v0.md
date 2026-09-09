# verdict-span/v0

A **verdict span** is a typed, turn-linked answer record laid over an
existing coding-agent transcript. It answers the question every long session
ends with — *what did the agent actually conclude, and where is the proof?* —
in a form a machine can check: the final answer, the files changed, the last
command outcomes, and a pointer from every claim to the exact transcript turn
that produced it.

This document is the open format reference. LastWord v0.1 emits spans for
finished Claude Code session transcripts; any tool may emit or consume them.
The format is agent-generic by design — the transcript side only requires
turns, tool events and results.

## Document

```json
{
  "schema": "verdict-span/v0",
  "session": "<session uuid>",
  "answer": {
    "lead": "leading paragraph of the final assistant turn",
    "changes": [{"file": "src/auth.go", "added": 12, "removed": 4}],
    "results": [{"kind": "command", "cmd": "go test ./...", "ok": true, "exit_code": null}]
  },
  "pointers": [
    {"turn": 412, "uuid": "<message uuid>", "kind": "final | edit | result",
     "snippet": "verbatim 1-2 lines from that turn",
     "provenance": "which claim this turn backs"}
  ],
  "checks": [
    {"name": "pointer_resolution", "pass": true},
    {"name": "answer_diff", "pass": true, "detail": "3 files claimed, 3 edited, 0 unclaimed"}
  ]
}
```

### `session`

The session identifier as recorded in the transcript. When the transcript
carries no identifier, the file name (without extension) is used.

## Answer

### `answer.lead`

The leading paragraph (text up to the first blank line) of the **last
assistant turn that carries text**. A tool-only final turn has no paragraph
to quote, so the most recent narrating turn is used; a session with no
assistant text has an empty `lead`.

### `answer.changes`

One entry per file touched by `Edit` or `Write` tool events, in
first-appearance order:

- `Edit`: `added` = line count of the new string, `removed` = line count of
  the old string, summed across every edit to that file.
- `Write`: `added` = line count of the written content, `removed` = 0. A
  write replaces the file, but the previous content is not in the
  transcript, so nothing is claimed as removed.

Line counting is git-style: an empty payload is 0 lines; a payload of *n*
newline-separated segments after dropping one trailing newline is *n* lines.
Other tool events (MultiEdit, NotebookEdit, ...) are not counted in v0.

### `answer.results`

One entry per distinct Bash command, in first-appearance order, holding the
**last** observed outcome (a command re-run later replaces its earlier
outcome). Commands are compared after trimming surrounding whitespace.

Claude Code `tool_result` blocks carry no structured exit code, so entries
are derived, not read:

- `cmd` comes from the paired `tool_use` (matched via `tool_use_id`);
- `ok` mirrors `is_error` (`true` when the result is not an error);
- `exit_code` is parsed from `Exit code N` free text only when a failure
  prints one — not all do. It is `null` when no code was printed.

`kind` is `command` in v0; other result kinds may be added later.

## Pointers

Every claim in the answer carries a pointer to the turn that produced it:

| Field | Meaning |
| --- | --- |
| `turn` | 1-based index of the turn among **counted turns** (see below) |
| `uuid` | the transcript message uuid of that turn |
| `kind` | `final`, `edit`, or `result` |
| `snippet` | verbatim quote of up to two non-empty lines from that turn, hard truncated at 120 characters |
| `provenance` | the claim it backs, as the answer's JSON path: `answer.lead`, `answer.changes[<file>]`, `answer.results[<cmd>]` |

Rules:

- `final` — the last assistant turn with text (the lead's source).
- `edit` — the turn of the **last** Edit/Write event for each claimed file.
  The snippet quotes the turn's narration when it has one, otherwise the
  first lines of the payload it wrote.
- `result` — the turn carrying the `tool_result` of each command's last run;
  the snippet quotes the first lines of that result.

Pointer order is deterministic: `final` first, then edits and results in the
same order as their answer entries.

## Checks

A span is only valid if every claim holds against the transcript:

- `pointer_resolution` — every pointer's `turn` and `uuid` resolve to a real
  counted turn, and its `snippet` appears verbatim in that turn.
- `answer_diff` — the claimed `changes` equal the transcript's actual
  Edit/Write events: every claimed file was edited, every edited file is
  claimed, and the added/removed counts match.

The semantics live in the format so any tool can enforce them. LastWord
evaluates them with `lastword verify` (milestone m2); an emitter that has
not evaluated its span sets `checks` to `null` rather than asserting passes.

## Turn semantics

A **turn** is one counted entry of the main conversation:

- `user` and `assistant` entries are counted; tool results are recorded as
  `user` entries and count as the turns that carry them.
- Sidechain (subagent) entries are **not** counted — they belong to nested
  transcripts, not the main session.
- Bookkeeping types (`summary`, `system`, `progress`, ...) are not counted.
- Unknown block types inside a message (thinking, images, future blocks) are
  ignored; text blocks are joined with a blank line.

## Determinism

Extraction is rule-based over the transcript alone: no model calls, no
network, no configuration. The same session file always produces the same
span on the same LastWord version. Interpretation — summarizing, judging,
rephrasing — is deliberately out of scope; a span claims only what the
transcript objectively contains.

## Versioning

`v0` is the first published shape. Breaking changes bump the schema string
(`verdict-span/v1`); additive changes within v0 keep the string. Emitters
must set `schema` to the version they emit; consumers must reject unknown
versions.
