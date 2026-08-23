package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bg-dao/axon-codex-dramaops/internal/redact"
	"github.com/google/uuid"
)

const (
	DefaultImageModel  = "gpt-image-2"
	DefaultVideoModel  = "sora-2"
	DefaultSpeechModel = "gpt-4o-mini-tts"
)

type APIKeySource func() (string, error)

type OpenAI struct {
	baseURL    string
	httpClient *http.Client
	apiKey     APIKeySource
	imagesMu   sync.RWMutex
	images     map[string][]byte
}

func NewOpenAI(apiKey APIKeySource) *OpenAI {
	return &OpenAI{baseURL: "https://api.openai.com/v1", httpClient: &http.Client{Timeout: 2 * time.Minute}, apiKey: apiKey, images: make(map[string][]byte)}
}

func NewOpenAIWithClient(baseURL string, client *http.Client, apiKey APIKeySource) *OpenAI {
	return &OpenAI{baseURL: strings.TrimRight(baseURL, "/"), httpClient: client, apiKey: apiKey, images: make(map[string][]byte)}
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) ImageCapabilities(_ context.Context) (Capabilities, error) {
	return Capabilities{ImageGeneration: true, ImageReferences: true, MaxImageReferences: 8, ImageModels: []string{DefaultImageModel}}, nil
}

func (o *OpenAI) SpeechCapabilities(_ context.Context) (Capabilities, error) {
	return Capabilities{
		SpeechGeneration: true, CustomVoices: true, SpeechModels: []string{DefaultSpeechModel},
		BuiltInVoices: []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer"},
	}, nil
}

func (o *OpenAI) VideoCapabilities(ctx context.Context) (Capabilities, error) {
	result := Capabilities{
		VideoExperimental: true, VideoReferenceRoles: []string{"keyframe"}, MaxVideoReferences: 1,
		VideoNotice: "OpenAI Videos API is deprecated and scheduled to shut down on September 24, 2026; prefer external video import.",
	}
	request, err := o.request(ctx, http.MethodGet, "/models/"+url.PathEscape(DefaultVideoModel), nil, "")
	if err != nil {
		result.Reason = err.Error()
		return result, nil
	}
	response, err := o.httpClient.Do(request)
	if err != nil {
		result.Reason = "video capability probe failed"
		return result, nil
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		result.VideoGeneration, result.VideoModels = true, []string{DefaultVideoModel}
	} else {
		result.Reason = "OpenAI Videos API is not available for this API key"
	}
	return result, nil
}

func (o *OpenAI) Capabilities(ctx context.Context) (Capabilities, error) {
	image, _ := o.ImageCapabilities(ctx)
	speech, _ := o.SpeechCapabilities(ctx)
	video, err := o.VideoCapabilities(ctx)
	video.ImageGeneration, video.ImageReferences, video.MaxImageReferences, video.ImageModels = image.ImageGeneration, image.ImageReferences, image.MaxImageReferences, image.ImageModels
	video.SpeechGeneration, video.CustomVoices, video.SpeechModels, video.BuiltInVoices = speech.SpeechGeneration, speech.CustomVoices, speech.SpeechModels, speech.BuiltInVoices
	return video, err
}

func (o *OpenAI) GenerateImage(ctx context.Context, input ImageRequest) (Job, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return Job{}, errors.New("image prompt is required")
	}
	model := defaultString(input.Model, DefaultImageModel)
	size := defaultString(input.Size, "1024x1536")
	quality := defaultString(input.Quality, "medium")
	if len(input.References) > 0 {
		return o.generateImageEdit(ctx, input, model, size, quality)
	}
	payload := map[string]any{"model": model, "prompt": input.Prompt, "size": size, "quality": quality, "output_format": "png"}
	body, err := json.Marshal(payload)
	if err != nil {
		return Job{}, err
	}
	response, err := o.perform(ctx, http.MethodPost, "/images/generations", bytes.NewReader(body), "application/json")
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	imageBytes, err := decodeImage(response.Body)
	if err != nil {
		return Job{}, err
	}
	return o.storeImage(imageBytes, response.Header.Get("x-request-id")), nil
}

