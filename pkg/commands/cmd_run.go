package commands

import (
	"context"
	"strings"
)

func runCommand() Definition {
	return Definition{
		Name:        "run",
		Description: "Execute a shell command",
		Usage:   "<command>",
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.ExecuteShell == nil {
				return req.Reply(unavailableMsg)
			}
			parts := strings.Fields(strings.TrimSpace(req.Text))
			if len(parts) < 2 {
				return req.Reply("Usage: /run <command>")
			}
			cmd := strings.TrimSpace(req.Text)
			if strings.HasPrefix(cmd, "/run") {
				cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "/run"))
			} else if strings.HasPrefix(cmd, "!run") {
				cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "!run"))
			}
			if cmd == "" {
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
		},
	}
}
