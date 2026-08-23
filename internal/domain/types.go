package domain

import "time"

const SchemaVersion = 1

type Orientation string

const (
	OrientationPortrait  Orientation = "portrait"
	OrientationLandscape Orientation = "landscape"
)

type OutputSettings struct {
	Orientation      Orientation `json:"orientation"`
	Width            int         `json:"width"`
	Height           int         `json:"height"`
	FPS              int         `json:"fps"`
	VideoCodec       string      `json:"videoCodec"`
	AudioCodec       string      `json:"audioCodec"`
	AudioSampleRate  int         `json:"audioSampleRate"`
	AudioChannels    int         `json:"audioChannels"`
	LoudnessLUFS     float64     `json:"loudnessLufs"`
	TruePeakDBTP     float64     `json:"truePeakDbtp"`
	BurnSubtitles    bool        `json:"burnSubtitles"`
	SubtitleSafeArea float64     `json:"subtitleSafeArea"`
}

func DefaultOutputSettings(orientation Orientation) OutputSettings {
	if orientation == OrientationLandscape {
		return OutputSettings{
			Orientation: orientation, Width: 1920, Height: 1080, FPS: 25,
			VideoCodec: "h264", AudioCodec: "aac", AudioSampleRate: 48000,
			AudioChannels: 2, LoudnessLUFS: -16, TruePeakDBTP: -1,
			BurnSubtitles: false, SubtitleSafeArea: 0.08,
		}
	}
	return OutputSettings{
		Orientation: OrientationPortrait, Width: 1080, Height: 1920, FPS: 25,
		VideoCodec: "h264", AudioCodec: "aac", AudioSampleRate: 48000,
		AudioChannels: 2, LoudnessLUFS: -16, TruePeakDBTP: -1,
		BurnSubtitles: true, SubtitleSafeArea: 0.12,
	}
}

type StyleBible struct {
	VisualStyle     string   `json:"visualStyle,omitempty"`
	ColorPalette    []string `json:"colorPalette,omitempty"`
	LightingRules   string   `json:"lightingRules,omitempty"`
	NegativePrompt  string   `json:"negativePrompt,omitempty"`
	ReferenceAssets []string `json:"referenceAssets,omitempty"`
}

type SoundPalette struct {
	AmbienceAssetIDs []string          `json:"ambienceAssetIds,omitempty"`
	BGMAssetIDs      []string          `json:"bgmAssetIds,omitempty"`
	Motifs           map[string]string `json:"motifs,omitempty"`
	Notes            string            `json:"notes,omitempty"`
}

