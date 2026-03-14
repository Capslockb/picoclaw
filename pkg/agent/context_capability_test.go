package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSystemPromptIncludesCapabilityPrimer(t *testing.T) {
	tmpDir := t.TempDir()
	cb := NewContextBuilder(tmpDir)
	prompt := cb.BuildSystemPrompt()
	if !strings.Contains(prompt, "# Capability Use") {
		t.Fatalf("expected capability primer in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Skills are local instruction files") {
		t.Fatalf("expected skill guidance in prompt, got %q", prompt)
	}
}

func TestDetectMCPContext_NoServersConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PICOCLAW_HOME", home)
	cfgPath := filepath.Join(home, "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"tools":{"mcp":{"enabled":true,"servers":{}}}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := detectMCPContext()
	if !strings.Contains(got, "No MCP servers are configured") {
		t.Fatalf("unexpected MCP context: %q", got)
	}
}
