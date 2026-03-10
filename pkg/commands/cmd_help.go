package commands

import (
	"context"
	"fmt"
	"sort"
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
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	var sb strings.Builder
	sb.WriteString("🦞 *PicoClaw Commands*\n\n")

	for _, d := range defs {
		// Skip legacy stub commands
		if strings.HasPrefix(d.Description, "Alias:") {
			continue
		}

		usage := d.Usage
		if usage == "" {
			usage = "/" + d.Name
		}
		sb.WriteString(fmt.Sprintf("• `%s` — %s\n", usage, d.Description))
	}

	sb.WriteString("\n• `<shell command>` — Run a shell command (if exec tool enabled)\n")
	sb.WriteString("\nTip: All commands are single-word, e.g. `/model gpt-4o`")
	return sb.String()
}
