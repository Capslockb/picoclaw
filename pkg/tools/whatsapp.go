package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// HistoryProvider matches the interface defined in pkg/channels/interfaces.go
type HistoryProvider interface {
	FetchHistory(ctx context.Context, chatID string, limit int) ([]bus.InboundMessage, error)
}

// ChannelManagerGetter is an interface to get a channel by name.
type ChannelManagerGetter interface {
	GetChannel(name string) (any, bool)
}

// WhatsAppTool allows the agent to fetch message history from WhatsApp.
type WhatsAppTool struct {
	manager ChannelManagerGetter
}

// NewWhatsAppTool creates a new WhatsAppTool.
func NewWhatsAppTool(manager ChannelManagerGetter) *WhatsAppTool {
	return &WhatsAppTool{
		manager: manager,
	}
}

func (t *WhatsAppTool) Name() string {
	return "whatsapp"
}

func (t *WhatsAppTool) Description() string {
	return "Interact with WhatsApp. Actions: 'list_messages', 'sync'. Use 'list_messages' to fetch recent chat history, and 'sync' to save the latest messages from a chat into the agent's contextual memory."
}

func (t *WhatsAppTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_messages", "sync"},
				"description": "Action to perform.",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "The WhatsApp JID or phone number (e.g., '1234567890@s.whatsapp.net' or a group JID).",
			},
			"limit": map[string]any{
				"type":        "integer",
				"default":     10,
				"description": "Number of messages to retrieve (max 50).",
			},
		},
		"required": []string{"action", "chat_id"},
	}
}

func (t *WhatsAppTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	chatID, _ := args["chat_id"].(string)
	if chatID == "" {
		return ErrorResult("chat_id is required")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	if limit > 50 {
		limit = 50
	}

	// Try to find the whatsapp_native channel
	var hp HistoryProvider
	if ch, ok := t.manager.GetChannel("whatsapp_native"); ok {
		if provider, ok := ch.(HistoryProvider); ok {
			hp = provider
		}
	}

	if hp == nil {
		return ErrorResult("WhatsApp history retrieval is not supported (requires whatsapp_native).")
	}

	messages, err := hp.FetchHistory(ctx, chatID, limit)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to fetch WhatsApp history: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("WhatsApp History for %s:\n\n", chatID))
	for _, m := range messages {
		sender := "Me"
		if m.SenderID == chatID {
			sender = "Contact"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.Timestamp.Format("2006-01-02 15:04"), sender, m.Content))
	}

	resultText := sb.String()

	if action == "sync" {
		if getter, ok := t.manager.(interface{ GetMemoryStore() any }); ok {
			if ms := getter.GetMemoryStore(); ms != nil {
				if writer, ok := ms.(interface{ AppendCommunications(string) error }); ok {
					err := writer.AppendCommunications(fmt.Sprintf("WhatsApp Sync (%s) @ %s:\n%s", chatID, time.Now().Format("2006-01-02 15:04"), resultText))
					if err != nil {
						return ErrorResult(fmt.Sprintf("Failed to sync to memory: %v", err))
					}
					return SilentResult("WhatsApp chat history synced to contextual memory.")
				}
			}
		}
		return ErrorResult("Memory store not available for sync.")
	}

	return SilentResult(resultText)
}
