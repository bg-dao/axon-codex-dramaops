package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOpenAIImageGenerationAndDownload(t *testing.T) {
	image := []byte("fake-png-data")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header was not set")
		}
		if request.URL.Path != "/images/generations" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		writer.Header().Set("x-request-id", "req_image_1")
		_, _ = io.WriteString(writer, `{"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString(image)+`"}]}`)
	}))
	defer server.Close()
	client := NewOpenAIWithClient(server.URL, server.Client(), func() (string, error) { return "test-key", nil })
	job, err := client.GenerateImage(context.Background(), ImageRequest{Prompt: "a precise frame"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != JobSucceeded || job.ProviderRequestID != "req_image_1" {
		t.Fatalf("unexpected job: %+v", job)
	}
	var output bytes.Buffer
	if err := client.DownloadImage(context.Background(), job.ID, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), image) {
		t.Fatalf("download = %q", output.Bytes())
	}
}

func TestOpenAIVideoLifecycle(t *testing.T) {
	var retrievals atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/models/sora-2":
			_, _ = io.WriteString(writer, `{"id":"sora-2"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/videos":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if request.FormValue("seconds") != "4" || request.FormValue("size") != "1280x720" {
				t.Fatalf("unexpected video defaults: %s %s", request.FormValue("seconds"), request.FormValue("size"))
			}
			file, _, err := request.FormFile("input_reference")
			if err != nil {
				t.Fatalf("missing input_reference: %v", err)
			}
			data, _ := io.ReadAll(file)
			_ = file.Close()
			if string(data) != "reference-image" {
				t.Fatalf("input_reference = %q", data)
			}
			_, _ = io.WriteString(writer, `{"id":"video_1","status":"queued","progress":0,"created_at":1712697600}`)
		case request.Method == http.MethodGet && request.URL.Path == "/videos/video_1":
			count := retrievals.Add(1)
			if count == 1 {
				_, _ = io.WriteString(writer, `{"id":"video_1","status":"in_progress","progress":48,"created_at":1712697600}`)
			} else {
				_, _ = io.WriteString(writer, `{"id":"video_1","status":"completed","progress":100,"created_at":1712697600}`)
			}
		case request.Method == http.MethodGet && request.URL.Path == "/videos/video_1/content":
			_, _ = writer.Write([]byte("fake-mp4"))
		case request.Method == http.MethodDelete && request.URL.Path == "/videos/video_1":
			_, _ = io.WriteString(writer, `{}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewOpenAIWithClient(server.URL, server.Client(), func() (string, error) { return "test-key", nil })
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.VideoGeneration || capabilities.MaxVideoReferences != 1 || !capabilities.VideoExperimental {
		t.Fatalf("capabilities = %+v, err = %v", capabilities, err)
	}
	referencePath := filepath.Join(t.TempDir(), "reference.png")
	if err := os.WriteFile(referencePath, []byte("reference-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := client.GenerateVideo(context.Background(), VideoRequest{
		Prompt: "slow camera move", Size: "1280x720",
		References: []Reference{{AssetID: "keyframe-1", Role: "keyframe", Path: referencePath}},
	})
	if err != nil || job.Status != JobQueued {
		t.Fatalf("generate = %+v, err = %v", job, err)
	}
	job, _ = client.GetVideoJob(context.Background(), job.ID)
	if job.Status != JobRunning {
		t.Fatalf("first poll = %+v", job)
	}
	job, _ = client.GetVideoJob(context.Background(), job.ID)
	if job.Status != JobSucceeded {
		t.Fatalf("second poll = %+v", job)
	}
	var output bytes.Buffer
	if err := client.DownloadVideo(context.Background(), job.ID, &output); err != nil || output.String() != "fake-mp4" {
		t.Fatalf("download = %q, err = %v", output.String(), err)
	}
	cancelled, err := client.CancelVideoJob(context.Background(), job.ID)
	if err != nil || cancelled.Status != JobCancelled {
		t.Fatalf("cancel = %+v, err = %v", cancelled, err)
	}
}

func TestOpenAIImageEditUsesMultipleHighFidelityReferences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/images/edits" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		if err := request.ParseMultipartForm(2 << 20); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("input_fidelity") != "high" {
			t.Fatalf("input_fidelity = %q", request.FormValue("input_fidelity"))
		}
		if got := len(request.MultipartForm.File["image[]"]); got != 2 {
			t.Fatalf("reference count = %d", got)
		}
		_, _ = io.WriteString(writer, `{"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString([]byte("edited"))+`"}]}`)
	}))
	defer server.Close()
	directory := t.TempDir()
	references := make([]Reference, 0, 2)
	for index, contents := range []string{"character", "location"} {
		path := filepath.Join(directory, string(rune('a'+index))+".png")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		references = append(references, Reference{AssetID: contents, Role: contents, Path: path})
	}
	client := NewOpenAIWithClient(server.URL, server.Client(), func() (string, error) { return "test-key", nil })
	if _, err := client.GenerateImage(context.Background(), ImageRequest{Prompt: "consistent frame", References: references}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAISpeechAndCustomVoiceConsent(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/audio/speech":
			writer.Header().Set("x-request-id", "req_speech")
			_, _ = writer.Write([]byte("wav"))
		case "/audio/voice_consents":
			calls.Add(1)
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := request.FormFile("recording"); err != nil {
				t.Fatalf("consent recording missing: %v", err)
			}
			_, _ = io.WriteString(writer, `{"id":"consent_1"}`)
		case "/audio/voices":
			calls.Add(1)
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if request.FormValue("consent") != "consent_1" {
				t.Fatalf("consent = %q", request.FormValue("consent"))
			}
			_, _ = io.WriteString(writer, `{"id":"voice_1"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewOpenAIWithClient(server.URL, server.Client(), func() (string, error) { return "test-key", nil })
	var speech bytes.Buffer
	result, err := client.GenerateSpeech(context.Background(), SpeechRequest{Text: "你好", Voice: "alloy"}, &speech)
	if err != nil || speech.String() != "wav" || result.ProviderRequestID != "req_speech" {
		t.Fatalf("speech = %+v %q, err = %v", result, speech.String(), err)
	}
	if _, err := client.CreateCustomVoice(context.Background(), CustomVoiceRequest{}); err == nil {
		t.Fatal("custom voice without authorization must fail closed")
	}
	consent := filepath.Join(t.TempDir(), "consent.wav")
	sample := filepath.Join(t.TempDir(), "sample.wav")
	for path, contents := range map[string]string{consent: "consent", sample: "sample"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	voice, err := client.CreateCustomVoice(context.Background(), CustomVoiceRequest{Name: "Actor", Language: "zh-CN", ConsentPath: consent, SamplePath: sample, Confirmed: true})
	if err != nil || voice.ProviderVoiceID != "voice_1" || voice.ConsentID != "consent_1" || calls.Load() != 2 {
		t.Fatalf("voice = %+v, calls = %d, err = %v", voice, calls.Load(), err)
	}
}

func TestOpenAIErrorRedactsAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(writer, `{"error":"api_key=sk-supersecret123"}`)
	}))
	defer server.Close()
	client := NewOpenAIWithClient(server.URL, server.Client(), func() (string, error) { return "sk-supersecret123", nil })
	_, err := client.GenerateImage(context.Background(), ImageRequest{Prompt: "test"})
	if err == nil || strings.Contains(err.Error(), "sk-supersecret123") {
		t.Fatalf("error was not safely redacted: %v", err)
	}
}
