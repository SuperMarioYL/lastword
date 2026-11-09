# Changelog

All notable changes to LastWord are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] — 2026-11-10

### Fixed

- Pointer snippets are contiguous verbatim quotes again. The quote builder
  kept the first two non-empty lines and rejoined them with a single newline
  after dropping the blank line between them, so a turn that narrated in two
  paragraphs (or in two text blocks, which the parser joins with a blank
  line) produced a snippet no turn contains — and `lastword verify` then
  rejected a span that `lastword verdict` had just extracted, including on
  the bundled `testdata/sessions/session-edges.jsonl` fixture (4/5 pointers
  resolved). The quote now ends at the first blank line, so it is always a
  contiguous run of lines from its source. ASCII single-paragraph quotes are
  byte-identical; the 500-turn span is unchanged.
- A parsed exit code is claimed only for failed commands. A passing command
  whose output merely mentioned a historical failure used to report
  `"ok": true` together with a non-null `exit_code`; the format already says
  the code is parsed "only when a failure prints one", and the terminal panel
  always treated it that way.

### Changed

- The site content and the format reference now describe what v0.2 shipped:
  `lastword verify` fills the checks and exits 0/1, and `lastword view`
  writes the pinned single-file HTML viewer. The previous text still called
  both "milestones m2 and m3", said the checks were "defined but not yet
  evaluated", and showed a recorded panel line ("verify arrives with
  milestone m2") the binary no longer prints. The format reference no longer
  pins a product version and no longer names internal milestones.
- Added a version-lockstep regression test: every release surface (VERSION,
  the binary's compiled default, the newest CHANGELOG heading, both README
  badges and demo captions, the recorded demo version, both site version
  keys) must equal `VERSION`.

## [0.2.0] — 2026-10-12

### Added

- `lastword verify [file]` runs the verdict-span/v0 machine checks —
  pointer_resolution (every pointer's turn/uuid resolves and its snippet
  appears verbatim in that turn, with the documented truncation marker
  matched at a line boundary) and answer_diff (claimed changes equal the
  transcript's actual Edit/Write aggregation) — prints the per-check report
  and exits 0/1, usable from scripts and CI. `--format json` emits the span
  document with `checks` filled; the terminal panel's checks line reflects
  evaluated checks instead of milestone jargon.
- `lastword view [file]` writes a self-contained HTML viewer (`--out`,
  default `lastword.html`): the verdict panel pinned at the top while the
  full counted-turn transcript scrolls beneath it, every pointer a link
  that jumps to and highlights its turn via CSS `:target`. Inline CSS only
  — no CDN, no external fetches, no JavaScript build step — plus `--open`
  for a best-effort browser launch.
- The verify and view smoke checks run in CI over the bundled sessions.

### Fixed

- Pointer snippets no longer corrupt non-ASCII content. The 120-unit
  truncation cap counted bytes, so a cut inside a multi-byte rune emitted
  invalid UTF-8 that `encoding/json` coerced to U+FFFD; the cap now counts
  runes and the quote stays verbatim.
- A torn final JSONL line no longer aborts the whole session. Discovery
  prefers the newest file — usually the session still being written — and
  a trailing entry cut mid-write is skipped; a malformed line anywhere
  before the end still fails the load with its line number.

## [0.1.0] — 2026-09-09

### Added

- m1: `lastword verdict [file]` parses a finished Claude Code session JSONL
  into turns and prints its verdict span (`verdict-span/v0`): the leading
  paragraph of the final assistant answer, per-file Edit/Write change counts,
  and the last outcome of each distinct Bash command, each claim linked by a
  pointer to the turn that produced it. With no file argument the newest
  main session under `~/.claude/projects` is discovered automatically.
- `lastword latest` prints the newest main-session transcript path (main
  sessions are `*.jsonl` files directly inside a project directory; nested
  subagent transcripts are excluded).
- `--format text` renders a readable panel over the same span for eyeballing
  a session; JSON remains the default and the format contract.
- Open format reference `docs/verdict-span-v0.md`, including the
  pointer-resolution and answer-diff check semantics `lastword verify` will
  enforce.
- Sanitized synthetic session fixtures under `testdata/sessions/`, including
  a 500-turn session, exercised by the Go test suite.

### Not yet

- `lastword verify` (pointer-resolution + answer-diff machine checks with a
  pass/fail exit code) is milestone m2.
- `lastword view` (pinned single-file HTML viewer with click-to-jump
  provenance) is milestone m3.
