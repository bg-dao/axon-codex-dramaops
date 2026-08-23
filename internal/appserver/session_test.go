package appserver

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAppServerArgsUseTemporaryMCPOverrides(t *testing.T) {
	args := AppServerArgs("/Applications/SceneOps.app/Contents/MacOS/SceneOps", "/tmp/my film")
	joined := strings.Join(args, " ")
	for _, fragment := range []string{"app-server", "--stdio", "mcp_servers.sceneops.command", "mcp_servers.sceneops.args", "mcp_servers.sceneops.required=true"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("args missing %q: %v", fragment, args)
		}
	}
	if strings.Contains(joined, filepath.Join(".codex", "config.toml")) {
		t.Fatalf("app-server args must not mutate global Codex config: %v", args)
	}
}
