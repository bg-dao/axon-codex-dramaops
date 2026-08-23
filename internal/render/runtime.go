package render

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type RuntimeStatus struct {
	FFmpegPath  string `json:"ffmpegPath,omitempty"`
	FFprobePath string `json:"ffprobePath,omitempty"`
	Version     string `json:"version,omitempty"`
	Compatible  bool   `json:"compatible"`
	Encoder     string `json:"encoder,omitempty"`
	Source      string `json:"source,omitempty"`
	Error       string `json:"error,omitempty"`
}

// DetectRuntime deliberately fails closed. Release builds may place a verified
// fallback pair in privateBinDir after validating their checked-in release
// manifest; an unverified executable is never downloaded or launched.
func DetectRuntime(ctx context.Context, privateBinDir string) RuntimeStatus {
	candidates := [][3]string{}
	if privateBinDir != "" {
		candidates = append(candidates, [3]string{privateBinDir + "/ffmpeg", privateBinDir + "/ffprobe", "managed"})
	}
	if ffmpeg, err := exec.LookPath("ffmpeg"); err == nil {
		if ffprobe, probeErr := exec.LookPath("ffprobe"); probeErr == nil {
			candidates = append([][3]string{{ffmpeg, ffprobe, "system"}}, candidates...)
		}
	}
	for _, candidate := range candidates {
		status := verifyRuntime(ctx, candidate[0], candidate[1])
		status.Source = candidate[2]
		if status.Compatible {
			return status
		}
	}
	return RuntimeStatus{Error: "compatible FFmpeg and ffprobe were not found; install FFmpeg with h264_videotoolbox support", Source: "missing"}
}

func verifyRuntime(ctx context.Context, ffmpeg, ffprobe string) RuntimeStatus {
	status := RuntimeStatus{FFmpegPath: ffmpeg, FFprobePath: ffprobe, Encoder: "h264_videotoolbox"}
	version, err := exec.CommandContext(ctx, ffmpeg, "-version").Output()
	if err != nil {
		status.Error = fmt.Sprintf("verify ffmpeg: %v", err)
		return status
	}
	first := strings.SplitN(string(version), "\n", 2)[0]
	status.Version = strings.TrimSpace(first)
	if err := exec.CommandContext(ctx, ffprobe, "-version").Run(); err != nil {
		status.Error = fmt.Sprintf("verify ffprobe: %v", err)
		return status
	}
	encoders, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		status.Error = fmt.Sprintf("list ffmpeg encoders: %v", err)
		return status
	}
	if !strings.Contains(string(encoders), "h264_videotoolbox") {
		status.Error = "FFmpeg lacks h264_videotoolbox"
		return status
	}
	status.Compatible = true
	return status
}

func (s RuntimeStatus) Require() error {
	if !s.Compatible {
		if s.Error != "" {
			return errors.New(s.Error)
		}
		return errors.New("FFmpeg runtime is unavailable")
	}
	return nil
}
