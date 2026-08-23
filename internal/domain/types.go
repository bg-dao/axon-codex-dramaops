package domain

import "time"

const SchemaVersion = 1

type Project struct {
	SchemaVersion  int       `json:"schemaVersion"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ActiveThreadID string    `json:"activeThreadId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Scene struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	Order         int       `json:"order"`
	ShotIDs       []string  `json:"shotIds"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Shot struct {
	SchemaVersion   int            `json:"schemaVersion"`
	ID              string         `json:"id"`
	SceneID         string         `json:"sceneId"`
	Title           string         `json:"title"`
	Order           int            `json:"order"`
	Prompt          string         `json:"prompt,omitempty"`
	DurationSeconds int            `json:"durationSeconds,omitempty"`
	AspectRatio     string         `json:"aspectRatio,omitempty"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	ReferenceAssets []string       `json:"referenceAssets,omitempty"`
	SelectedAssetID string         `json:"selectedAssetId,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type AssetKind string

const (
	AssetKindImage     AssetKind = "image"
	AssetKindVideo     AssetKind = "video"
	AssetKindReference AssetKind = "reference"
)

type Provenance struct {
	Provider          string         `json:"provider"`
	Model             string         `json:"model,omitempty"`
	Prompt            string         `json:"prompt,omitempty"`
	Parameters        map[string]any `json:"parameters,omitempty"`
	ProviderRequestID string         `json:"providerRequestId,omitempty"`
	GeneratedAt       time.Time      `json:"generatedAt,omitempty"`
}

type Asset struct {
	SchemaVersion int        `json:"schemaVersion"`
	ID            string     `json:"id"`
	ShotID        string     `json:"shotId,omitempty"`
	Kind          AssetKind  `json:"kind"`
	RelativePath  string     `json:"relativePath"`
	SHA256        string     `json:"sha256"`
	ParentAssetID string     `json:"parentAssetId,omitempty"`
	RunID         string     `json:"runId,omitempty"`
	Provenance    Provenance `json:"provenance"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type RunStatus string

const (
	RunQueued           RunStatus = "queued"
	RunAwaitingApproval RunStatus = "awaiting_approval"
	RunRunning          RunStatus = "running"
	RunSucceeded        RunStatus = "succeeded"
	RunFailed           RunStatus = "failed"
	RunCancelled        RunStatus = "cancelled"
)

type Run struct {
	SchemaVersion int            `json:"schemaVersion"`
	ID            string         `json:"id"`
	Operation     string         `json:"operation"`
	Status        RunStatus      `json:"status"`
	ShotID        string         `json:"shotId,omitempty"`
	ProviderJobID string         `json:"providerJobId,omitempty"`
	Error         string         `json:"error,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type Snapshot struct {
	Root    string  `json:"root"`
	Brief   string  `json:"brief"`
	Project Project `json:"project"`
	Scenes  []Scene `json:"scenes"`
	Shots   []Shot  `json:"shots"`
	Assets  []Asset `json:"assets"`
	Runs    []Run   `json:"runs"`
}

func CanTransitionRun(from, to RunStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case RunQueued:
		return to == RunAwaitingApproval || to == RunRunning || to == RunCancelled || to == RunFailed
	case RunAwaitingApproval:
		return to == RunRunning || to == RunCancelled || to == RunFailed
	case RunRunning:
		return to == RunSucceeded || to == RunFailed || to == RunCancelled
	case RunSucceeded, RunFailed, RunCancelled:
		return false
	default:
		return false
	}
}
