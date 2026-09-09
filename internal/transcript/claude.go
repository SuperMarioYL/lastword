// Package transcript parses finished Claude Code session transcripts
// (JSONL) into typed turns.
//
// LastWord is a post-hoc reader: it never hooks a running session and never
// calls a model. A session file is one JSON object per line; this package
// keeps only the entry types that carry conversation content ("user" and
// "assistant"), skips sidechain (subagent) entries and bookkeeping types
// such as "summary", and exposes the tool_use / tool_result pairing the
// verdict-span extraction needs.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Turn is one counted transcript entry: a user message, an assistant
// message, or the tool_result entries Claude Code records as user messages.
// Index is the 1-based position among counted turns and is the number
// verdict-span pointers reference.
type Turn struct {
	Index       int
	UUID        string
	Kind        string // "user" or "assistant"
	Text        string // concatenated text blocks (thinking excluded)
	ToolUses    []ToolUse
	ToolResults []ToolResult
	Timestamp   time.Time
}

// ToolUse is a tool_use block from an assistant turn.
type ToolUse struct {
	ID    string
	Name  string // "Edit", "Write", "Bash", ...
	Input ToolInput
}

// ToolInput carries the input fields of the tool events LastWord reads:
// file edits (Edit/Write) and commands (Bash).
type ToolInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	Content   string `json:"content"`
	Command   string `json:"command"`
}

// ToolResult is a tool_result block from a user turn. It carries no
// structured exit code: OK mirrors is_error, and an exit code exists only
// when the tool's free text prints one.
type ToolResult struct {
	ToolUseID string
	IsError   bool
	Content   string
}

// Session is a parsed transcript file.
type Session struct {
	// ID is the sessionId recorded in the file; when no entry carries one,
	// the file name without its .jsonl extension is used.
	ID    string
	Turns []Turn
}

// The wire shapes below are deliberately permissive: Claude Code writes
// message content either as a plain string or as a block array, and block
// fields vary by block type. Unknown block types are ignored.

type rawEntry struct {
	Type        string          `json:"type"`
	UUID        string          `json:"uuid"`
	SessionID   string          `json:"sessionId"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	Message     json.RawMessage `json:"message"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type rawBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Load reads a session JSONL file and returns its counted turns. Lines that
// are not JSON objects, or entry types other than user/assistant, are not
// counted; sidechain entries are skipped because they belong to subagent
// transcripts, not the main conversation.
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	s := &Session{ID: sessionIDFromPath(path)}
	haveRecordedID := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	// A single long turn (pasted file, large tool output) can exceed the
	// default 64 KiB scanner limit.
	sc.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("%s:%d: malformed JSONL entry: %w", path, line, err)
		}
		// Prefer the recorded sessionId over the file-name fallback; the
		// first entry that carries one wins.
		if e.SessionID != "" && !haveRecordedID {
			s.ID = e.SessionID
			haveRecordedID = true
		}
		if e.IsSidechain || (e.Type != "user" && e.Type != "assistant") {
			continue
		}
		s.Turns = append(s.Turns, parseTurn(e, len(s.Turns)+1))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	return s, nil
}

func parseTurn(e rawEntry, index int) Turn {
	t := Turn{Index: index, UUID: e.UUID, Kind: e.Type}
	if e.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			t.Timestamp = ts
		}
	}
	if len(e.Message) == 0 {
		return t
	}
	var m rawMessage
	if err := json.Unmarshal(e.Message, &m); err != nil {
		return t
	}
	// Plain-string content is the common shape for user prompts.
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		t.Text = s
		return t
	}
	var blocks []rawBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		// Neither string nor block array (or null): nothing to read.
		return t
	}
	var texts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			texts = append(texts, b.Text)
		case "tool_use":
			use := ToolUse{ID: b.ID, Name: b.Name}
			if len(b.Input) > 0 {
				_ = json.Unmarshal(b.Input, &use.Input)
			}
			t.ToolUses = append(t.ToolUses, use)
		case "tool_result":
			t.ToolResults = append(t.ToolResults, ToolResult{
				ToolUseID: b.ToolUseID,
				IsError:   b.IsError,
				Content:   blockText(b.Content),
			})
		default:
			// thinking, images, and any future block type: skipped.
		}
	}
	t.Text = strings.Join(texts, "\n\n")
	return t
}

// blockText flattens a tool_result content field, which is either a plain
// string or an array of {type:"text", text:...} blocks, into one string.
func blockText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []rawBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var texts []string
		for _, b := range blocks {
			if b.Type == "text" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
