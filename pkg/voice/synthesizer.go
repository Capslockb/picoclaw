package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// Synthesizer is an interface for text-to-speech services.
type Synthesizer interface {
	Name() string
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// ElevenLabsSynthesizer implements Synthesizer using ElevenLabs API.
type ElevenLabsSynthesizer struct {
	apiKey     string
	voiceID    string
	apiBase    string
	httpClient *http.Client
}

// NewElevenLabsSynthesizer creates a new ElevenLabsSynthesizer.
func NewElevenLabsSynthesizer(apiKey, voiceID string) *ElevenLabsSynthesizer {
	if voiceID == "" {
		voiceID = "EXAVITQu4vr4xnSDxMaL" // Default: Bella
	}
	return &ElevenLabsSynthesizer{
		apiKey:  apiKey,
		voiceID: voiceID,
		apiBase: "https://api.elevenlabs.io/v1",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Synthesize converts text to speech audio data (MP3).
func (s *ElevenLabsSynthesizer) Synthesize(ctx context.Context, text string) ([]byte, error) {
	logger.InfoCF("voice", "Starting speech synthesis", map[string]any{
		"provider": "elevenlabs",
		"voice_id": s.voiceID,
		"text_len": len(text),
	})

	url := fmt.Sprintf("%s/text-to-speech/%s", s.apiBase, s.voiceID)

	requestBody, err := json.Marshal(map[string]any{
		"text":     text,
		"model_id": "eleven_monolingual_v1",
		"voice_settings": map[string]any{
			"stability":        0.5,
			"similarity_boost": 0.5,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", s.apiKey)
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.ErrorCF("voice", "ElevenLabs API error", map[string]any{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
		return nil, fmt.Errorf("ElevenLabs API error (status %d): %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.InfoCF("voice", "Speech synthesis completed successfully", map[string]any{
		"audio_size_bytes": len(audioData),
	})

	return audioData, nil
}

func (s *ElevenLabsSynthesizer) Name() string {
	return "elevenlabs"
}
