package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultProjectsDir returns the Claude Code projects root (~/.claude/projects).
func DefaultProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Latest returns the path of the newest main-session transcript under
// projectsDir. Main sessions are the *.jsonl files directly inside a project
// directory (projectsDir/<project>/<session-uuid>.jsonl); anything nested
// deeper — such as <session-id>/subagents/** transcripts — is excluded.
// Ties on modification time break by file name so the choice is deterministic.
func Latest(projectsDir string) (string, error) {
	projects, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("read projects dir: %w", err)
	}
	var best string
	var bestMod time.Time
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, project.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			path := filepath.Join(dir, f.Name())
			if best == "" || info.ModTime().After(bestMod) ||
				(info.ModTime().Equal(bestMod) && path < best) {
				best, bestMod = path, info.ModTime()
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no session transcripts found under %s (expected <project>/<session-uuid>.jsonl)", projectsDir)
	}
	return best, nil
}
