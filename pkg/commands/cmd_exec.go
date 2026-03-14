package commands

import "context"

func execCommand() Definition {
	return Definition{
		Name:        "exec",
		Description: "Alias for /run",
		Usage:       "/exec <command>",
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			return executeShellCommand(ctx, req, rt, "/exec", "!exec")
		},
	}
}
