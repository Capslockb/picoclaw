package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/skills"
)

// InspectSkillTool allows the LLM agent to inspect a skill's details before installation.
type InspectSkillTool struct {
	registryMgr *skills.RegistryManager
}

// NewInspectSkillTool creates a new InspectSkillTool.
func NewInspectSkillTool(registryMgr *skills.RegistryManager) *InspectSkillTool {
	return &InspectSkillTool{
		registryMgr: registryMgr,
	}
}

func (t *InspectSkillTool) Name() string {
	return "inspect_skill"
}

func (t *InspectSkillTool) Description() string {
	return "Retrieve in-depth information about a skill (slug, version, files, tools, permissions, moderation status) before installation. Use this for trust and safety review."
}

func (t *InspectSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"slug": map[string]any{
				"type":        "string",
				"description": "The unique identifier of the skill to inspect (e.g., 'agentbox-openrouter')",
			},
			"registry": map[string]any{
				"type":        "string",
				"description": "Optional registry name (e.g., 'clawhub')",
			},
		},
		"required": []string{"slug"},
	}
}

func (t *InspectSkillTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	slug, ok := args["slug"].(string)
	if !ok || slug == "" {
		return ErrorResult("slug is required")
	}

	registryName, _ := args["registry"].(string)

	var reg skills.SkillRegistry
	if registryName != "" {
		reg = t.registryMgr.GetRegistry(registryName)
		if reg == nil {
			return ErrorResult(fmt.Sprintf("registry %q not found", registryName))
		}
	} else {
		// Try to find the skill in any registry (simplification: just use the first available one for now or loop)
		// Usually find_skills should provide the registry name.
		// For robustness, if not provided, we can't easily guess which registry has it without searching.
		// However, ClawHub is usually the default.
		reg = t.registryMgr.GetRegistry("clawhub")
		if reg == nil {
			return ErrorResult("no skill registries available")
		}
	}

	details, err := reg.Inspect(ctx, slug)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to inspect skill: %v", err))
	}

	return SilentResult(formatSkillDetails(details))
}

func formatSkillDetails(d *skills.SkillDetails) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Skill Inspection: %s\n\n", d.Slug))
	sb.WriteString(fmt.Sprintf("- **Name:** %s\n", d.DisplayName))
	sb.WriteString(fmt.Sprintf("- **Version:** %s\n", d.Version))
	sb.WriteString(fmt.Sprintf("- **Author:** %s\n", d.Author))
	sb.WriteString(fmt.Sprintf("- **License:** %s\n", d.License))
	sb.WriteString(fmt.Sprintf("- **Registry:** %s\n", d.RegistryName))
	
	if d.Summary != "" {
		sb.WriteString(fmt.Sprintf("\n**Summary:**\n%s\n", d.Summary))
	}
	
	if d.Description != "" {
		sb.WriteString(fmt.Sprintf("\n**Description:**\n%s\n", d.Description))
	}

	sb.WriteString("\n#### Safety & Trust:\n")
	if d.IsMalwareBlocked {
		sb.WriteString("- 🛡️ **Clean:** No known malware detected.\n")
	} else if d.IsSuspicious {
		sb.WriteString("- ⚠️ **Suspicious:** This skill has been flagged for manual review.\n")
	} else {
		sb.WriteString("- ℹ️ **Status:** Verified according to registry standards.\n")
	}

	if len(d.Permissions) > 0 {
		sb.WriteString("\n**Requested Permissions:**\n")
		for _, p := range d.Permissions {
			sb.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
	}

	if len(d.Tools) > 0 {
		sb.WriteString("\n**Tools Provided:**\n")
		for _, t := range d.Tools {
			sb.WriteString(fmt.Sprintf("- `%s`\n", t))
		}
	}

	if len(d.Files) > 0 {
		sb.WriteString("\n**Files:**\n")
		for _, f := range d.Files {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	sb.WriteString("\n*Use `install_skill` to install this skill if you trust its contents.*")
	return sb.String()
}
