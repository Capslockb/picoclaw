package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

// Ensure transcribers satisfy the Transcriber interface at compile time.
var _ Transcriber = (*GroqTranscriber)(nil)
var _ Transcriber = (*ElevenLabsTranscriber)(nil)

func TestGroqTranscriberName(t *testing.T) {
	tr := NewGroqTranscriber("sk-test")
	if got := tr.Name(); got != "groq" {
		t.Errorf("Name() = %q, want %q", got, "groq")
	}
}

func TestElevenLabsTranscriberName(t *testing.T) {
	tr := NewElevenLabsTranscriber("sk-test", "", "")
	if got := tr.Name(); got != "elevenlabs" {
		t.Errorf("Name() = %q, want %q", got, "elevenlabs")
	}
}

func TestDetectTranscriber(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantNil  bool
		wantName string
	}{
		{
			name:    "no config",
			cfg:     &config.Config{},
			wantNil: true,
		},
		{
			name: "elevenlabs via model list",
			cfg: &config.Config{
				ModelList: []config.ModelConfig{
					{Model: "elevenlabs/scribe_v1", APIBase: "https://api.elevenlabs.io", APIKey: "sk-11"},
				},
			},
			wantName: "elevenlabs",
		},
		{
			name: "elevenlabs takes priority over groq provider",
			cfg: &config.Config{
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
				ModelList: []config.ModelConfig{
					{Model: "elevenlabs/scribe_v1", APIBase: "https://api.elevenlabs.io", APIKey: "sk-11"},
				},
			},
			wantName: "elevenlabs",
		},
		{
			name: "groq provider key",
			cfg: &config.Config{
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
			},
			wantName: "groq",
		},
		{
			name: "groq via model list",
			cfg: &config.Config{
				ModelList: []config.ModelConfig{
					{Model: "openai/gpt-4o", APIKey: "sk-openai"},
					{Model: "groq/llama-3.3-70b", APIKey: "sk-groq-model"},
				},
			},
			wantName: "groq",
		},
		{
			name: "groq model list entry without key is skipped",
			cfg: &config.Config{
				ModelList: []config.ModelConfig{
					{Model: "groq/llama-3.3-70b", APIKey: ""},
				},
			},
			wantNil: true,
		},
		{
			name: "groq provider key takes priority over groq model list",
			cfg: &config.Config{
				Providers: config.ProvidersConfig{
					Groq: config.ProviderConfig{APIKey: "sk-groq-direct"},
				},
				ModelList: []config.ModelConfig{
					{Model: "groq/llama-3.3-70b", APIKey: "sk-groq-model"},
				},
			},
			wantName: "groq",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := DetectTranscriber(tc.cfg)
			if tc.wantNil {
				if tr != nil {
					t.Errorf("DetectTranscriber() = %v, want nil", tr)
				}
				return
			}
			if tr == nil {
				t.Fatal("DetectTranscriber() = nil, want non-nil")
			}
			if got := tr.Name(); got != tc.wantName {
				t.Errorf("Name() = %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestTranscribe(t *testing.T) {
	// Write a minimal fake audio file so the transcriber can open and send it.
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "clip.ogg")
	if err := os.WriteFile(audioPath, []byte("fake-audio-data"), 0o644); err != nil {
		t.Fatalf("failed to write fake audio file: %v", err)
	}

	t.Run("groq success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/audio/transcriptions" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer sk-test" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TranscriptionResponse{
				Text:     "hello world",
				Language: "en",
				Duration: 1.5,
			})
		}))
		defer srv.Close()

		tr := NewGroqTranscriber("sk-test")
		tr.apiBase = srv.URL

		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "hello world" {
			t.Errorf("Text = %q, want %q", resp.Text, "hello world")
		}
		if resp.Language != "en" {
			t.Errorf("Language = %q, want %q", resp.Language, "en")
		}
	})

	t.Run("elevenlabs success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/speech-to-text" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("xi-api-key") != "xi-test" {
				t.Errorf("unexpected xi-api-key header: %s", r.Header.Get("xi-api-key"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"hello from eleven","language_code":"en"}`))
		}))
		defer srv.Close()

		tr := NewElevenLabsTranscriber("xi-test", srv.URL, "scribe_v1")
		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "hello from eleven" {
			t.Errorf("Text = %q, want %q", resp.Text, "hello from eleven")
		}
		if resp.Language != "en" {
			t.Errorf("Language = %q, want %q", resp.Language, "en")
		}
	})

	t.Run("groq api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
		}))
		defer srv.Close()

		tr := NewGroqTranscriber("sk-bad")
		tr.apiBase = srv.URL

		_, err := tr.Transcribe(context.Background(), audioPath)
		if err == nil {
			t.Fatal("expected error for non-200 response, got nil")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		tr := NewGroqTranscriber("sk-test")
		_, err := tr.Transcribe(context.Background(), filepath.Join(tmpDir, "nonexistent.ogg"))
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})
}
