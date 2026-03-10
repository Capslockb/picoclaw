package commands

import "context"

// switchCommand is kept as a no-op stub for backward compat.
// Use /model <name> to switch models.
func switchCommand() Definition {
	return Definition{
		Name:        "switch",
		Description: "Alias: use /model <name> to switch",
		Usage:       "/switch",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply("Use /model <name> to switch models.")
		},
	}
}
