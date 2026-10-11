#!/bin/sh
# LastWord in a few lines — from a finished session to a checked, pinned verdict.
set -e
go build -o /tmp/lastword ./cmd/lastword
/tmp/lastword verdict testdata/sessions/session-500turn.jsonl --format text
/tmp/lastword verify  testdata/sessions/session-500turn.jsonl   # pointer + diff checks, exit 0/1
/tmp/lastword view    testdata/sessions/session-500turn.jsonl   # single-file HTML viewer
/tmp/lastword latest  # newest main session under ~/.claude/projects
