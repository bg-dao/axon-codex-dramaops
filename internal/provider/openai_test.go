package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
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
	if err := client.DownloadResult(context.Background(), job.ID, &output); err != nil {
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
	if err != nil || !capabilities.VideoGeneration {
		t.Fatalf("capabilities = %+v, err = %v", capabilities, err)
	}
	job, err := client.GenerateVideo(context.Background(), VideoRequest{Prompt: "slow camera move"})
	if err != nil || job.Status != JobQueued {
		t.Fatalf("generate = %+v, err = %v", job, err)
	}
	job, _ = client.GetJob(context.Background(), job.ID)
	if job.Status != JobRunning {
		t.Fatalf("first poll = %+v", job)
	}
	job, _ = client.GetJob(context.Background(), job.ID)
	if job.Status != JobSucceeded {
		t.Fatalf("second poll = %+v", job)
	}
	var output bytes.Buffer
	if err := client.DownloadResult(context.Background(), job.ID, &output); err != nil || output.String() != "fake-mp4" {
		t.Fatalf("download = %q, err = %v", output.String(), err)
	}
	cancelled, err := client.CancelJob(context.Background(), job.ID)
	if err != nil || cancelled.Status != JobCancelled {
		t.Fatalf("cancel = %+v, err = %v", cancelled, err)
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
