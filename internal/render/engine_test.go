package render

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func TestBuildFilterUsesFixedTimelineTransitionsSubtitlesDuckingAndLoudness(t *testing.T) {
	edit := domain.EpisodeEdit{
		Output: domain.DefaultOutputSettings(domain.OrientationPortrait),
		AudioCues: []domain.AudioCue{
			{ID: "dialogue-1", Lane: domain.LaneDialogue, StartSeconds: 0, DurationSeconds: 2, GainDB: -1, DuckBGM: true},
			{ID: "bgm-1", Lane: domain.LaneBGM, StartSeconds: 0, DurationSeconds: 7, GainDB: -8, Loop: true},
		},
		SubtitleCues: []domain.SubtitleCue{{ID: "sub-1", StartSeconds: 0.5, DurationSeconds: 1.5, Text: "别回头"}},
	}
	clips := []domain.VideoClip{
		{ID: "clip-1", Fit: domain.FitCover, Transition: domain.TransitionCut},
		{ID: "clip-2", Fit: domain.FitContain, Transition: domain.TransitionDissolve, TransitionSeconds: 0.5},
	}
	filter, duration, err := buildFilter(edit, clips, []float64{4, 4}, map[string]int{"dialogue-1": 2, "bgm-1": 3}, "/tmp/episode.srt")
	if err != nil {
		t.Fatal(err)
	}
	if duration != 7.5 {
		t.Fatalf("duration = %v", duration)
	}
	for _, fragment := range []string{
		"scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920",
		"scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920",
		"xfade=transition=fade:duration=0.500:offset=3.500",
		"subtitles='/tmp/episode.srt'", "sidechaincompress", "loudnorm=I=-16.000:TP=-1.000",
	} {
		if !strings.Contains(filter, fragment) {
			t.Errorf("filter omitted %q:\n%s", fragment, filter)
		}
	}
}

func TestBuildFilterRejectsOversizedTransitionAndCreatesSilentAudio(t *testing.T) {
	edit := domain.EpisodeEdit{Output: domain.DefaultOutputSettings(domain.OrientationLandscape)}
	clips := []domain.VideoClip{{ID: "clip-1", Fit: domain.FitCover}, {ID: "clip-2", Fit: domain.FitCover, Transition: domain.TransitionFade, TransitionSeconds: 5}}
	if _, _, err := buildFilter(edit, clips, []float64{4, 4}, nil, ""); err == nil {
		t.Fatal("transition longer than adjacent clips must fail")
	}
	filter, _, err := buildFilter(edit, clips[:1], []float64{4}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "anullsrc=r=48000:cl=stereo") || !strings.Contains(filter, "scale=1920:1080") {
		t.Fatalf("silent landscape filter = %s", filter)
	}
}

func TestWriteSRTOrdersCuesAndUsesMillisecondTimecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "episode.srt")
	if err := writeSRT(path, []domain.SubtitleCue{
		{ID: "later", StartSeconds: 2.25, DurationSeconds: 1.5, Text: "Second"},
		{ID: "first", StartSeconds: 0.125, DurationSeconds: 1, Text: "First"},
	}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	want := "1\n00:00:00,125 --> 00:00:01,125\nFirst\n\n2\n00:00:02,250 --> 00:00:03,750\nSecond\n\n"
	if string(content) != want {
		t.Fatalf("SRT = %q", content)
	}
}

func TestFFmpegIntegrationProbesSyntheticMediaAndExtractsFrame(t *testing.T) {
	runtime := DetectRuntime(context.Background(), "")
	if !runtime.Compatible {
		t.Skip(runtime.Error)
	}
	directory := t.TempDir()
	video := filepath.Join(directory, "synthetic.mp4")
	output, err := exec.Command(runtime.FFmpegPath,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=0x10b981:s=320x568:r=25:d=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1",
		"-c:v", "h264_videotoolbox", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", video,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("generate synthetic media: %v: %s", err, output)
	}
	info, err := Probe(context.Background(), runtime, video)
	if err != nil || ValidateMedia(info, domain.AssetKindVideo) != nil || info.Width != 320 || info.Height != 568 || info.AudioCodec == "" {
		t.Fatalf("probe = %+v, err = %v", info, err)
	}
	frame := filepath.Join(directory, "tail.png")
	if err := ExtractFrame(context.Background(), runtime, video, 0.9, frame); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(frame); err != nil || stat.Size() == 0 {
		t.Fatalf("extracted frame = %+v, err = %v", stat, err)
	}
}
