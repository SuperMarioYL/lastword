// Package cli wires the LastWord command surface: `verdict` extracts a span
// from a finished session, `verify` machine-checks it, `view` renders the
// pinned HTML viewer, and `latest` finds the newest main session.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/lastword/internal/render"
	"github.com/SuperMarioYL/lastword/internal/transcript"
	"github.com/SuperMarioYL/lastword/internal/verdictspan"
)

// Execute builds the root command and runs it. version comes from the
// build (goreleaser injects the release tag).
func Execute(version string) error {
	root := NewRootCommand(version)
	return root.Execute()
}

// NewRootCommand assembles the full command tree.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "lastword",
		Short: "Pin the verdict of a finished Claude Code session",
		Long: "LastWord reads a finished Claude Code session transcript and extracts the\n" +
			"verdict span: the agent's final answer, every file it changed with +/- line\n" +
			"counts, and the last outcome of each command — each claim linked to the\n" +
			"turn that produced it.\n\n" +
			"Extraction is deterministic: no model calls, no network, finished JSONL only.",
		Version: version,
	}
	root.AddCommand(newVerdictCommand(), newVerifyCommand(), newViewCommand(), newLatestCommand())
	return root
}

// resolveProjectsDir applies the --projects-dir default.
func resolveProjectsDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return transcript.DefaultProjectsDir()
}

// resolveSessionPath returns the transcript path to read: the explicit file
// argument, or the newest main session discovered under --projects-dir. The
// discovery note goes to stderr so stdout stays machine-readable.
func resolveSessionPath(cmd *cobra.Command, args []string, projectsDir string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	dir, err := resolveProjectsDir(projectsDir)
	if err != nil {
		return "", err
	}
	path, err := transcript.Latest(dir)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "discovered session: %s\n", path)
	return path, nil
}

// loadSpan loads the session at path and extracts its verdict span — the
// shared front half of verdict, verify and view.
func loadSpan(path string) (*transcript.Session, *verdictspan.Span, error) {
	session, err := transcript.Load(path)
	if err != nil {
		return nil, nil, err
	}
	return session, verdictspan.Extract(session), nil
}

func newVerdictCommand() *cobra.Command {
	var format string
	var projectsDir string
	cmd := &cobra.Command{
		Use:   "verdict [file]",
		Short: "Extract and print the verdict span of a session",
		Long: "Extract the verdict span (verdict-span/v0) from a Claude Code session JSONL\n" +
			"file and print it to stdout.\n\n" +
			"With no file argument, the newest main session under --projects-dir is\n" +
			"used (same discovery as `lastword latest`).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveSessionPath(cmd, args, projectsDir)
			if err != nil {
				return err
			}
			_, span, err := loadSpan(path)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				return render.WriteJSON(cmd.OutOrStdout(), span)
			case "text":
				return render.WriteText(cmd.OutOrStdout(), span)
			default:
				return fmt.Errorf("unknown format %q (want json or text)", format)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "output format: json (the verdict-span/v0 document) or text (a readable panel)")
	cmd.Flags().StringVar(&projectsDir, "projects-dir", "",
		"Claude Code projects root used when no file is given (default $HOME/.claude/projects)")
	return cmd
}

func newVerifyCommand() *cobra.Command {
	var format string
	var projectsDir string
	cmd := &cobra.Command{
		Use:   "verify [file]",
		Short: "Machine-check a session's verdict span and exit 0/1",
		Long: "Extract the verdict span from a Claude Code session JSONL file and run the\n" +
			"verdict-span/v0 checks against it: every pointer must resolve to a real\n" +
			"turn with a verbatim snippet (pointer_resolution), and the claimed changes\n" +
			"must equal the transcript's actual Edit/Write events (answer_diff).\n\n" +
			"With no file argument, the newest main session under --projects-dir is\n" +
			"used. Exits 0 when all checks pass and 1 on any violation, so the command\n" +
			"is usable from scripts and CI.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true, // a failed check is a verdict, not a usage error
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveSessionPath(cmd, args, projectsDir)
			if err != nil {
				return err
			}
			session, span, err := loadSpan(path)
			if err != nil {
				return err
			}
			span.Checks = verdictspan.Verify(session, span)
			switch format {
			case "json":
				if err := render.WriteJSON(cmd.OutOrStdout(), span); err != nil {
					return err
				}
			case "text":
				if err := render.WriteVerify(cmd.OutOrStdout(), span); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown format %q (want json or text)", format)
			}
			for _, c := range span.Checks {
				if !c.Pass {
					return fmt.Errorf("verdict span failed verification")
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text (the check report) or json (the span document with checks filled)")
	cmd.Flags().StringVar(&projectsDir, "projects-dir", "",
		"Claude Code projects root used when no file is given (default $HOME/.claude/projects)")
	return cmd
}

func newViewCommand() *cobra.Command {
	var out, projectsDir string
	var open bool
	cmd := &cobra.Command{
		Use:   "view [file]",
		Short: "Render the pinned verdict and full transcript as one HTML file",
		Long: "Write a self-contained HTML viewer for a Claude Code session: the verdict\n" +
			"panel — answer, changes, command outcomes — pinned at the top while the\n" +
			"full transcript scrolls beneath it, every pointer a link that jumps to and\n" +
			"highlights the turn that produced its claim.\n\n" +
			"The file is standalone: inline CSS, no CDN, no JavaScript build step.\n" +
			"With no file argument, the newest main session under --projects-dir is\n" +
			"used (same discovery as `lastword latest`).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveSessionPath(cmd, args, projectsDir)
			if err != nil {
				return err
			}
			session, span, err := loadSpan(path)
			if err != nil {
				return err
			}
			if out == "" {
				out = "lastword.html"
			}
			f, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("create viewer: %w", err)
			}
			writeErr := render.WriteViewer(f, span, session.Turns)
			closeErr := f.Close()
			if writeErr != nil {
				return fmt.Errorf("write viewer: %w", writeErr)
			}
			if closeErr != nil {
				return fmt.Errorf("write viewer: %w", closeErr)
			}
			abs, err := filepath.Abs(out)
			if err != nil {
				abs = out
			}
			fmt.Fprintln(cmd.OutOrStdout(), abs)
			if open {
				if err := openBrowser(abs); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "could not open a browser: %v (open %s yourself)\n", err, abs)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output HTML path (default lastword.html)")
	cmd.Flags().BoolVar(&open, "open", false, "open the viewer in a browser after writing it (best-effort)")
	cmd.Flags().StringVar(&projectsDir, "projects-dir", "",
		"Claude Code projects root used when no file is given (default $HOME/.claude/projects)")
	return cmd
}

// openBrowser opens path with the platform opener. Best-effort by contract:
// the caller reports failure without failing the command.
func openBrowser(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Run()
	default:
		return exec.Command("xdg-open", path).Run()
	}
}

func newLatestCommand() *cobra.Command {
	var projectsDir string
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Print the newest main Claude Code session transcript",
		Long: "Print the path of the newest main-session JSONL under the Claude Code\n" +
			"projects root. Main sessions are the *.jsonl files directly inside a\n" +
			"project directory; nested subagent transcripts are excluded.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveProjectsDir(projectsDir)
			if err != nil {
				return err
			}
			path, err := transcript.Latest(dir)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectsDir, "projects-dir", "", "Claude Code projects root (default $HOME/.claude/projects)")
	return cmd
}
