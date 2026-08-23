package render

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func Probe(ctx context.Context, runtime RuntimeStatus, path string) (domain.MediaInfo, error) {
	if err := runtime.Require(); err != nil {
		return domain.MediaInfo{}, err
	}
	output, err := exec.CommandContext(ctx, runtime.FFprobePath, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return domain.MediaInfo{}, fmt.Errorf("ffprobe asset: %w", err)
	}
	var value struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
			SampleRate   string `json:"sample_rate"`
			Channels     int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &value); err != nil {
		return domain.MediaInfo{}, fmt.Errorf("decode ffprobe output: %w", err)
	}
	info := domain.MediaInfo{}
	info.DurationSeconds, _ = strconv.ParseFloat(value.Format.Duration, 64)
	for _, stream := range value.Streams {
		switch stream.CodecType {
		case "video":
			info.Width, info.Height, info.VideoCodec, info.FPS = stream.Width, stream.Height, stream.CodecName, parseRate(stream.AvgFrameRate)
		case "audio":
			info.AudioCodec, info.SampleRate, info.Channels = stream.CodecName, parseInt(stream.SampleRate), stream.Channels
		}
	}
	return info, nil
}

func ValidateMedia(info domain.MediaInfo, kind domain.AssetKind) error {
	if info.DurationSeconds <= 0 || math.IsNaN(info.DurationSeconds) || math.IsInf(info.DurationSeconds, 0) {
		return fmt.Errorf("media duration is invalid")
	}
	if (kind == domain.AssetKindVideo || kind == domain.AssetKindRender) && (info.Width <= 0 || info.Height <= 0 || info.VideoCodec == "") {
		return fmt.Errorf("media has no usable video stream")
	}
	if kind == domain.AssetKindAudio && info.AudioCodec == "" {
		return fmt.Errorf("media has no usable audio stream")
	}
	return nil
}

func parseRate(value string) float64 {
	parts := strings.Split(value, "/")
	if len(parts) == 2 {
		numerator, _ := strconv.ParseFloat(parts[0], 64)
		denominator, _ := strconv.ParseFloat(parts[1], 64)
		if denominator != 0 {
			return numerator / denominator
		}
	}
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

func parseInt(value string) int { result, _ := strconv.Atoi(value); return result }
