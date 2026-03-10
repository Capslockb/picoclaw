package commands

import (
	"context"
	"fmt"
)

// showCommand is kept as a no-op stub for backward compat.
// All functionality moved to /model, /channel, /agents.
func showCommand() Definition {
	return Definition{
		Name:        "show",
		Description: "Alias: use /model, /channel, or /agents",
		Usage:       "/show",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply("Use /model, /channel, or /agents instead.")
		},
	}
}

func channelCommand() Definition {
	return Definition{
		Name:        "channel",
		Description: "Show current channel or check a named channel",
		Usage:       "/channel [name]",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			name := nthToken(req.Text, 1)

			// No arg: show current channel
			if name == "" {
				return req.Reply(fmt.Sprintf("Current channel: %s", req.Channel))
			}

			// With arg: check/switch channel
			if rt == nil || rt.SwitchChannel == nil {
				return req.Reply(unavailableMsg)
			}
			if err := rt.SwitchChannel(name); err != nil {
				return req.Reply(fmt.Sprintf("Channel '%s' is not available: %v", name, err))
			}
			return req.Reply(fmt.Sprintf("Channel '%s' is available and enabled.", name))
		},
	}
}

func agentsCommand() Definition {
	return Definition{
		Name:        "agents",
		Description: "List registered agents",
		Usage:       "/agents",
		Handler:     agentsHandler(),
	}
}
