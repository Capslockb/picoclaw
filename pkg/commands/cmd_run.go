package commands

import (
	"context"
	"strings"
)

func runCommand() Definition {
	return Definition{
		Name:        "run",
		Description: "Execute a shell command",
		Usage:       "/run <command>",
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			return executeShellCommand(ctx, req, rt, "/run", "!run")
		},
	}
}

func executeShellCommand(ctx context.Context, req Request, rt *Runtime, prefixes ...string) error {
	if rt == nil || rt.ExecuteShell == nil {
		return req.Reply(unavailableMsg)
	}
	cmd := strings.TrimSpace(req.Text)
	for _, p := range prefixes {
		if strings.HasPrefix(cmd, p) {
			cmd = strings.TrimSpace(strings.TrimPrefix(cmd, p))
			break
		}
	}
	if cmd == "" {
		if len(prefixes) > 0 {
			return req.Reply("Usage: " + strings.TrimPrefix(prefixes[0], "!") + " <command>")
		}
		return req.Reply("Usage: /run <command>")
	}
	out, err := rt.ExecuteShell(ctx, cmd)
	if err != nil {
		if out != "" {
			return req.Reply("Command failed:\n" + out)
		}
		return req.Reply("Command failed: " + err.Error())
	}
	if strings.TrimSpace(out) == "" {
		return req.Reply("Done.")
	}
	return req.Reply(out)
}
