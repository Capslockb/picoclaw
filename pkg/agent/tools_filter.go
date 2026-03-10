package agent

import (
	"sort"
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

	type ScoredTool struct {
		tool  providers.ToolDefinition
		score int
	}

	scored := make([]ScoredTool, 0, len(allTools))
	for _, tool := range allTools {
		name := tool.Function.Name
		score := 0

		// 1. Always include essential tools (high score).
		if essentialTools[name] {
			score += 100
		}

		// 2. Score based on keywords.
		if kws, ok := toolKeywords[name]; ok {
			for _, kw := range kws {
				if strings.Contains(lowerMsg, kw) {
					score += 10
				}
			}
		}

		// 3. Fallback: if tool has no keywords defined, give it a base score to avoid starving unknown tools.
		if _, ok := toolKeywords[name]; !ok {
			score += 5
		}

		if score > 0 {
			scored = append(scored, ScoredTool{tool: tool, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take Top-K (up to 12 tools for a good balance of capability and context)
	topK := 12
	relevant := make([]providers.ToolDefinition, 0, min(len(scored), topK))
	for i := 0; i < min(len(scored), topK); i++ {
		relevant = append(relevant, scored[i].tool)
	}

	return relevant
}
