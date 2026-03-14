package commands

import (
	"context"
	"fmt"
	"strings"
)

func switchCommand() Definition {
	return Definition{
		Name:        "switch",
		Description: "Switch model",
		Usage:       "/switch [model to <name>|channel]",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil {
				return req.Reply(unavailableMsg)
			}
			arg1 := normalizeCommandName(nthToken(req.Text, 1))
			if arg1 == "" {
				return req.Reply("Usage: /switch [model to <name>|channel]")
			}

			if arg1 == "channel" {
				return req.Reply("This command has moved. Please use: /check channel <name>")
			}

			if rt.SwitchModel == nil {
				return req.Reply(unavailableMsg)
			}

			var value string
			if arg1 == "model" {
				if normalizeCommandName(nthToken(req.Text, 2)) != "to" {
					return req.Reply("Usage: /switch model to <name>")
				}
				value = nthToken(req.Text, 3)
			} else {
				// Convenience form: /switch <name>
				value = nthToken(req.Text, 1)
			}

			value = normalizeSwitchModelValue(value)
			if value == "" {
				return req.Reply("Usage: /switch model to <name>")
			}
			oldModel, err := rt.SwitchModel(value)
			if err != nil {
				return req.Reply(err.Error())
			}
			return req.Reply(fmt.Sprintf("Switched model from %s to %s", oldModel, value))
		},
	}
}

func normalizeSwitchModelValue(v string) string {
	value := strings.TrimSpace(v)
	if value == "" {
		return ""
	}
	switch strings.ToLower(value) {
	case "openrouter", "openrouter/free", "free":
		return "openrouter-free"
	default:
		return value
	}
}
