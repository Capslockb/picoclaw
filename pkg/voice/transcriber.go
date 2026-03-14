package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
)

type Transcriber interface {
	Name() string
	Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error)
}

type OpenRouterTranscriber struct {
	apiKey     string
	apiBase    string
	model      string
	httpClient *http.Client
}

type GeminiTranscriber struct {
	apiKey     string
	apiBase    string
	model      string
	httpClient *http.Client
}

type GroqTranscriber struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
}

type ElevenLabsTranscriber struct {
	apiKey     string
	apiBase    string
	modelID    string
	httpClient *http.Client
}

type ChainTranscriber struct {
	items []Transcriber
}

type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func (c *ChainTranscriber) Name() string {
	names := make([]string, 0, len(c.items))
	for _, item := range c.items {
		if item == nil {
			continue
		}
		names = append(names, item.Name())
	}
	return strings.Join(names, "+")
}

func (c *ChainTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	var lastErr error
	for _, item := range c.items {
		if item == nil {
			continue
		}
		resp, err := item.Transcribe(ctx, audioFilePath)
		if err == nil {
			return resp, nil
		}
		lastErr = fmt.Errorf("%s: %w", item.Name(), err)
		logger.WarnCF("voice", "Transcription fallback", map[string]any{"provider": item.Name(), "error": err.Error()})
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no transcription provider configured")
	}
	return nil, lastErr
}

func NewOpenRouterTranscriber(apiKey, apiBase, model string) *OpenRouterTranscriber {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = "https://openrouter.ai/api/v1"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "openrouter/openrouter/free"
	}
	return &OpenRouterTranscriber{apiKey: strings.TrimSpace(apiKey), apiBase: base, model: model, httpClient: &http.Client{Timeout: 90 * time.Second}}
}

func (t *OpenRouterTranscriber) Name() string { return "openrouter" }

func (t *OpenRouterTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, fmt.Errorf("missing OpenRouter API key")
	}
	audio, format, _, err := readAudioInput(audioFilePath)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model": t.model,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "Transcribe this audio verbatim. Return only the spoken words with no commentary, labels, or markdown."},
				{"type": "input_audio", "input_audio": map[string]any{"data": base64.StdEncoding.EncodeToString(audio), "format": format}},
			},
		}},
		"temperature": 0,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := t.apiBase + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter stt status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	text, err := parseOpenAICompatTranscript(raw)
	if err != nil {
		return nil, err
	}
	return &TranscriptionResponse{Text: text}, nil
}

func NewGeminiTranscriber(apiKey, apiBase, model string) *GeminiTranscriber {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiTranscriber{apiKey: strings.TrimSpace(apiKey), apiBase: base, model: model, httpClient: &http.Client{Timeout: 90 * time.Second}}
}

func (t *GeminiTranscriber) Name() string { return "gemini" }

func (t *GeminiTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, fmt.Errorf("missing Gemini API key")
	}
	audio, _, mimeType, err := readAudioInput(audioFilePath)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"contents": []map[string]any{{"parts": []map[string]any{
			{"text": "Generate a verbatim transcript of the speech in this audio. Return only the spoken words with no commentary, labels, or markdown."},
			{"inlineData": map[string]any{"mimeType": mimeType, "data": base64.StdEncoding.EncodeToString(audio)}},
		}}},
		"generationConfig": map[string]any{"temperature": 0},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", t.apiBase, url.PathEscape(t.model), url.QueryEscape(t.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini stt status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini transcript: %w", err)
	}
	var text string
	for _, candidate := range parsed.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				text = strings.TrimSpace(part.Text)
				break
			}
		}
		if text != "" {
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("gemini returned empty transcription")
	}
	return &TranscriptionResponse{Text: text}, nil
}

func NewGroqTranscriber(apiKey string) *GroqTranscriber {
	logger.DebugCF("voice", "Creating Groq transcriber", map[string]any{"has_api_key": apiKey != ""})
	apiBase := "https://api.groq.com/openai/v1"
	return &GroqTranscriber{apiKey: apiKey, apiBase: apiBase, httpClient: &http.Client{Timeout: 60 * time.Second}}
}

