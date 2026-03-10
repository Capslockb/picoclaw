package commands

import (
	"context"
	"fmt"
	"strings"
)

// listCommand is kept as a no-op stub for backward compat.
// All functionality moved to /models and /channels.
func listCommand() Definition {
	return Definition{
		Name:        "list",
		Description: "Alias: use /models or /channels",
		Usage:       "/list",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply("Use /models or /channels instead.")
		},
	}
}

func modelsCommand() Definition {
	return Definition{
		Name:        "models",
		Description: "Show the currently configured model",
		Usage:       "/models",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.GetModelInfo == nil {
				return req.Reply(unavailableMsg)
			}
			name, provider := rt.GetModelInfo()
			if provider == "" {
				provider = "configured default"
			}
			return req.Reply(fmt.Sprintf("Model: %s\nProvider: %s", name, provider))
		},
	}
}

func channelsCommand() Definition {
	return Definition{
		Name:        "channels",
		Description: "List enabled channels",
		Usage:       "/channels",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.GetEnabledChannels == nil {
				return req.Reply(unavailableMsg)
			}
			enabled := rt.GetEnabledChannels()
			if len(enabled) == 0 {
				return req.Reply("No channels enabled.")
			}
			return req.Reply("Enabled channels:\n- " + strings.Join(enabled, "\n- "))
		},
	}
}
