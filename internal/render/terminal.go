// Package render turns a verdict span into its user-facing outputs.
// terminal.go renders the span to stdout: canonical JSON (the format
// contract, default) and a compact text panel for eyeballing a session.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/SuperMarioYL/lastword/internal/verdictspan"
)

// WriteJSON emits the canonical verdict-span/v0 document. Field order and
// structure are the format contract; consumers parse this output.
func WriteJSON(w io.Writer, span *verdictspan.Span) error {
	data, err := json.MarshalIndent(span, "", "  ")
	if err != nil {
		return fmt.Errorf("encode verdict span: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// WriteText emits a human-readable panel over the same span: the answer
// lead, per-file change counts, the last outcome of each command, and the
// pointer table linking every claim back to its turn.
func WriteText(w io.Writer, span *verdictspan.Span) error {
	var b strings.Builder
	b.WriteString("LastWord — " + span.Schema + "\n")
	b.WriteString("session " + span.Session + "\n\n")

	b.WriteString("ANSWER\n")
	lead := span.Answer.Lead
	if lead == "" {
		lead = "(no assistant text in this session)"
	}
	b.WriteString(indent(wrap(lead, 74), "  ") + "\n")

	fmt.Fprintf(&b, "\nCHANGES (%d)\n", len(span.Answer.Changes))
	if len(span.Answer.Changes) == 0 {
		b.WriteString("  (no Edit/Write tool events)\n")
	} else {
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		for _, c := range span.Answer.Changes {
			fmt.Fprintf(tw, "  %s\t+%d\t-%d\n", c.File, c.Added, c.Removed)
		}
		tw.Flush()
	}

	fmt.Fprintf(&b, "\nRESULTS (%d)\n", len(span.Answer.Results))
	if len(span.Answer.Results) == 0 {
		b.WriteString("  (no Bash commands ran)\n")
	} else {
		for _, r := range span.Answer.Results {
			status := "ok"
			if !r.OK {
				status = "FAIL"
				if r.ExitCode != nil {
					status = fmt.Sprintf("FAIL (exit %d)", *r.ExitCode)
				}
			}
			fmt.Fprintf(&b, "  [%s] %s\n", status, r.Cmd)
		}
	}

	byKind := map[string]int{}
	for _, p := range span.Pointers {
		byKind[p.Kind]++
	}
	fmt.Fprintf(&b, "\nPOINTERS (%d)\n", len(span.Pointers))
	if len(span.Pointers) == 0 {
		b.WriteString("  (no claims to anchor)\n")
	} else {
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		for _, p := range span.Pointers {
			fmt.Fprintf(tw, "  #%d\t%s\t%s\n", p.Turn, p.Kind, p.Provenance)
		}
		tw.Flush()
	}
	b.WriteString("\nchecks: not evaluated (verify arrives with milestone m2)\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// wrap breaks text into lines of at most width characters on word
// boundaries, preserving existing newlines.
func wrap(text string, width int) string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		var line string
		for _, word := range strings.Fields(para) {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func indent(text, pad string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}