func (o *OpenAI) generateImageEdit(ctx context.Context, input ImageRequest, model, size, quality string) (Job, error) {
	if len(input.References) > 8 {
		return Job{}, errors.New("OpenAI image edit accepts at most 8 references")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"model": model, "prompt": input.Prompt, "size": size, "quality": quality, "output_format": "png", "input_fidelity": "high"} {
		_ = writer.WriteField(key, value)
	}
	for _, reference := range input.References {
		if strings.TrimSpace(reference.Path) == "" {
			return Job{}, fmt.Errorf("reference %s has no local path", reference.AssetID)
		}
		if err := appendFile(writer, "image[]", reference.Path); err != nil {
			return Job{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Job{}, err
	}
	response, err := o.perform(ctx, http.MethodPost, "/images/edits", &body, writer.FormDataContentType())
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	imageBytes, err := decodeImage(response.Body)
	if err != nil {
		return Job{}, err
	}
	return o.storeImage(imageBytes, response.Header.Get("x-request-id")), nil
}

func (o *OpenAI) storeImage(content []byte, requestID string) Job {
	jobID := "image_" + uuid.NewString()
	o.imagesMu.Lock()
	o.images[jobID] = content
	o.imagesMu.Unlock()
	now := time.Now().UTC()
	return Job{ID: jobID, Kind: "image", Status: JobSucceeded, Progress: 100, ProviderRequestID: requestID, CreatedAt: now, UpdatedAt: now}
}

func (o *OpenAI) DownloadImage(_ context.Context, jobID string, destination io.Writer) error {
	o.imagesMu.RLock()
	content, ok := o.images[jobID]
	o.imagesMu.RUnlock()
	if !ok {
		return fmt.Errorf("image job %s not found", jobID)
	}
	_, err := io.Copy(destination, bytes.NewReader(content))
	return err
}

func (o *OpenAI) GenerateVideo(ctx context.Context, input VideoRequest) (Job, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return Job{}, errors.New("video prompt is required")
	}
	model := defaultString(input.Model, DefaultVideoModel)
	seconds := input.Seconds
	if seconds == 0 {
		seconds = 4
	}
	if seconds != 4 && seconds != 8 && seconds != 12 {
		return Job{}, errors.New("video seconds must be 4, 8, or 12")
	}
	size := defaultString(input.Size, "720x1280")
	if len(input.References) > 1 {
		return Job{}, errors.New("OpenAI video accepts one keyframe reference")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("prompt", input.Prompt)
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("seconds", strconv.Itoa(seconds))
	_ = writer.WriteField("size", size)
	if len(input.References) == 1 && input.References[0].Path != "" {
		if err := appendFile(writer, "input_reference", input.References[0].Path); err != nil {
			return Job{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Job{}, err
	}
	response, err := o.perform(ctx, http.MethodPost, "/videos", &body, writer.FormDataContentType())
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	job, err := decodeVideoJob(response.Body)
	if err != nil {
		return Job{}, err
	}
	job.ProviderRequestID = response.Header.Get("x-request-id")
	return job, nil
}

func (o *OpenAI) GetVideoJob(ctx context.Context, jobID string) (Job, error) {
	response, err := o.perform(ctx, http.MethodGet, "/videos/"+url.PathEscape(jobID), nil, "")
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	return decodeVideoJob(response.Body)
}

func (o *OpenAI) CancelVideoJob(ctx context.Context, jobID string) (Job, error) {
	response, err := o.perform(ctx, http.MethodDelete, "/videos/"+url.PathEscape(jobID), nil, "")
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	now := time.Now().UTC()
	return Job{ID: jobID, Kind: "video", Status: JobCancelled, UpdatedAt: now}, nil
}

func (o *OpenAI) DownloadVideo(ctx context.Context, jobID string, destination io.Writer) error {
	response, err := o.perform(ctx, http.MethodGet, "/videos/"+url.PathEscape(jobID)+"/content", nil, "")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(destination, response.Body)
	return err
}

func (o *OpenAI) GenerateSpeech(ctx context.Context, input SpeechRequest, destination io.Writer) (SpeechResult, error) {
	if strings.TrimSpace(input.Text) == "" {
		return SpeechResult{}, errors.New("speech text is required")
	}
	if strings.TrimSpace(input.Voice) == "" {
		return SpeechResult{}, errors.New("speech voice is required")
	}
	model := defaultString(input.Model, DefaultSpeechModel)
	format := defaultString(input.ResponseFormat, "wav")
	payload := map[string]any{"model": model, "input": input.Text, "voice": input.Voice, "response_format": format}
	if input.Instructions != "" {
		payload["instructions"] = input.Instructions
	}
	if input.Speed > 0 {
		payload["speed"] = input.Speed
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return SpeechResult{}, err
	}
	response, err := o.perform(ctx, http.MethodPost, "/audio/speech", bytes.NewReader(body), "application/json")
	if err != nil {
		return SpeechResult{}, err
	}
	defer response.Body.Close()
	if _, err := io.Copy(destination, response.Body); err != nil {
		return SpeechResult{}, err
	}
	return SpeechResult{ProviderRequestID: response.Header.Get("x-request-id"), Model: model, Voice: input.Voice, Format: format}, nil
}

func (o *OpenAI) CreateCustomVoice(ctx context.Context, input CustomVoiceRequest) (CustomVoiceResult, error) {
	if !input.Confirmed {
		return CustomVoiceResult{}, errors.New("explicit voice authorization confirmation is required")
	}
	if input.ConsentPath == "" || input.SamplePath == "" {
		return CustomVoiceResult{}, errors.New("consent recording and voice sample are required")
	}
	consentID, err := o.createVoiceConsent(ctx, input)
	if err != nil {
		return CustomVoiceResult{}, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", input.Name)
	_ = writer.WriteField("consent", consentID)
	if err := appendFile(writer, "audio_sample", input.SamplePath); err != nil {
		return CustomVoiceResult{}, err
	}
	if err := writer.Close(); err != nil {
		return CustomVoiceResult{}, err
	}
	response, err := o.perform(ctx, http.MethodPost, "/audio/voices", &body, writer.FormDataContentType())
	if err != nil {
		return CustomVoiceResult{}, err
	}
	defer response.Body.Close()
	var value struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return CustomVoiceResult{}, fmt.Errorf("decode custom voice response: %w", err)
	}
	if value.ID == "" {
		return CustomVoiceResult{}, errors.New("custom voice response contained no id")
	}
	return CustomVoiceResult{ProviderVoiceID: value.ID, ConsentID: consentID}, nil
}

func (o *OpenAI) createVoiceConsent(ctx context.Context, input CustomVoiceRequest) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", input.Name)
	_ = writer.WriteField("language", defaultString(input.Language, "zh"))
	if err := appendFile(writer, "recording", input.ConsentPath); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	response, err := o.perform(ctx, http.MethodPost, "/audio/voice_consents", &body, writer.FormDataContentType())
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var value struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return "", fmt.Errorf("decode voice consent response: %w", err)
	}
	if value.ID == "" {
		return "", errors.New("voice consent response contained no id")
	}
	return value.ID, nil
}

func appendFile(writer *multipart.Writer, field, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", field, err)
	}
	defer file.Close()
	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("encode %s: %w", field, err)
	}
	return nil
}