type Project struct {
	SchemaVersion   int            `json:"schemaVersion"`
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ContentLanguage string         `json:"contentLanguage"`
	ActiveEpisodeID string         `json:"activeEpisodeId,omitempty"`
	ActiveThreadID  string         `json:"activeThreadId,omitempty"`
	StyleBible      StyleBible     `json:"styleBible"`
	SoundPalette    SoundPalette   `json:"soundPalette"`
	Output          OutputSettings `json:"output"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type EpisodeStatus string

const (
	EpisodeDraft      EpisodeStatus = "draft"
	EpisodePlanning   EpisodeStatus = "planning"
	EpisodeProduction EpisodeStatus = "production"
	EpisodeEditing    EpisodeStatus = "editing"
	EpisodeComplete   EpisodeStatus = "complete"
)

type ScriptBlockKind string

const (
	ScriptAction    ScriptBlockKind = "action"
	ScriptDialogue  ScriptBlockKind = "dialogue"
	ScriptVoiceOver ScriptBlockKind = "voice_over"
	ScriptSFX       ScriptBlockKind = "sfx"
	ScriptMusic     ScriptBlockKind = "music"
)

type ScriptBlock struct {
	ID                   string          `json:"id"`
	SceneID              string          `json:"sceneId"`
	Kind                 ScriptBlockKind `json:"kind"`
	Order                int             `json:"order"`
	CharacterID          string          `json:"characterId,omitempty"`
	Text                 string          `json:"text"`
	Emotion              string          `json:"emotion,omitempty"`
	SelectedVoiceAssetID string          `json:"selectedVoiceAssetId,omitempty"`
}

type Episode struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            string        `json:"id"`
	Number        int           `json:"number"`
	Title         string        `json:"title"`
	Logline       string        `json:"logline,omitempty"`
	Synopsis      string        `json:"synopsis,omitempty"`
	Status        EpisodeStatus `json:"status"`
	SceneIDs      []string      `json:"sceneIds"`
	ScriptBlocks  []ScriptBlock `json:"scriptBlocks"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type VoiceProfileKind string

const (
	VoiceBuiltIn  VoiceProfileKind = "built_in"
	VoiceCustom   VoiceProfileKind = "custom"
	VoiceExternal VoiceProfileKind = "external"
)

type VoiceProfile struct {
	ID               string           `json:"id"`
	Kind             VoiceProfileKind `json:"kind"`
	Name             string           `json:"name"`
	BuiltInVoice     string           `json:"builtInVoice,omitempty"`
	ExternalAssetID  string           `json:"externalAssetId,omitempty"`
	ConsentConfirmed bool             `json:"consentConfirmed,omitempty"`
}

type Character struct {
	SchemaVersion   int          `json:"schemaVersion"`
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	Appearance      string       `json:"appearance,omitempty"`
	Wardrobe        string       `json:"wardrobe,omitempty"`
	NegativePrompt  string       `json:"negativePrompt,omitempty"`
	ReferenceAssets []string     `json:"referenceAssets,omitempty"`
	VoiceProfile    VoiceProfile `json:"voiceProfile"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

type Location struct {
	SchemaVersion   int       `json:"schemaVersion"`
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	ContinuityNotes string    `json:"continuityNotes,omitempty"`
	ReferenceAssets []string  `json:"referenceAssets,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Prop struct {
	SchemaVersion   int       `json:"schemaVersion"`
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	ContinuityState string    `json:"continuityState,omitempty"`
	ReferenceAssets []string  `json:"referenceAssets,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Scene struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	EpisodeID     string    `json:"episodeId"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	LocationID    string    `json:"locationId,omitempty"`
	TimeOfDay     string    `json:"timeOfDay,omitempty"`
	Order         int       `json:"order"`
	ShotIDs       []string  `json:"shotIds"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ShotSize string

const (
	ShotECU ShotSize = "ECU"
	ShotCU  ShotSize = "CU"
	ShotMCU ShotSize = "MCU"
	ShotMS  ShotSize = "MS"
	ShotMLS ShotSize = "MLS"
	ShotLS  ShotSize = "LS"
	ShotELS ShotSize = "ELS"
)

type CameraAngle string

const (
	AngleEyeLevel CameraAngle = "eye_level"
	AngleHigh     CameraAngle = "high"
	AngleLow      CameraAngle = "low"
	AngleOverhead CameraAngle = "overhead"
	AngleDutch    CameraAngle = "dutch"
	AnglePOV      CameraAngle = "pov"
	AngleOTS      CameraAngle = "over_the_shoulder"
)

type CameraMovement string

const (
	MovementStatic   CameraMovement = "static"
	MovementPan      CameraMovement = "pan"
	MovementTilt     CameraMovement = "tilt"
	MovementDolly    CameraMovement = "dolly"
	MovementTruck    CameraMovement = "truck"
	MovementPedestal CameraMovement = "pedestal"
	MovementOrbit    CameraMovement = "orbit"
	MovementHandheld CameraMovement = "handheld"
	MovementCrane    CameraMovement = "crane"
	MovementZoom     CameraMovement = "zoom"
)

type TransitionKind string

const (
	TransitionCut      TransitionKind = "cut"
	TransitionDissolve TransitionKind = "dissolve"
	TransitionFade     TransitionKind = "fade"
)

type Shot struct {
	SchemaVersion           int            `json:"schemaVersion"`
	ID                      string         `json:"id"`
	EpisodeID               string         `json:"episodeId"`
	SceneID                 string         `json:"sceneId"`
	Title                   string         `json:"title"`
	Order                   int            `json:"order"`
	ScriptBlockIDs          []string       `json:"scriptBlockIds,omitempty"`
	Prompt                  string         `json:"prompt,omitempty"`
	DurationSeconds         float64        `json:"durationSeconds"`
	AspectRatio             string         `json:"aspectRatio"`
	ShotSize                ShotSize       `json:"shotSize"`
	CameraAngle             CameraAngle    `json:"cameraAngle"`
	CameraMovement          CameraMovement `json:"cameraMovement"`
	LensMM                  int            `json:"lensMm,omitempty"`
	Composition             string         `json:"composition,omitempty"`
	FocusSubject            string         `json:"focusSubject,omitempty"`
	Blocking                string         `json:"blocking,omitempty"`
	Lighting                string         `json:"lighting,omitempty"`
	ScreenDirection         string         `json:"screenDirection,omitempty"`
	EyeLine                 string         `json:"eyeLine,omitempty"`
	CharacterIDs            []string       `json:"characterIds,omitempty"`
	PropIDs                 []string       `json:"propIds,omitempty"`
	WardrobeContinuity      string         `json:"wardrobeContinuity,omitempty"`
	PropContinuity          string         `json:"propContinuity,omitempty"`
	Transition              TransitionKind `json:"transition"`
	ReferenceAssets         []string       `json:"referenceAssets,omitempty"`
	SelectedKeyframeAssetID string         `json:"selectedKeyframeAssetId,omitempty"`
	SelectedVideoAssetID    string         `json:"selectedVideoAssetId,omitempty"`
	CreatedAt               time.Time      `json:"createdAt"`
	UpdatedAt               time.Time      `json:"updatedAt"`
}

type AssetKind string

const (
	AssetKindImage     AssetKind = "image"
	AssetKindVideo     AssetKind = "video"
	AssetKindReference AssetKind = "reference"
	AssetKindAudio     AssetKind = "audio"
	AssetKindSubtitle  AssetKind = "subtitle"
	AssetKindRender    AssetKind = "render"
)

type AssetInput struct {
	AssetID string `json:"assetId"`
	Role    string `json:"role"`
}

type MediaInfo struct {
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	FPS             float64 `json:"fps,omitempty"`
	VideoCodec      string  `json:"videoCodec,omitempty"`
	AudioCodec      string  `json:"audioCodec,omitempty"`
	SampleRate      int     `json:"sampleRate,omitempty"`
	Channels        int     `json:"channels,omitempty"`
}

type Provenance struct {
	Provider          string         `json:"provider"`
	Model             string         `json:"model,omitempty"`
	Prompt            string         `json:"prompt,omitempty"`
	Parameters        map[string]any `json:"parameters,omitempty"`
	ProviderRequestID string         `json:"providerRequestId,omitempty"`
	ToolVersion       string         `json:"toolVersion,omitempty"`
	GeneratedAt       time.Time      `json:"generatedAt,omitempty"`
}

type Asset struct {
	SchemaVersion int          `json:"schemaVersion"`
	ID            string       `json:"id"`
	EpisodeID     string       `json:"episodeId,omitempty"`
	ShotID        string       `json:"shotId,omitempty"`
	ScriptBlockID string       `json:"scriptBlockId,omitempty"`
	Kind          AssetKind    `json:"kind"`
	RelativePath  string       `json:"relativePath"`
	SHA256        string       `json:"sha256"`
	Inputs        []AssetInput `json:"inputs,omitempty"`
	RunID         string       `json:"runId,omitempty"`
	MediaInfo     MediaInfo    `json:"mediaInfo,omitempty"`
	Provenance    Provenance   `json:"provenance"`
	CreatedAt     time.Time    `json:"createdAt"`
}

type ClipFit string

const (
	FitCover   ClipFit = "cover"
	FitContain ClipFit = "contain"
)

type VideoClip struct {
	ID                string         `json:"id"`
	ShotID            string         `json:"shotId"`
	AssetID           string         `json:"assetId"`
	Order             int            `json:"order"`
	InSeconds         float64        `json:"inSeconds"`
	OutSeconds        float64        `json:"outSeconds"`
	Transition        TransitionKind `json:"transition"`
	TransitionSeconds float64        `json:"transitionSeconds,omitempty"`
	Fit               ClipFit        `json:"fit"`
}

type AudioLane string

const (
	LaneDialogue AudioLane = "dialogue"
	LaneSFX      AudioLane = "sfx"
	LaneBGM      AudioLane = "bgm"
)

type AudioCue struct {
	ID              string    `json:"id"`
	Lane            AudioLane `json:"lane"`
	AssetID         string    `json:"assetId"`
	ScriptBlockID   string    `json:"scriptBlockId,omitempty"`
	StartSeconds    float64   `json:"startSeconds"`
	DurationSeconds float64   `json:"durationSeconds"`
	GainDB          float64   `json:"gainDb,omitempty"`
	Loop            bool      `json:"loop,omitempty"`
	DuckBGM         bool      `json:"duckBgm,omitempty"`
}

type SubtitleCue struct {
	ID              string  `json:"id"`
	ScriptBlockID   string  `json:"scriptBlockId,omitempty"`
	StartSeconds    float64 `json:"startSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
	Text            string  `json:"text"`
}

type EpisodeEdit struct {
	SchemaVersion int            `json:"schemaVersion"`
	EpisodeID     string         `json:"episodeId"`
	VideoTrack    []VideoClip    `json:"videoTrack"`
	AudioCues     []AudioCue     `json:"audioCues"`
	SubtitleCues  []SubtitleCue  `json:"subtitleCues"`
	Output        OutputSettings `json:"output"`
	UpdatedAt     time.Time      `json:"updatedAt"`
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
	EpisodeID     string         `json:"episodeId,omitempty"`
	ShotID        string         `json:"shotId,omitempty"`
	ScriptBlockID string         `json:"scriptBlockId,omitempty"`
	ProviderJobID string         `json:"providerJobId,omitempty"`
	Progress      int            `json:"progress,omitempty"`
	Error         string         `json:"error,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type ContinuitySeverity string

const (
	ContinuityInfo    ContinuitySeverity = "info"
	ContinuityWarning ContinuitySeverity = "warning"
	ContinuityError   ContinuitySeverity = "error"
)

type ContinuityIssue struct {
	Code      string             `json:"code"`
	Severity  ContinuitySeverity `json:"severity"`
	EpisodeID string             `json:"episodeId,omitempty"`
	SceneID   string             `json:"sceneId,omitempty"`
	ShotID    string             `json:"shotId,omitempty"`
	Message   string             `json:"message"`
}

type Snapshot struct {
	Root             string            `json:"root"`
	Project          Project           `json:"project"`
	Episodes         []Episode         `json:"episodes"`
	Characters       []Character       `json:"characters"`
	Locations        []Location        `json:"locations"`
	Props            []Prop            `json:"props"`
	Scenes           []Scene           `json:"scenes"`
	Shots            []Shot            `json:"shots"`
	Edits            []EpisodeEdit     `json:"edits"`
	Assets           []Asset           `json:"assets"`
	Runs             []Run             `json:"runs"`
	ContinuityIssues []ContinuityIssue `json:"continuityIssues"`
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
