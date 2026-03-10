package commands

import "context"

// checkCommand is kept as a no-op stub for backward compat.
// Use /channel <name> instead.
func checkCommand() Definition {
	return Definition{
		Name:        "check",
		Description: "Alias: use /channel <name>",
		Usage:       "/check",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply("Use /channel <name> to check channel availability.")
		},
	}
}