func decodeImage(reader io.Reader) ([]byte, error) {
	var value struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&value); err != nil {
		return nil, fmt.Errorf("decode OpenAI image response: %w", err)
	}
	if len(value.Data) == 0 || value.Data[0].B64JSON == "" {
		return nil, errors.New("OpenAI image response contained no image data")
	}
	content, err := base64.StdEncoding.DecodeString(value.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode OpenAI image data: %w", err)
	}
	return content, nil
}

func decodeVideoJob(reader io.Reader) (Job, error) {
	var value struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Progress  int    `json:"progress"`
		CreatedAt int64  `json:"created_at"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(reader).Decode(&value); err != nil {
		return Job{}, fmt.Errorf("decode OpenAI video response: %w", err)
	}
	status := JobQueued
	switch value.Status {
	case "in_progress":
		status = JobRunning
	case "completed":
		status = JobSucceeded
	case "failed":
		status = JobFailed
	case "cancelled":
		status = JobCancelled
	}
	created := time.Now().UTC()
	if value.CreatedAt > 0 {
		created = time.Unix(value.CreatedAt, 0).UTC()
	}
	job := Job{ID: value.ID, Kind: "video", Status: status, Progress: value.Progress, CreatedAt: created, UpdatedAt: time.Now().UTC()}
	if value.Error != nil {
		job.Error = redact.String(value.Error.Message)
	}
	return job, nil
}

func (o *OpenAI) perform(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	request, err := o.request(ctx, method, path, body, contentType)
	if err != nil {
		return nil, err
	}
	response, err := o.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	if err := requireSuccess(response); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	return response, nil
}

func (o *OpenAI) request(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Request, error) {
	key, err := o.apiKey()
	if err != nil {
		return nil, fmt.Errorf("OpenAI media API key is not configured: %w", err)
	}
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("OpenAI media API key is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, method, o.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request, nil
}

func requireSuccess(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	limited := io.LimitReader(response.Body, 64<<10)
	body, _ := io.ReadAll(limited)
	return fmt.Errorf("OpenAI API returned %s: %s", response.Status, redact.String(strings.TrimSpace(string(body))))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
