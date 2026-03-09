package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func checkCommand() Definition {
	return Definition{
		Name:        "check",
		Description: "Check channel availability and runtime health",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil {
				return req.Reply(unavailableMsg)
			}

			enabled := []string{}
			if rt.GetEnabledChannels != nil {
				enabled = rt.GetEnabledChannels()
			}
			shellEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("PICOCLAW_DASHBOARD_ALLOW_SHELL")), "true")

			msg := "Quick check:\n"
			if req.Channel != "" {
				msg += fmt.Sprintf("- channel: %s\n", req.Channel)
			}
			msg += fmt.Sprintf("- shell_exec: %t\n", shellEnabled)
			msg += fmt.Sprintf("- enabled_channels: %s\n", strings.Join(enabled, ", "))
			msg += "- hint: /check channel <name>"
			return req.Reply(strings.TrimSpace(msg))
		},
		SubCommands: []SubCommand{
			{
				Name:        "channel",
				Description: "Check if a channel is available",
				ArgsUsage:   "<name>",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.SwitchChannel == nil {
						return req.Reply(unavailableMsg)
					}
					value := nthToken(req.Text, 2)
					if value == "" {
						return req.Reply("Usage: /check channel <name>")
					}
					if err := rt.SwitchChannel(value); err != nil {
						return req.Reply(err.Error())
					}
					return req.Reply(fmt.Sprintf("Channel %s is available and enabled", value))
				},
			},
		},
	}
}
