package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot is the module root relative to this package.
const repoRoot = "../.."

// TestVersionLockstep guards the release contract: every surface that names
// the product version must EQUAL VERSION. Asserting only that the CHANGELOG
// contains some version heading would not catch the drift this exists for —
// one surface bumped while another lags.
func TestVersionLockstep(t *testing.T) {
	version := strings.TrimSpace(readRepoFile(t, "VERSION"))
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		t.Fatalf("VERSION = %q, want a bare MAJOR.MINOR.PATCH", version)
	}

	t.Run("binary default", func(t *testing.T) {
		if got := mainVersionConstant(t); got != version {
			t.Errorf("cmd/lastword/main.go version = %q, want %q", got, version)
		}
	})
	t.Run("changelog head", func(t *testing.T) {
		if got := changelogHead(t); got != version {
			t.Errorf("newest CHANGELOG heading = %q, want %q", got, version)
		}
	})
	for _, readme := range []string{"README.md", "README.zh-CN.md"} {
		t.Run(readme+" badge", func(t *testing.T) {
			if !strings.Contains(readDemoLine(t, readme), "`"+version+"`") {
				t.Errorf("%s demo caption does not name %s", readme, version)
			}
			if !strings.Contains(readRepoFile(t, readme), "badge/version-"+version+"-") {
				t.Errorf("%s version badge is not %s", readme, version)
			}
		})
	}
	t.Run("demo record", func(t *testing.T) {
		var record struct {
			Version string `json:"version"`
		}
		parseJSON(t, "docs/demo-results.json", &record)
		if record.Version != version {
			t.Errorf("docs/demo-results.json version = %q, want %q", record.Version, version)
		}
	})
	t.Run("site meta", func(t *testing.T) {
		var site struct {
			Meta struct {
				ContentVersion        string `json:"content_version"`
				ImplementationVersion string `json:"implementation_version"`
			} `json:"meta"`
		}
		parseJSON(t, "web/site.json", &site)
		if site.Meta.ImplementationVersion != version {
			t.Errorf("web/site.json meta.implementation_version = %q, want %q",
				site.Meta.ImplementationVersion, version)
		}
		if site.Meta.ContentVersion != "v"+version {
			t.Errorf("web/site.json meta.content_version = %q, want %q",
				site.Meta.ContentVersion, "v"+version)
		}
	})
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func parseJSON(t *testing.T, rel string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(readRepoFile(t, rel)), v); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

// mainVersionConstant reads the compiled-in default from package main; the
// test cannot import main, so it reads the declaration.
func mainVersionConstant(t *testing.T) string {
	t.Helper()
	m := regexp.MustCompile(`var version = "([^"]+)"`).
		FindStringSubmatch(readRepoFile(t, "cmd/lastword/main.go"))
	if m == nil {
		t.Fatal("cmd/lastword/main.go: `var version` declaration not found")
	}
	return m[1]
}

// changelogHead returns the newest released version heading.
func changelogHead(t *testing.T) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^## \[([^\]]+)\]`).
		FindStringSubmatch(readRepoFile(t, "CHANGELOG.md"))
	if m == nil {
		t.Fatal("CHANGELOG.md: no `## [version]` heading found")
	}
	return m[1]
}

// readDemoLine returns the line that claims the demo was recorded from a real
// run of a specific version (English or Chinese README).
func readDemoLine(t *testing.T, readme string) string {
	t.Helper()
	for _, line := range strings.Split(readRepoFile(t, readme), "\n") {
		if strings.Contains(line, "copied verbatim from a real") ||
			strings.Contains(line, "逐字复制自") {
			return line
		}
	}
	t.Fatalf("%s: no recorded-demo caption line found", readme)
	return ""
}
