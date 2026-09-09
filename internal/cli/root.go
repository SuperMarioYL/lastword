// Package cli wires the LastWord command surface. v0.1 ships the m1
// milestone: `verdict` extracts a span from a finished session and `latest`
// finds the newest main session.
package cli

import (
	"fmt"

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
	root.AddCommand(newVerdictCommand(), newLatestCommand())
	return root
}

// resolveProjectsDir applies the --projects-dir default.
func resolveProjectsDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	return transcript.DefaultProjectsDir()
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
			path := ""
			if len(args) == 1 {
				path = args[0]
			} else {
				dir, err := resolveProjectsDir(projectsDir)
				if err != nil {
					return err
				}
				path, err = transcript.Latest(dir)
				if err != nil {
					return err
				}
				// Keep stdout pure for pipes: the discovery note goes to stderr.
				fmt.Fprintf(cmd.ErrOrStderr(), "discovered session: %s\n", path)
			}
			session, err := transcript.Load(path)
			if err != nil {
				return err
			}
			span := verdictspan.Extract(session)
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
