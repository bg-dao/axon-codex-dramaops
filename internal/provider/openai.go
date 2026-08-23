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

	"github.com/bg-dao/axon-codex-sceneops/internal/redact"
	"github.com/google/uuid"
)

const (
	DefaultImageModel = "gpt-image-2"
	DefaultVideoModel = "sora-2"
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
	return &OpenAI{
		baseURL:    "https://api.openai.com/v1",
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		apiKey:     apiKey,
		images:     make(map[string][]byte),
	}
}

func NewOpenAIWithClient(baseURL string, client *http.Client, apiKey APIKeySource) *OpenAI {
	return &OpenAI{baseURL: strings.TrimRight(baseURL, "/"), httpClient: client, apiKey: apiKey, images: make(map[string][]byte)}
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) Capabilities(ctx context.Context) (Capabilities, error) {
	result := Capabilities{
		ImageGeneration:   true,
		ImageModels:       []string{DefaultImageModel},
		VideoExperimental: true,
		VideoNotice:       "OpenAI Videos API is deprecated and scheduled to shut down on September 24, 2026; prefer external video import.",
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
		result.VideoGeneration = true
		result.VideoReferences = true
		result.VideoModels = []string{DefaultVideoModel}
	} else {
		result.Reason = "OpenAI Videos API is not available for this API key"
	}
	return result, nil
}

func (o *OpenAI) GenerateImage(ctx context.Context, input ImageRequest) (Job, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return Job{}, errors.New("image prompt is required")
	}
	if len(input.ReferencePaths) > 0 {
		return Job{}, errors.New("image references are not supported by the v0.1 generation adapter; import or edit support will be added separately")
	}
	model := input.Model
	if model == "" {
		model = DefaultImageModel
	}
	size := input.Size
	if size == "" {
		size = "1536x1024"
	}
	quality := input.Quality
	if quality == "" {
		quality = "medium"
	}
	payload := map[string]any{
		"model":         model,
		"prompt":        input.Prompt,
		"size":          size,
		"quality":       quality,
		"output_format": "png",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Job{}, err
	}
	request, err := o.request(ctx, http.MethodPost, "/images/generations", bytes.NewReader(body), "application/json")
	if err != nil {
		return Job{}, err
	}
	response, err := o.httpClient.Do(request)
	if err != nil {
		return Job{}, fmt.Errorf("OpenAI image request failed: %w", err)
	}
	defer response.Body.Close()
	if err := requireSuccess(response); err != nil {
		return Job{}, err
	}
	var decoded struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return Job{}, fmt.Errorf("decode OpenAI image response: %w", err)
	}
	if len(decoded.Data) == 0 || decoded.Data[0].B64JSON == "" {
		return Job{}, errors.New("OpenAI image response contained no image data")
	}
	imageBytes, err := base64.StdEncoding.DecodeString(decoded.Data[0].B64JSON)
	if err != nil {
		return Job{}, fmt.Errorf("decode OpenAI image data: %w", err)
	}
	jobID := "image_" + uuid.NewString()
	o.imagesMu.Lock()
	o.images[jobID] = imageBytes
	o.imagesMu.Unlock()
	now := time.Now().UTC()
	return Job{ID: jobID, Kind: "image", Status: JobSucceeded, Progress: 100, ProviderRequestID: response.Header.Get("x-request-id"), CreatedAt: now, UpdatedAt: now}, nil
}

func (o *OpenAI) GenerateVideo(ctx context.Context, input VideoRequest) (Job, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return Job{}, errors.New("video prompt is required")
	}
	model := input.Model
	if model == "" {
		model = DefaultVideoModel
	}
	seconds := input.Seconds
	if seconds == 0 {
		seconds = 4
	}
	if seconds != 4 && seconds != 8 && seconds != 12 {
		return Job{}, fmt.Errorf("video seconds must be 4, 8, or 12")
	}
	size := input.Size
	if size == "" {
		size = "1280x720"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("prompt", input.Prompt)
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("seconds", strconv.Itoa(seconds))
	_ = writer.WriteField("size", size)
	if input.ReferencePath != "" {
		file, err := os.Open(input.ReferencePath)
		if err != nil {
			return Job{}, fmt.Errorf("open video input reference: %w", err)
		}
		part, err := writer.CreateFormFile("input_reference", filepath.Base(input.ReferencePath))
		if err == nil {
			_, err = io.Copy(part, file)
		}
		closeErr := file.Close()
		if err != nil {
			return Job{}, fmt.Errorf("encode video input reference: %w", err)
		}
		if closeErr != nil {
			return Job{}, closeErr
		}
	}
	if err := writer.Close(); err != nil {
		return Job{}, err
	}
	request, err := o.request(ctx, http.MethodPost, "/videos", &body, writer.FormDataContentType())
	if err != nil {
		return Job{}, err
	}
	response, err := o.httpClient.Do(request)
	if err != nil {
		return Job{}, fmt.Errorf("OpenAI video request failed: %w", err)
	}
	defer response.Body.Close()
	if err := requireSuccess(response); err != nil {
		return Job{}, err
	}
	job, err := decodeVideoJob(response.Body)
	if err != nil {
		return Job{}, err
	}
	job.ProviderRequestID = response.Header.Get("x-request-id")
	return job, nil
}

func (o *OpenAI) GetJob(ctx context.Context, jobID string) (Job, error) {
	o.imagesMu.RLock()
	_, imageExists := o.images[jobID]
	o.imagesMu.RUnlock()
	if imageExists {
		now := time.Now().UTC()
		return Job{ID: jobID, Kind: "image", Status: JobSucceeded, Progress: 100, UpdatedAt: now}, nil
	}
	response, err := o.do(ctx, http.MethodGet, "/videos/"+url.PathEscape(jobID), nil, "")
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	return decodeVideoJob(response.Body)
}

func (o *OpenAI) CancelJob(ctx context.Context, jobID string) (Job, error) {
	o.imagesMu.Lock()
	if _, ok := o.images[jobID]; ok {
		delete(o.images, jobID)
		o.imagesMu.Unlock()
		now := time.Now().UTC()
		return Job{ID: jobID, Kind: "image", Status: JobCancelled, UpdatedAt: now}, nil
	}
	o.imagesMu.Unlock()
	response, err := o.do(ctx, http.MethodDelete, "/videos/"+url.PathEscape(jobID), nil, "")
	if err != nil {
		return Job{}, err
	}
	defer response.Body.Close()
	now := time.Now().UTC()
	return Job{ID: jobID, Kind: "video", Status: JobCancelled, UpdatedAt: now}, nil
}

func (o *OpenAI) DownloadResult(ctx context.Context, jobID string, destination io.Writer) error {
	o.imagesMu.RLock()
	imageBytes, imageExists := o.images[jobID]
	o.imagesMu.RUnlock()
	if imageExists {
		_, err := io.Copy(destination, bytes.NewReader(imageBytes))
		return err
	}
	response, err := o.do(ctx, http.MethodGet, "/videos/"+url.PathEscape(jobID)+"/content", nil, "")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(destination, response.Body)
	return err
}

func (o *OpenAI) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	request, err := o.request(ctx, method, path, body, contentType)
	if err != nil {
		return nil, err
	}
	response, err := o.httpClient.Do(request)
	if err != nil {
		return nil, err
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
