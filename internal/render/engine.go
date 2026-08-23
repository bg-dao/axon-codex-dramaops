package render

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
	"github.com/bg-dao/axon-codex-dramaops/internal/project"
	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
)

type Progress struct {
	Percent int    `json:"percent"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type Request struct {
	Root       string
	Snapshot   domain.Snapshot
	Edit       domain.EpisodeEdit
	OutputPath string
	SRTPath    string
}

type Result struct {
	Path      string           `json:"path"`
	SRTPath   string           `json:"srtPath"`
	MediaInfo domain.MediaInfo `json:"mediaInfo"`
	Version   string           `json:"version"`
	Arguments []string         `json:"arguments"`
	Duration  float64          `json:"durationSeconds"`
}

type Engine struct{ Runtime RuntimeStatus }

func (e *Engine) Render(ctx context.Context, request Request, progress func(Progress)) (Result, error) {
	if err := e.Runtime.Require(); err != nil {
		return Result{}, err
	}
	args, total, err := e.BuildCommand(ctx, request)
	if err != nil {
		return Result{}, err
	}
	if progress != nil {
		progress(Progress{Percent: 1, Phase: "prepare", Message: "Validated timeline and media"})
	}
	if err := os.MkdirAll(filepath.Dir(request.OutputPath), 0o755); err != nil {
		return Result{}, err
	}
	if err := writeSRT(request.SRTPath, request.Edit.SubtitleCues); err != nil {
		return Result{}, err
	}
	command := exec.CommandContext(ctx, e.Runtime.FFmpegPath, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start ffmpeg: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "out_time_ms=") {
			continue
		}
		microseconds, _ := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_ms="), 64)
		percent := int((microseconds / 1_000_000) / total * 100)
		if percent < 1 {
			percent = 1
		}
		if percent > 99 {
			percent = 99
		}
		if progress != nil {
			progress(Progress{Percent: percent, Phase: "render", Message: "Rendering episode"})
		}
	}
	if err := scanner.Err(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return Result{}, err
	}
	if err := command.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return Result{}, context.Canceled
		}
		return Result{}, fmt.Errorf("ffmpeg render failed: %s", redact.String(strings.TrimSpace(stderr.String())))
	}
	info, err := Probe(ctx, e.Runtime, request.OutputPath)
	if err != nil {
		return Result{}, err
	}
	if err := ValidateMedia(info, domain.AssetKindRender); err != nil {
		return Result{}, err
	}
	if progress != nil {
		progress(Progress{Percent: 100, Phase: "complete", Message: "Episode render complete"})
	}
	return Result{Path: request.OutputPath, SRTPath: request.SRTPath, MediaInfo: info, Version: e.Runtime.Version, Arguments: sanitizeArguments(args, request.Root), Duration: total}, nil
}

func (e *Engine) BuildCommand(ctx context.Context, request Request) ([]string, float64, error) {
	if err := e.Runtime.Require(); err != nil {
		return nil, 0, err
	}
	if request.Edit.EpisodeID == "" || len(request.Edit.VideoTrack) == 0 {
		return nil, 0, errors.New("episode timeline has no video clips")
	}
	assets := make(map[string]domain.Asset, len(request.Snapshot.Assets))
	for _, asset := range request.Snapshot.Assets {
		assets[asset.ID] = asset
	}
	clips := append([]domain.VideoClip(nil), request.Edit.VideoTrack...)
	sort.Slice(clips, func(i, j int) bool { return clips[i].Order < clips[j].Order })
	args := []string{"-hide_banner", "-y"}
	videoDurations := make([]float64, len(clips))
	for i, clip := range clips {
		if clip.Order != i {
			return nil, 0, fmt.Errorf("video track order is not contiguous at clip %s", clip.ID)
		}
		asset, ok := assets[clip.AssetID]
		if !ok || asset.Kind != domain.AssetKindVideo {
			return nil, 0, fmt.Errorf("clip %s has no valid video asset", clip.ID)
		}
		path, err := verifiedPath(request.Root, asset)
		if err != nil {
			return nil, 0, err
		}
		info, err := Probe(ctx, e.Runtime, path)
		if err != nil {
			return nil, 0, err
		}
		if err := ValidateMedia(info, domain.AssetKindVideo); err != nil {
			return nil, 0, fmt.Errorf("clip %s: %w", clip.ID, err)
		}
		if clip.OutSeconds > info.DurationSeconds+0.05 {
			return nil, 0, fmt.Errorf("clip %s out point exceeds source duration", clip.ID)
		}
		duration := clip.OutSeconds - clip.InSeconds
		if duration <= 0 {
			return nil, 0, fmt.Errorf("clip %s has invalid trim", clip.ID)
		}
		videoDurations[i] = duration
		args = append(args, "-ss", seconds(clip.InSeconds), "-t", seconds(duration), "-i", path)
	}

	audioInput := make(map[string]int)
	for _, cue := range request.Edit.AudioCues {
		asset, ok := assets[cue.AssetID]
		if !ok || asset.Kind != domain.AssetKindAudio {
			return nil, 0, fmt.Errorf("audio cue %s has no valid audio asset", cue.ID)
		}
		path, err := verifiedPath(request.Root, asset)
		if err != nil {
			return nil, 0, err
		}
		info, err := Probe(ctx, e.Runtime, path)
		if err != nil {
			return nil, 0, err
		}
		if err := ValidateMedia(info, domain.AssetKindAudio); err != nil {
			return nil, 0, fmt.Errorf("audio cue %s: %w", cue.ID, err)
		}
		if cue.Loop {
			args = append(args, "-stream_loop", "-1")
		}
		audioInput[cue.ID] = len(clips) + len(audioInput)
		args = append(args, "-i", path)
	}

	filter, total, err := buildFilter(request.Edit, clips, videoDurations, audioInput, request.SRTPath)
	if err != nil {
		return nil, 0, err
	}
	args = append(args, "-filter_complex", filter, "-map", "[vout]", "-map", "[aout]", "-c:v", "h264_videotoolbox", "-b:v", "8M", "-maxrate", "12M", "-bufsize", "16M", "-pix_fmt", "yuv420p", "-r", strconv.Itoa(request.Edit.Output.FPS), "-c:a", "aac", "-ar", strconv.Itoa(request.Edit.Output.AudioSampleRate), "-ac", strconv.Itoa(request.Edit.Output.AudioChannels), "-t", seconds(total), "-movflags", "+faststart", "-progress", "pipe:1", "-nostats", request.OutputPath)
	return args, total, nil
}

func buildFilter(edit domain.EpisodeEdit, clips []domain.VideoClip, durations []float64, audioInputs map[string]int, srtPath string) (string, float64, error) {
	output := edit.Output
	if output.Width <= 0 || output.Height <= 0 || output.FPS <= 0 {
		return "", 0, errors.New("output settings are invalid")
	}
	filters := make([]string, 0, len(clips)+len(edit.AudioCues)+8)
	for i, clip := range clips {
		fit := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", output.Width, output.Height, output.Width, output.Height)
		if clip.Fit == domain.FitContain {
			fit = fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", output.Width, output.Height, output.Width, output.Height)
		}
		filters = append(filters, fmt.Sprintf("[%d:v]%s,fps=%d,setsar=1,format=yuv420p,setpts=PTS-STARTPTS[v%d]", i, fit, output.FPS, i))
	}
	current, total := "v0", durations[0]
	for i := 1; i < len(clips); i++ {
		label := fmt.Sprintf("vjoin%d", i)
		transition := clips[i].Transition
		if transition == "" || transition == domain.TransitionCut || clips[i].TransitionSeconds <= 0 {
			filters = append(filters, fmt.Sprintf("[%s][v%d]concat=n=2:v=1:a=0[%s]", current, i, label))
			total += durations[i]
		} else {
			duration := clips[i].TransitionSeconds
			if duration > total || duration > durations[i] {
				return "", 0, fmt.Errorf("transition on clip %s exceeds adjacent duration", clips[i].ID)
			}
			name := "fade"
			if transition == domain.TransitionFade {
				name = "fadeblack"
			}
			filters = append(filters, fmt.Sprintf("[%s][v%d]xfade=transition=%s:duration=%s:offset=%s[%s]", current, i, name, seconds(duration), seconds(total-duration), label))
			total += durations[i] - duration
		}
		current = label
	}
	if output.BurnSubtitles && len(edit.SubtitleCues) > 0 {
		shortEdge := min(output.Width, output.Height)
		fontSize := max(32, shortEdge/22)
		margin := max(48, int(float64(output.Height)*output.SubtitleSafeArea))
		filters = append(filters, fmt.Sprintf("[%s]subtitles='%s':force_style='Alignment=2,MarginV=%d,FontSize=%d,Outline=2'[vout]", current, escapeFilterPath(srtPath), margin, fontSize))
	} else {
		filters = append(filters, fmt.Sprintf("[%s]null[vout]", current))
	}

	laneLabels := map[domain.AudioLane][]string{}
	for i, cue := range edit.AudioCues {
		inputIndex := audioInputs[cue.ID]
		label := fmt.Sprintf("acue%d", i)
		delay := int(cue.StartSeconds*1000 + 0.5)
		filters = append(filters, fmt.Sprintf("[%d:a]atrim=0:%s,asetpts=PTS-STARTPTS,volume=%sdB,adelay=%d|%d[%s]", inputIndex, seconds(cue.DurationSeconds), seconds(cue.GainDB), delay, delay, label))
		laneLabels[cue.Lane] = append(laneLabels[cue.Lane], label)
	}
	mixLane := func(lane domain.AudioLane, target string) string {
		labels := laneLabels[lane]
		if len(labels) == 0 {
			return ""
		}
		if len(labels) == 1 {
			filters = append(filters, fmt.Sprintf("[%s]anull[%s]", labels[0], target))
			return target
		}
		inputs := ""
		for _, label := range labels {
			inputs += "[" + label + "]"
		}
		filters = append(filters, fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0[%s]", inputs, len(labels), target))
		return target
	}
	dialogue, sfx, bgm := mixLane(domain.LaneDialogue, "dialogue"), mixLane(domain.LaneSFX, "sfx"), mixLane(domain.LaneBGM, "bgmraw")
	duck := false
	for _, cue := range edit.AudioCues {
		if cue.Lane == domain.LaneDialogue && cue.DuckBGM {
			duck = true
		}
	}
	if duck && dialogue != "" && bgm != "" {
		filters = append(filters, "[bgmraw][dialogue]sidechaincompress=threshold=0.03:ratio=8:attack=20:release=300[bgm]")
		bgm = "bgm"
	}
	finalLabels := []string{}
	for _, label := range []string{dialogue, sfx, bgm} {
		if label != "" {
			finalLabels = append(finalLabels, label)
		}
	}
	if len(finalLabels) == 0 {
		filters = append(filters, fmt.Sprintf("anullsrc=r=%d:cl=stereo,atrim=duration=%s[aout]", output.AudioSampleRate, seconds(total)))
	} else {
		inputs := ""
		for _, label := range finalLabels {
			inputs += "[" + label + "]"
		}
		filters = append(filters, fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0,loudnorm=I=%s:TP=%s:LRA=11,atrim=duration=%s[aout]", inputs, len(finalLabels), seconds(output.LoudnessLUFS), seconds(output.TruePeakDBTP), seconds(total)))
	}
	return strings.Join(filters, ";"), total, nil
}

func verifiedPath(root string, asset domain.Asset) (string, error) {
	path, err := project.ResolveRelative(root, asset.RelativePath)
	if err != nil {
		return "", err
	}
	hash, err := project.HashFile(path)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(hash, asset.SHA256) {
		return "", fmt.Errorf("asset %s failed SHA-256 verification", asset.ID)
	}
	return path, nil
}

func writeSRT(path string, cues []domain.SubtitleCue) error {
	ordered := append([]domain.SubtitleCue(nil), cues...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartSeconds < ordered[j].StartSeconds })
	var output strings.Builder
	for i, cue := range ordered {
		fmt.Fprintf(&output, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(cue.StartSeconds), srtTime(cue.StartSeconds+cue.DurationSeconds), strings.TrimSpace(cue.Text))
	}
	return project.AtomicWrite(path, []byte(output.String()), 0o644)
}

func srtTime(value float64) string {
	if value < 0 {
		value = 0
	}
	ms := int64(value*1000 + 0.5)
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func escapeFilterPath(path string) string {
	return strings.NewReplacer("\\", "\\\\", ":", "\\:", "'", "\\'").Replace(path)
}
func seconds(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }
func sanitizeArguments(args []string, root string) []string {
	result := make([]string, len(args))
	for i, value := range args {
		result[i] = strings.ReplaceAll(value, root, "$PROJECT")
	}
	return result
}
