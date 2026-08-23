package appserver

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAppServerArgsUseTemporaryMCPOverrides(t *testing.T) {
	args := AppServerArgs("/Applications/DramaOps.app/Contents/MacOS/DramaOps", "/tmp/my film")
	joined := strings.Join(args, " ")
	for _, fragment := range []string{"app-server", "--stdio", "mcp_servers.dramaops.command", "mcp_servers.dramaops.args", "mcp_servers.dramaops.required=true"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("args missing %q: %v", fragment, args)
		}
	}
	if strings.Contains(joined, filepath.Join(".codex", "config.toml")) {
		t.Fatalf("app-server args must not mutate global Codex config: %v", args)
	}
}
