package commands

import (
	"context"
	"fmt"
	"strings"
)

func helpCommand() Definition {
	return Definition{
		Name:        "help",
		Description: "Show this help message",
		Usage:       "/help",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			var defs []Definition
			if rt != nil && rt.ListDefinitions != nil {
				defs = rt.ListDefinitions()
			} else {
				defs = BuiltinDefinitions()
			}
			return req.Reply(formatHelpMessage(defs))
		},
	}
}

func formatHelpMessage(defs []Definition) string {
	if len(defs) == 0 {
		return "No commands available."
	}

	lines := make([]string, 0, len(defs)+8)
	for _, def := range defs {
		usage := def.EffectiveUsage()
		if usage == "" {
			usage = "/" + def.Name
		}
		desc := def.Description
		if desc == "" {
			desc = "No description"
		}
		lines = append(lines, fmt.Sprintf("%s - %s", usage, desc))
	}

	lines = append(lines, "")
	lines = append(lines, "Quick Ops:")
	lines = append(lines, "/switch openrouter - switch to OpenRouter free profile")
	lines = append(lines, "/check gws - show Google Workspace auth status")
	lines = append(lines, "/exec gws gmail +triage --max 5 --format table - list recent email via gws")
	lines = append(lines, "/exec gws drive files list --params '{\"pageSize\":10}' --format table - list recent drive files")
	lines = append(lines, "/exec gws calendar +agenda --days 3 --format table - list upcoming calendar events")

	return strings.Join(lines, "\n")
}
