package commands

import (
	"context"
	"fmt"
	"strings"
)

func showCommand() Definition {
	return Definition{
		Name:        "show",
		Description: "Show current configuration",
		Usage:       "/show [model|channel|agents]",
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			sub := normalizeCommandName(nthToken(req.Text, 1))
			switch sub {
			case "", "all":
				modelLine := "Current Model: unavailable"
				if rt != nil && rt.GetModelInfo != nil {
					name, provider := rt.GetModelInfo()
					modelLine = fmt.Sprintf("Current Model: %s (Provider: %s)", name, provider)
				}

				channelLine := fmt.Sprintf("Current Channel: %s", req.Channel)

				agentsLine := "Registered Agents: unavailable"
				if rt != nil && rt.ListAgentIDs != nil {
					ids := rt.ListAgentIDs()
					if len(ids) == 0 {
						agentsLine = "Registered Agents: none"
					} else {
						agentsLine = fmt.Sprintf("Registered Agents: %s", strings.Join(ids, ", "))
					}
				}

				return req.Reply(strings.Join([]string{modelLine, channelLine, agentsLine}, "\n"))
			case "model":
				if rt == nil || rt.GetModelInfo == nil {
					return req.Reply(unavailableMsg)
				}
				name, provider := rt.GetModelInfo()
				return req.Reply(fmt.Sprintf("Current Model: %s (Provider: %s)", name, provider))
			case "channel":
				return req.Reply(fmt.Sprintf("Current Channel: %s", req.Channel))
			case "agents":
				return agentsHandler()(ctx, req, rt)
			default:
				return req.Reply("Usage: /show [model|channel|agents]")
			}
		},
	}
}
