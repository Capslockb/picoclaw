package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/media"
)

// SynthesizerProvider is an interface to get a speech synthesizer.
type SynthesizerProvider interface {
	GetSynthesizer(name string) (any, bool)
}

// MediaStoreGetter is an interface to get the media store.
type MediaStoreGetter interface {
	GetMediaStore() media.MediaStore
}

// WorkspaceGetter is an interface to get the agent's workspace.
type WorkspaceGetter interface {
	GetWorkspace() string
}

// SpeechTool allows the agent to generate speech audio from text.
type SpeechTool struct {
	manager any // AgentLoop/Manager
}

// NewSpeechTool creates a new SpeechTool.
func NewSpeechTool(manager any) *SpeechTool {
	return &SpeechTool{
		manager: manager,
	}
}

func (t *SpeechTool) Name() string {
	return "speech"
}

func (t *SpeechTool) Description() string {
	return "Convert text to speech audio. Action: 'speak'. Generates an MP3 file and returns a media:// reference."
}

func (t *SpeechTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"speak"},
				"description": "Action to perform.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "The text to convert to speech.",
			},
			"provider": map[string]any{
				"type":        "string",
				"description": "Optional: speech provider (e.g., 'elevenlabs'). Default is the system default.",
			},
		},
		"required": []string{"action", "text"},
	}
}

func (t *SpeechTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	text, _ := args["text"].(string)
	providerName, _ := args["provider"].(string)

	if action != "speak" {
		return ErrorResult(fmt.Sprintf("Unsupported action: %s", action))
	}
	if text == "" {
		return ErrorResult("text is required")
	}

	// 1. Get Synthesizer
	var synth any
	if getter, ok := t.manager.(SynthesizerProvider); ok {
		if s, found := getter.GetSynthesizer(providerName); found {
			synth = s
		}
	}

	if synth == nil {
		return ErrorResult("No speech synthesizer available.")
	}

	// 2. Synthesize
	type synthesizer interface {
		Synthesize(ctx context.Context, text string) ([]byte, error)
	}

	s, ok := synth.(synthesizer)
	if !ok {
		return ErrorResult("Internal error: invalid synthesizer instance.")
	}

	audioData, err := s.Synthesize(ctx, text)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Speech synthesis failed: %v", err))
	}

	// 3. Save to workspace and register in MediaStore
	var workspace string
	if wg, ok := t.manager.(WorkspaceGetter); ok {
		workspace = wg.GetWorkspace()
	}
	if workspace == "" {
		workspace = os.TempDir()
	}

	mediaDir := filepath.Join(workspace, "media")
	_ = os.MkdirAll(mediaDir, 0o755)

	filename := fmt.Sprintf("speech-%s.mp3", uuid.New().String()[:8])
	localPath := filepath.Join(mediaDir, filename)

	if err := os.WriteFile(localPath, audioData, 0o644); err != nil {
		return ErrorResult(fmt.Errorf("failed to save audio file: %w", err).Error())
	}

	// 4. Register in MediaStore
	var store media.MediaStore
	if sg, ok := t.manager.(MediaStoreGetter); ok {
		store = sg.GetMediaStore()
	}

	if store != nil {
		ref, err := store.Store(localPath, media.MediaMeta{
			Filename:    filename,
			ContentType: "audio/mpeg",
			Source:      "tool:speech",
		}, "agent_session") // TODO: pass actual scope if available
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to register media: %v", err))
		}
		return SilentResult(fmt.Sprintf("Speech generated successfully. Audio reference: %s", ref))
	}

	return SilentResult(fmt.Sprintf("Speech generated successfully. Saved to: %s", localPath))
}
