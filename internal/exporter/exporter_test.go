package exporter

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/project"
)

func TestProjectExportIncludesTruthAndExcludesRuntimeState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if _, err := project.NewStore().Create(root, "Export Test"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dramaops", "private-token"), []byte("sk-should-not-export"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Project(root)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	var contents strings.Builder
	for _, file := range reader.File {
		names = append(names, file.Name)
		entry, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		_, _ = io.Copy(&contents, entry)
		_ = entry.Close()
	}
	joined := strings.Join(names, "\n")
	if !strings.Contains(joined, "dramaops.project.json") || !strings.Contains(joined, "episodes/episode-001/episode.json") || !strings.Contains(joined, "exports/episode-001.fountain") || !strings.Contains(joined, "exports/episode-001.srt") {
		t.Fatalf("export omitted project truth: %v", names)
	}
	if strings.Contains(joined, ".dramaops") || strings.Contains(joined, ".git") {
		t.Fatalf("export leaked runtime/private state: %v", names)
	}
	if strings.Contains(contents.String(), "sk-should-not-export") {
		t.Fatal("export leaked a secret value")
	}
	if !strings.HasSuffix(result.Path, ".dramaops.zip") || len(result.SHA256) != 64 || result.Files != len(names) {
		t.Fatalf("invalid export result: %+v", result)
	}
}
