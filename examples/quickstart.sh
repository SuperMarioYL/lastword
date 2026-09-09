#!/bin/sh
# LastWord in three lines — from a finished session to a verdict span.
set -e
go build -o /tmp/lastword ./cmd/lastword
/tmp/lastword verdict testdata/sessions/session-500turn.jsonl --format text
/tmp/lastword latest   # newest main session under ~/.claude/projects
