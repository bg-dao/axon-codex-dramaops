package render

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
)

// ExtractFrame writes one PNG frame with the verified FFmpeg runtime. Callers
// provide a temporary destination and atomically commit the bytes afterward.
func ExtractFrame(ctx context.Context, runtime RuntimeStatus, source string, atSeconds float64, destination string) error {
	if err := runtime.Require(); err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(destination) == "" {
		return fmt.Errorf("frame source and destination are required")
	}
	if atSeconds < 0 {
		atSeconds = 0
	}
	output, err := exec.CommandContext(ctx, runtime.FFmpegPath,
		"-hide_banner", "-loglevel", "error", "-y", "-ss", seconds(atSeconds), "-i", source,
		"-frames:v", "1", "-c:v", "png", "-f", "image2", destination,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract video frame: %s", redact.String(strings.TrimSpace(string(output))))
	}
	return nil
}
