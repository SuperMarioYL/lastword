# Changelog

All notable changes to LastWord are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
