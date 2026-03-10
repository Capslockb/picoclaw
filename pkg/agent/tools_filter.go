package agent

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// selectRelevantTools filters available tools based on user intent to reduce context.
func (al *AgentLoop) selectRelevantTools(agent *AgentInstance, userMsg string) []providers.ToolDefinition {
	allTools := agent.Tools.ToProviderDefs()
	if len(allTools) <= 5 {
		return allTools
	}

	lowerMsg := strings.ToLower(userMsg)

	// Essential tools are always included.
	essentialTools := map[string]bool{
		"read_file": true,
		"list_dir":  true,
		"google":    true, // Web search is often needed for verification
	}

	// Keyword mappings for contextual tools.
	toolKeywords := map[string][]string{
		"write_file":  {"write", "save", "create", "file", "code"},
		"edit_file":   {"edit", "modify", "change", "file", "code", "replace", "fix"},
		"append_file": {"append", "add", "log", "file"},
		"exec":        {"run", "execute", "shell", "command", "terminal", "install", "build", "cat", "ls", "git"},
		"vps":         {"vps", "server", "remote", "ssh", "cloud"},
		"voice_call":  {"call", "voice", "phone", "speak", "tell", "say"},
		"spawn":       {"spawn", "acp", "harness", "agent", "protocol"},
	}

	relevant := make([]providers.ToolDefinition, 0)
	for _, tool := range allTools {
		name := tool.Function.Name
		
		// 1. Always include essential tools.
		if essentialTools[name] {
			relevant = append(relevant, tool)
			continue
		}

		// 2. Include based on keywords.
		if kws, ok := toolKeywords[name]; ok {
			matched := false
			for _, kw := range kws {
				if strings.Contains(lowerMsg, kw) {
					matched = true
					break
				}
			}
			if matched {
				relevant = append(relevant, tool)
				continue
			}
		}

		// 3. Fallback: if tool has no keywords defined, include it by default?
		// To be safe and avoid context bloat, we only include tools with defined keywords if they match.
		// If a tool is NOT in our map, we'll include it for now to avoid breaking unknown tools.
		if _, ok := toolKeywords[name]; !ok {
			relevant = append(relevant, tool)
		}
	}

	// Safety: ensure we don't return an empty tool set if any were available.
	if len(relevant) == 0 && len(allTools) > 0 {
		return allTools[:min(len(allTools), 5)]
	}

	return relevant
}
