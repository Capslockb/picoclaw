// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type Synthesizer interface {
	Name() string
	Synthesize(ctx context.Context, text string) (*SynthesisResponse, error)
}

type SynthesisResponse struct {
	AudioFilePath string
	ContentType   string
	Filename      string
}

type GeminiSynthesizer struct {
	apiKey     string
	apiBase    string
	model      string
	voiceName  string
	httpClient *http.Client
}

type ElevenLabsSynthesizer struct {
	apiKey     string
	apiBase    string
	modelID    string
	voiceID    string
	httpClient *http.Client
}

type ChainSynthesizer struct {
	items []Synthesizer
}

func (c *ChainSynthesizer) Name() string {
	names := make([]string, 0, len(c.items))
	for _, item := range c.items {
		if item == nil {
			continue
		}
		names = append(names, item.Name())
	}
	return strings.Join(names, "+")
}

func (c *ChainSynthesizer) Synthesize(ctx context.Context, text string) (*SynthesisResponse, error) {
	var lastErr error
	for _, item := range c.items {
		if item == nil {
			continue
		}
		resp, err := item.Synthesize(ctx, text)
		if err == nil {
			return resp, nil
		}
		lastErr = fmt.Errorf("%s: %w", item.Name(), err)
		logger.WarnCF("voice", "Speech synthesis fallback", map[string]any{"provider": item.Name(), "error": err.Error()})
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no speech synthesizer configured")
	}
	return nil, lastErr
}

func NewGeminiSynthesizer(apiKey, apiBase, model, voiceName string) *GeminiSynthesizer {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	if strings.TrimSpace(model) == "" {
		model = "gemini-2.5-flash-preview-tts"
	}
	if strings.TrimSpace(voiceName) == "" {
		voiceName = "Kore"
	}
	return &GeminiSynthesizer{apiKey: strings.TrimSpace(apiKey), apiBase: base, model: strings.TrimSpace(model), voiceName: strings.TrimSpace(voiceName), httpClient: &http.Client{Timeout: 90 * time.Second}}
}

func (g *GeminiSynthesizer) Name() string { return "gemini" }

func (g *GeminiSynthesizer) Synthesize(ctx context.Context, text string) (*SynthesisResponse, error) {
	if strings.TrimSpace(g.apiKey) == "" {
		return nil, fmt.Errorf("missing Gemini API key")
	}
	payload := map[string]any{
		"contents": []map[string]any{{"parts": []map[string]any{{"text": "Convert the following transcript to speech only. Output audio only and read it exactly as written: " + text}}}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig":       map[string]any{"voiceConfig": map[string]any{"prebuiltVoiceConfig": map[string]any{"voiceName": g.voiceName}}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiBase, url.PathEscape(g.model), url.QueryEscape(g.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini tts status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini tts response: %w", err)
	}
	var mimeType, data string
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.InlineData.Data) != "" {
				data = part.InlineData.Data
				mimeType = strings.TrimSpace(part.InlineData.MimeType)
				break
			}
		}
		if data != "" {
			break
		}
	}
	if data == "" {
		return nil, fmt.Errorf("gemini tts returned no audio data")
	}
	audio, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("decode gemini audio: %w", err)
	}
	return encodeSpeechToTelegramVoice(ctx, audio, mimeType)
}

func NewElevenLabsSynthesizer(apiKey, apiBase, modelID, voiceID string) *ElevenLabsSynthesizer {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = "https://api.elevenlabs.io"
	}
	if strings.TrimSpace(modelID) == "" {
		modelID = "eleven_multilingual_v2"
	}
	if strings.TrimSpace(voiceID) == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM"
	}
	return &ElevenLabsSynthesizer{apiKey: strings.TrimSpace(apiKey), apiBase: base, modelID: strings.TrimSpace(modelID), voiceID: strings.TrimSpace(voiceID), httpClient: &http.Client{Timeout: 90 * time.Second}}
}

func (e *ElevenLabsSynthesizer) Name() string { return "elevenlabs" }

func (e *ElevenLabsSynthesizer) Synthesize(ctx context.Context, text string) (*SynthesisResponse, error) {
	if strings.TrimSpace(e.apiKey) == "" {
		return nil, fmt.Errorf("missing ElevenLabs API key")
	}
	body, err := json.Marshal(map[string]any{"text": text, "model_id": e.modelID, "output_format": "mp3_44100_128"})
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/v1/text-to-speech/%s", e.apiBase, url.PathEscape(e.voiceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", e.apiKey)
	req.Header.Set("Accept", "audio/mpeg")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elevenlabs tts status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return encodeSpeechToTelegramVoice(ctx, raw, "audio/mpeg")
}

func encodeSpeechToTelegramVoice(ctx context.Context, raw []byte, mimeType string) (*SynthesisResponse, error) {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	if mime == "" {
		mime = "audio/L16;rate=24000"
	}
	tmpDir, err := os.MkdirTemp("", "picoclaw-tts-")
	if err != nil {
		return nil, err
	}
	inputPath := filepath.Join(tmpDir, "input.bin")
	outputPath := filepath.Join(tmpDir, "reply.ogg")
	if strings.Contains(mime, "mpeg") || strings.Contains(mime, "mp3") {
		inputPath = filepath.Join(tmpDir, "input.mp3")
	} else if strings.Contains(mime, "wav") || strings.Contains(mime, "wave") {
		inputPath = filepath.Join(tmpDir, "input.wav")
	} else if strings.Contains(mime, "ogg") || strings.Contains(mime, "opus") {
		inputPath = filepath.Join(tmpDir, "input.ogg")
	} else if strings.Contains(mime, "pcm") || strings.Contains(mime, "l16") {
		inputPath = filepath.Join(tmpDir, "input.pcm")
	}
	if err := os.WriteFile(inputPath, raw, 0o600); err != nil {
		return nil, err
	}
	args := []string{"-y"}
	if strings.HasSuffix(inputPath, ".pcm") {
		args = append(args, "-f", "s16le", "-ar", "24000", "-ac", "1")
	}
	args = append(args, "-i", inputPath, "-c:a", "libopus", "-b:a", "24k", outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg voice transcode failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return &SynthesisResponse{AudioFilePath: outputPath, ContentType: "audio/ogg", Filename: "reply.ogg"}, nil
}

func DetectSynthesizer(cfg *config.Config) Synthesizer {
	if cfg == nil {
		return nil
	}
	var chain []Synthesizer
	for _, mc := range cfg.ModelList {
		if strings.HasPrefix(strings.TrimSpace(mc.Model), "elevenlabs/") && strings.TrimSpace(mc.APIKey) != "" {
			chain = append(chain, NewElevenLabsSynthesizer(mc.APIKey, mc.APIBase, strings.TrimPrefix(strings.TrimSpace(mc.Model), "elevenlabs/"), os.Getenv("PICOCLAW_ELEVENLABS_VOICE_ID")))
			break
		}
	}
	if key := strings.TrimSpace(cfg.Providers.Gemini.APIKey); key != "" {
		chain = append(chain, NewGeminiSynthesizer(key, cfg.Providers.Gemini.APIBase, os.Getenv("PICOCLAW_GEMINI_TTS_MODEL"), os.Getenv("PICOCLAW_GEMINI_TTS_VOICE")))
	}
	if len(chain) == 0 {
		return nil
	}
	if len(chain) == 1 {
		return chain[0]
	}
	return &ChainSynthesizer{items: chain}
}
