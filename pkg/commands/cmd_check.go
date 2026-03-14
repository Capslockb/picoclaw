package commands

import (
	"context"
	"fmt"
	"os"
	"slices"
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
			msg += "- hints: /check channel <name> | /check gws | /check mcp"
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
					return req.Reply(fmt.Sprintf("Channel '%s' is available and enabled", value))
				},
			},
			{
				Name:        "gws",
				Description: "Check GWS CLI auth/status",
				Handler: func(ctx context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.ExecuteShell == nil {
						return req.Reply(unavailableMsg)
					}
					out, err := rt.ExecuteShell(ctx, "gws auth status 2>&1 | sed -n 1,80p")
					if err != nil {
						if strings.TrimSpace(out) == "" {
							return req.Reply("GWS check failed: " + err.Error())
						}
						return req.Reply("GWS check output:\n" + out)
					}
					if strings.TrimSpace(out) == "" {
						return req.Reply("GWS check returned no output")
					}
					return req.Reply("GWS status:\n" + out)
				},
			},
			{
				Name:        "mcp",
				Description: "Check MCP configuration status",
				Handler: func(_ context.Context, req Request, rt *Runtime) error {
					if rt == nil || rt.Config == nil {
						return req.Reply(unavailableMsg)
					}

					mcpCfg := rt.Config.Tools.MCP
					if !mcpCfg.Enabled {
						return req.Reply("MCP is disabled in config")
					}
					if len(mcpCfg.Servers) == 0 {
						return req.Reply("MCP is enabled but no servers are configured")
					}

					servers := make([]string, 0, len(mcpCfg.Servers))
					for name, server := range mcpCfg.Servers {
						state := "disabled"
						if server.Enabled {
							state = "enabled"
						}
						servers = append(servers, fmt.Sprintf("%s (%s)", name, state))
					}
					slices.Sort(servers)
					return req.Reply("MCP config:\n- servers: " + strings.Join(servers, ", "))
				},
			},
		},
	}
}
