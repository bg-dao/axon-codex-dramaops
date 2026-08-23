package provider

import (
	"context"
	"io"
	"time"
)

type Capabilities struct {
	ImageGeneration   bool     `json:"imageGeneration"`
	VideoGeneration   bool     `json:"videoGeneration"`
	VideoReferences   bool     `json:"videoReferences"`
	VideoExperimental bool     `json:"videoExperimental"`
	ImageModels       []string `json:"imageModels"`
	VideoModels       []string `json:"videoModels"`
	Reason            string   `json:"reason,omitempty"`
	VideoNotice       string   `json:"videoNotice,omitempty"`
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

type ImageRequest struct {
	Prompt         string         `json:"prompt"`
	Model          string         `json:"model,omitempty"`
	Size           string         `json:"size,omitempty"`
	Quality        string         `json:"quality,omitempty"`
	ReferencePaths []string       `json:"referencePaths,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
}

type VideoRequest struct {
	Prompt           string         `json:"prompt"`
	Model            string         `json:"model,omitempty"`
	Seconds          int            `json:"seconds,omitempty"`
	Size             string         `json:"size,omitempty"`
	ReferenceAssetID string         `json:"referenceAssetId,omitempty"`
	ReferencePath    string         `json:"-"`
	Parameters       map[string]any `json:"parameters,omitempty"`
}

type MediaProvider interface {
	Name() string
	Capabilities(ctx context.Context) (Capabilities, error)
	GenerateImage(ctx context.Context, request ImageRequest) (Job, error)
	GenerateVideo(ctx context.Context, request VideoRequest) (Job, error)
	GetJob(ctx context.Context, jobID string) (Job, error)
	CancelJob(ctx context.Context, jobID string) (Job, error)
	DownloadResult(ctx context.Context, jobID string, destination io.Writer) error
}
