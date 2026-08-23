package provider

import (
	"context"
	"io"
	"time"
)

type Capabilities struct {
	ImageGeneration     bool     `json:"imageGeneration"`
	ImageReferences     bool     `json:"imageReferences"`
	MaxImageReferences  int      `json:"maxImageReferences,omitempty"`
	VideoGeneration     bool     `json:"videoGeneration"`
	VideoExperimental   bool     `json:"videoExperimental"`
	VideoReferenceRoles []string `json:"videoReferenceRoles,omitempty"`
	MaxVideoReferences  int      `json:"maxVideoReferences,omitempty"`
	SpeechGeneration    bool     `json:"speechGeneration"`
	CustomVoices        bool     `json:"customVoices"`
	SoundGeneration     bool     `json:"soundGeneration"`
	ImageModels         []string `json:"imageModels,omitempty"`
	VideoModels         []string `json:"videoModels,omitempty"`
	SpeechModels        []string `json:"speechModels,omitempty"`
	BuiltInVoices       []string `json:"builtInVoices,omitempty"`
	Reason              string   `json:"reason,omitempty"`
	VideoNotice         string   `json:"videoNotice,omitempty"`
}

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

type Job struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Status            JobStatus `json:"status"`
	Progress          int       `json:"progress,omitempty"`
	ProviderRequestID string    `json:"providerRequestId,omitempty"`
	Error             string    `json:"error,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Reference struct {
	AssetID string `json:"assetId"`
	Role    string `json:"role"`
	Path    string `json:"-"`
}

type ImageRequest struct {
	Prompt     string         `json:"prompt"`
	Model      string         `json:"model,omitempty"`
	Size       string         `json:"size,omitempty"`
	Quality    string         `json:"quality,omitempty"`
	References []Reference    `json:"references,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type VideoRequest struct {
	Prompt     string         `json:"prompt"`
	Model      string         `json:"model,omitempty"`
	Seconds    int            `json:"seconds,omitempty"`
	Size       string         `json:"size,omitempty"`
	References []Reference    `json:"references,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type SpeechRequest struct {
	Text           string         `json:"text"`
	Model          string         `json:"model,omitempty"`
	Voice          string         `json:"voice,omitempty"`
	VoiceProfileID string         `json:"voiceProfileId"`
	Instructions   string         `json:"instructions,omitempty"`
	ResponseFormat string         `json:"responseFormat,omitempty"`
	Speed          float64        `json:"speed,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
}

type SpeechResult struct {
	ProviderRequestID string `json:"providerRequestId,omitempty"`
	Model             string `json:"model"`
	Voice             string `json:"voice"`
	Format            string `json:"format"`
}

type CustomVoiceRequest struct {
	Name        string `json:"name"`
	Language    string `json:"language"`
	ConsentPath string `json:"-"`
	SamplePath  string `json:"-"`
	Confirmed   bool   `json:"confirmed"`
}

type CustomVoiceResult struct {
	ProviderVoiceID string `json:"providerVoiceId"`
	ConsentID       string `json:"consentId"`
}

type SoundRequest struct {
	Prompt          string         `json:"prompt"`
	DurationSeconds float64        `json:"durationSeconds"`
	Parameters      map[string]any `json:"parameters,omitempty"`
}

type ImageProvider interface {
	Name() string
	ImageCapabilities(ctx context.Context) (Capabilities, error)
	GenerateImage(ctx context.Context, request ImageRequest) (Job, error)
	DownloadImage(ctx context.Context, jobID string, destination io.Writer) error
}

type VideoProvider interface {
	Name() string
	VideoCapabilities(ctx context.Context) (Capabilities, error)
	GenerateVideo(ctx context.Context, request VideoRequest) (Job, error)
	GetVideoJob(ctx context.Context, jobID string) (Job, error)
	CancelVideoJob(ctx context.Context, jobID string) (Job, error)
	DownloadVideo(ctx context.Context, jobID string, destination io.Writer) error
}

type SpeechProvider interface {
	Name() string
	SpeechCapabilities(ctx context.Context) (Capabilities, error)
	GenerateSpeech(ctx context.Context, request SpeechRequest, destination io.Writer) (SpeechResult, error)
	CreateCustomVoice(ctx context.Context, request CustomVoiceRequest) (CustomVoiceResult, error)
}

type SoundProvider interface {
	Name() string
	SoundCapabilities(ctx context.Context) (Capabilities, error)
	GenerateSound(ctx context.Context, request SoundRequest, destination io.Writer) error
}

// MediaProvider remains a convenience for providers that implement both visual
// capabilities; the application itself accepts the narrower interfaces.
type MediaProvider interface {
	ImageProvider
	VideoProvider
}