func (t *GroqTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	logger.InfoCF("voice", "Starting transcription", map[string]any{"provider": "groq", "audio_file": audioFilePath})
	audioFile, err := os.Open(audioFilePath)
	if err != nil {
		logger.ErrorCF("voice", "Failed to open audio file", map[string]any{"path": audioFilePath, "error": err})
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()
	fileInfo, err := audioFile.Stat()
	if err != nil {
		logger.ErrorCF("voice", "Failed to get file info", map[string]any{"path": audioFilePath, "error": err})
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}
	url := t.apiBase + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	var result TranscriptionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	logger.InfoCF("voice", "Transcription completed successfully", map[string]any{"provider": "groq", "text_length": len(result.Text), "language": result.Language, "duration_seconds": result.Duration, "file_size_bytes": fileInfo.Size(), "transcription_preview": utils.Truncate(result.Text, 50)})
	return &result, nil
}

func (t *GroqTranscriber) Name() string { return "groq" }

func NewElevenLabsTranscriber(apiKey, apiBase, modelID string) *ElevenLabsTranscriber {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		base = "https://api.elevenlabs.io"
	}
	base = strings.TrimRight(base, "/")
	model := strings.TrimSpace(modelID)
	if model == "" {
		model = "scribe_v1"
	}
	logger.DebugCF("voice", "Creating ElevenLabs transcriber", map[string]any{"has_api_key": apiKey != "", "api_base": base, "model_id": model})
	return &ElevenLabsTranscriber{apiKey: apiKey, apiBase: base, modelID: model, httpClient: &http.Client{Timeout: 60 * time.Second}}
}

func (t *ElevenLabsTranscriber) Name() string { return "elevenlabs" }

func (t *ElevenLabsTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	logger.InfoCF("voice", "Starting transcription", map[string]any{"provider": "elevenlabs", "audio_file": audioFilePath})
	audioFile, err := os.Open(audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()
	fileInfo, err := audioFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.WriteField("model_id", t.modelID); err != nil {
		return nil, fmt.Errorf("failed to write model_id field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}
	url := t.apiBase + "/v1/speech-to-text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("xi-api-key", t.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Text         string `json:"text"`
		LanguageCode string `json:"language_code,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if strings.TrimSpace(raw.Text) == "" {
		return nil, fmt.Errorf("empty transcription response from ElevenLabs")
	}
	result := &TranscriptionResponse{Text: raw.Text, Language: raw.LanguageCode}
	logger.InfoCF("voice", "Transcription completed successfully", map[string]any{"provider": "elevenlabs", "text_length": len(result.Text), "language": result.Language, "file_size_bytes": fileInfo.Size(), "transcription_preview": utils.Truncate(result.Text, 50)})
	return result, nil
}

func DetectTranscriber(cfg *config.Config) Transcriber {
	if cfg == nil {
		return nil
	}
	var chain []Transcriber
	for _, mc := range cfg.ModelList {
		if strings.HasPrefix(strings.TrimSpace(mc.Model), "elevenlabs/") && strings.TrimSpace(mc.APIKey) != "" {
			chain = append(chain, NewElevenLabsTranscriber(mc.APIKey, mc.APIBase, strings.TrimPrefix(strings.TrimSpace(mc.Model), "elevenlabs/")))
			break
		}
	}
	if key := strings.TrimSpace(cfg.Providers.Gemini.APIKey); key != "" {
		chain = append(chain, NewGeminiTranscriber(key, cfg.Providers.Gemini.APIBase, os.Getenv("PICOCLAW_GEMINI_STT_MODEL")))
	}
	if key := strings.TrimSpace(cfg.Providers.Groq.APIKey); key != "" {
		chain = append(chain, NewGroqTranscriber(key))
	} else {
		for _, mc := range cfg.ModelList {
			if strings.HasPrefix(strings.TrimSpace(mc.Model), "groq/") && strings.TrimSpace(mc.APIKey) != "" {
				chain = append(chain, NewGroqTranscriber(mc.APIKey))
				break
			}
		}
	}
	if len(chain) == 0 {
		return nil
	}
	if len(chain) == 1 {
		return chain[0]
	}
	return &ChainTranscriber{items: chain}
}

func readAudioInput(path string) ([]byte, string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read audio file: %w", err)
	}
	format := audioFormatFromPath(path)
	mimeType := audioMimeTypeFromFormat(format)
	return raw, format, mimeType, nil
}

func audioFormatFromPath(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	switch ext {
	case "mp3", "wav", "ogg", "webm", "mp4", "mpeg", "mpga":
		return ext
	case "oga", "opus":
		return "ogg"
	case "m4a":
		return "mp4"
	default:
		return "wav"
	}
}

func audioMimeTypeFromFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3", "mpga", "mpeg":
		return "audio/mpeg"
	case "ogg":
		return "audio/ogg"
	case "webm":
		return "audio/webm"
	case "mp4":
		return "audio/mp4"
	default:
		return "audio/wav"
	}
}

func parseOpenAICompatTranscript(raw []byte) (string, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode openai-compatible transcript: %w", err)
	}
	for _, choice := range parsed.Choices {
		if text := strings.TrimSpace(extractContentText(choice.Message.Content)); text != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("openai-compatible provider returned empty transcription")
}

func extractContentText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
