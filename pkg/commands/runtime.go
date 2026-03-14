package commands

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/config"
)

// Runtime provides runtime dependencies to command handlers. It is constructed
// per-request by the agent loop so that per-request state (like session scope)
// can coexist with long-lived callbacks (like GetModelInfo).
type PreviewInfo struct {
	Slug         string
	LocalURL     string
	TailscaleURL string
	Root         string
	Entry        string
}

type Runtime struct {
	Config             *config.Config
	GetModelInfo       func() (name, provider string)
	ListAgentIDs       func() []string
	ListDefinitions    func() []Definition
	GetEnabledChannels func() []string
	SwitchModel        func(value string) (oldModel string, err error)
	SwitchChannel      func(value string) error
	ExecuteShell       func(ctx context.Context, command string) (string, error)
	GetRecentPreviews  func() []PreviewInfo
	ClearHistory       func() error
	GetChannel         func(name string) (any, bool)
	GetVersion         func() string
	ListTools          func() []string
}
