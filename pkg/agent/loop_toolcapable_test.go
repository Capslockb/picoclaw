package agent

import (
	"testing"

	"github.com/sipeed/picoclaw/pkg/providers"
)

func TestPreferToolCapableCandidates_ReordersOpenRouterFreeForMediaTurns(t *testing.T) {
	candidates := []providers.FallbackCandidate{
		{Provider: "openrouter", Model: "openrouter/free"},
		{Provider: "gemini", Model: "gemini-2.5-flash"},
		{Provider: "deepseek", Model: "deepseek-chat"},
	}
	messages := []providers.Message{{Role: "user", Content: "[image: photo]", Media: []string{"media://x"}}}

	reordered, model, ok := preferToolCapableCandidates(candidates, candidates[0].Model, messages, false)
	if !ok {
		t.Fatal("expected reorder for media turn")
	}
	if reordered[0].Provider != "gemini" || reordered[0].Model != "gemini-2.5-flash" {
		t.Fatalf("first candidate = %s/%s, want gemini/gemini-2.5-flash", reordered[0].Provider, reordered[0].Model)
	}
	if model != "gemini-2.5-flash" {
		t.Fatalf("model = %q, want gemini-2.5-flash", model)
	}
}

func TestPreferToolCapableCandidates_LeavesPlainTextTurnsAlone(t *testing.T) {
	candidates := []providers.FallbackCandidate{
		{Provider: "openrouter", Model: "openrouter/free"},
		{Provider: "gemini", Model: "gemini-2.5-flash"},
	}
	messages := []providers.Message{{Role: "user", Content: "hello"}}

	if reordered, model, ok := preferToolCapableCandidates(candidates, candidates[0].Model, messages, false); ok || reordered != nil || model != "" {
		t.Fatalf("expected no reorder, got ok=%v model=%q reordered=%v", ok, model, reordered)
	}
}
