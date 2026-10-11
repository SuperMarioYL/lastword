package render

import (
	"embed"
	"html/template"
	"io"
	"strconv"

	"github.com/SuperMarioYL/lastword/internal/transcript"
	"github.com/SuperMarioYL/lastword/internal/verdictspan"
)

//go:embed viewer.html
var viewerFiles embed.FS

var viewerTemplate = template.Must(template.ParseFS(viewerFiles, "viewer.html"))

// WriteViewer renders the single-file HTML viewer: the verdict panel pinned
// at the top while the full counted-turn transcript scrolls beneath it. Each
// pointer is a link to an anchor on its turn; CSS :target highlights the
// turn a link lands on. The template is embedded and the output references
// no external resources — one file, no CDN, no JS build step.
func WriteViewer(w io.Writer, span *verdictspan.Span, turns []transcript.Turn) error {
	data := buildViewerData(span, turns)
	return viewerTemplate.Execute(w, data)
}

// viewerTurn is one transcript turn as the viewer renders it.
type viewerTurn struct {
	Index int
	Kind  string
	Body  string
}

// buildTurnBody flattens a turn into the lines the transcript pane shows:
// the narration, one line per tool use (command or file), and each tool
// result's output.
func buildTurnBody(t transcript.Turn) string {
	var lines []string
	if t.Text != "" {
		lines = append(lines, t.Text)
	}
	for _, u := range t.ToolUses {
		switch {
		case u.Input.Command != "":
			lines = append(lines, "$ "+u.Input.Command)
		case u.Input.FilePath != "":
			lines = append(lines, "tool "+u.Name+" → "+u.Input.FilePath)
		default:
			lines = append(lines, "tool "+u.Name)
		}
	}
	for _, r := range t.ToolResults {
		if r.Content == "" {
			lines = append(lines, "result (empty)")
			continue
		}
		if r.IsError {
			lines = append(lines, "error: "+r.Content)
		} else {
			lines = append(lines, "→ "+r.Content)
		}
	}
	if len(lines) == 0 {
		return "(empty)"
	}
	return joinLines(lines)
}

func joinLines(lines []string) string {
	out := lines[0]
	for _, l := range lines[1:] {
		out += "\n" + l
	}
	return out
}

// buildViewerData assembles everything the template renders. Pointer
// provenance is resolved back to its turn so each claim row links to the
// turn that produced it.
func buildViewerData(span *verdictspan.Span, turns []transcript.Turn) map[string]any {
	turnOf := map[string]int{} // provenance -> pointer turn
	leadTurn := 0
	for _, p := range span.Pointers {
		turnOf[p.Provenance] = p.Turn
		if p.Kind == verdictspan.PointerFinal {
			leadTurn = p.Turn
		}
	}

	changes := make([]map[string]any, 0, len(span.Answer.Changes))
	for _, c := range span.Answer.Changes {
		changes = append(changes, map[string]any{
			"File": c.File, "Added": c.Added, "Removed": c.Removed,
			"Turn": turnOf["answer.changes["+c.File+"]"],
		})
	}

	results := make([]map[string]any, 0, len(span.Answer.Results))
	for _, r := range span.Answer.Results {
		status, class := "ok", "ok"
		if !r.OK {
			status, class = "FAIL", "fail"
			if r.ExitCode != nil {
				status = "FAIL (exit " + strconv.Itoa(*r.ExitCode) + ")"
			}
		}
		results = append(results, map[string]any{
			"Status": status, "Class": class, "Cmd": r.Cmd,
			"Turn": turnOf["answer.results["+r.Cmd+"]"],
		})
	}

	pointers := make([]map[string]any, 0, len(span.Pointers))
	for _, p := range span.Pointers {
		pointers = append(pointers, map[string]any{
			"Turn": p.Turn, "Kind": p.Kind, "Provenance": p.Provenance,
		})
	}

	viewTurns := make([]viewerTurn, 0, len(turns))
	for _, t := range turns {
		viewTurns = append(viewTurns, viewerTurn{Index: t.Index, Kind: t.Kind, Body: buildTurnBody(t)})
	}

	return map[string]any{
		"Schema":   span.Schema,
		"Session":  span.Session,
		"Lead":     span.Answer.Lead,
		"LeadTurn": leadTurn,
		"Changes":  changes,
		"Results":  results,
		"Pointers": pointers,
		"Turns":    viewTurns,
	}
}
