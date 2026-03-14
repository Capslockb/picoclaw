package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

var _ Transcriber = (*OpenRouterTranscriber)(nil)
var _ Transcriber = (*GeminiTranscriber)(nil)
var _ Transcriber = (*GroqTranscriber)(nil)
var _ Transcriber = (*ElevenLabsTranscriber)(nil)
var _ Transcriber = (*ChainTranscriber)(nil)

func TestOpenRouterTranscriberName(t *testing.T) {
	tr := NewOpenRouterTranscriber("sk-test", "", "")
	if got := tr.Name(); got != "openrouter" {
		t.Errorf("Name() = %q, want %q", got, "openrouter")
	}
}

func TestGeminiTranscriberName(t *testing.T) {
	tr := NewGeminiTranscriber("gem-test", "", "")
	if got := tr.Name(); got != "gemini" {
		t.Errorf("Name() = %q, want %q", got, "gemini")
	}
}

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
		{name: "no config", cfg: &config.Config{}, wantNil: true},
		{
			name:     "elevenlabs first",
			cfg:      &config.Config{ModelList: []config.ModelConfig{{Model: "elevenlabs/scribe_v1", APIBase: "https://api.elevenlabs.io", APIKey: "xi-key"}}},
			wantName: "elevenlabs",
		},
		{
			name:     "elevenlabs then gemini chain",
			cfg:      &config.Config{Providers: config.ProvidersConfig{Gemini: config.ProviderConfig{APIKey: "gem-key"}}, ModelList: []config.ModelConfig{{Model: "elevenlabs/scribe_v1", APIBase: "https://api.elevenlabs.io", APIKey: "xi-key"}}},
			wantName: "elevenlabs+gemini",
		},
		{
			name:     "gemini ahead of groq",
			cfg:      &config.Config{Providers: config.ProvidersConfig{Gemini: config.ProviderConfig{APIKey: "gem-key"}, Groq: config.ProviderConfig{APIKey: "groq-key"}}},
			wantName: "gemini+groq",
		},
		{
			name:     "groq via provider key",
			cfg:      &config.Config{Providers: config.ProvidersConfig{Groq: config.ProviderConfig{APIKey: "groq-key"}}},
			wantName: "groq",
		},
		{
			name:     "groq via model list",
			cfg:      &config.Config{ModelList: []config.ModelConfig{{Model: "groq/whisper-large-v3", APIKey: "sk-groq-model"}}},
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
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "clip.wav")
	if err := os.WriteFile(audioPath, []byte("fake-audio-data"), 0o644); err != nil {
		t.Fatalf("failed to write fake audio file: %v", err)
	}

	t.Run("openrouter success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/chat/completions" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer sk-or" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "input_audio") {
				t.Errorf("request body missing input_audio: %s", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello from openrouter"}}]}`))
		}))
		defer srv.Close()

		tr := NewOpenRouterTranscriber("sk-or", srv.URL, "openrouter/openrouter/free")
		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "hello from openrouter" {
			t.Errorf("Text = %q, want %q", resp.Text, "hello from openrouter")
		}
	})

	t.Run("gemini success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/models/gemini-2.5-flash:generateContent") {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("key"); got != "gem-key" {
				t.Errorf("unexpected key query: %s", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello from gemini"}]}}]}`))
		}))
		defer srv.Close()

		tr := NewGeminiTranscriber("gem-key", srv.URL, "gemini-2.5-flash")
		resp, err := tr.Transcribe(context.Background(), audioPath)
		if err != nil {
			t.Fatalf("Transcribe() error: %v", err)
		}
		if resp.Text != "hello from gemini" {
			t.Errorf("Text = %q, want %q", resp.Text, "hello from gemini")
		}
	})

	t.Run("groq success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/audio/transcriptions" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer sk-test" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TranscriptionResponse{Text: "hello world", Language: "en", Duration: 1.5})
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

	t.Run("missing file", func(t *testing.T) {
		tr := NewOpenRouterTranscriber("sk-or", "", "")
		_, err := tr.Transcribe(context.Background(), filepath.Join(tmpDir, "nonexistent.ogg"))
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})
}
