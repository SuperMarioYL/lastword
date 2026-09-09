Output docs/demo-500turn.gif
Set FontSize 13
Set Width 1080
Set Height 820
Set Padding 16
Set Theme Dracula
Set TypingSpeed 6ms

# LastWord m1 demo — a finished 500-turn Claude Code session, pinned.
# Reproducible: build from source, stage the bundled sanitized fixture as a
# demo projects tree, then run latest / verdict on it.

Type "go build -o /tmp/lastword ./cmd/lastword"
Enter
Sleep 2500ms

Type "head -3 testdata/sessions/session-500turn.jsonl | cut -c1-108"
Enter
Sleep 700ms

Type "# 503 lines of interleaved narration — the answer is in there somewhere"
Enter
Sleep 500ms

Type "wc -l testdata/sessions/session-500turn.jsonl"
Enter
Sleep 700ms

Type "rm -rf /tmp/lwdemo && mkdir -p /tmp/lwdemo/projects/-Users-dev-work-api-server && cp testdata/sessions/session-500turn.jsonl /tmp/lwdemo/projects/-Users-dev-work-api-server/"
Enter
Sleep 600ms

Type "# 1. find the newest finished main session — no scrolling"
Enter
Sleep 500ms
Type "/tmp/lastword latest --projects-dir /tmp/lwdemo/projects"
Enter
Sleep 900ms

Type "# 2. the same verdict as the open verdict-span/v0 JSON document"
Enter
Sleep 500ms
Type "/tmp/lastword verdict testdata/sessions/session-500turn.jsonl | head -30"
Enter
Sleep 1600ms

Type "# 3. the verdict, pinned: answer, changes, results, pointers to turns"
Enter
Sleep 500ms
Type "/tmp/lastword verdict --projects-dir /tmp/lwdemo/projects --format text"
Enter
Sleep 3000ms

Type "# deterministic — zero model calls; verify lands in m2, the viewer in m3"
Enter
Sleep 1500ms
