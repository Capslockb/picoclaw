package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type GoogleTool struct{}

func (t *GoogleTool) Name() string {
	return "google"
}

func (t *GoogleTool) Description() string {
	return "Access Google services like Gmail and Calendar. Actions: 'list_emails', 'list_events'. Use 'list_emails' to get recent messages (subject, snippet). Use 'list_events' to get upcoming calendar events (summary, start/end time)."
}

func (t *GoogleTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_emails", "list_events"},
				"description": "The service action to perform.",
			},
			"count": map[string]any{
				"type":        "integer",
				"default":     10,
				"description": "Number of items to retrieve.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *GoogleTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	count := 10
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	cred, err := auth.GetCredential("google-antigravity")
	if err != nil || cred == nil {
		return ErrorResult("Google account not linked. User must authenticate via 'google' provider first.")
	}

	// Automatic refresh if needed
	if cred.NeedsRefresh() {
		logger.InfoC("tools", "Refreshing Google access token")
		newCred, err := auth.RefreshAccessToken(cred, auth.GoogleAntigravityOAuthConfig())
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to refresh Google token: %v", err))
		}
		cred = newCred
		_ = auth.SetCredential("google-antigravity", cred)
	}

	switch action {
	case "list_emails":
		return t.listEmails(ctx, cred, count)
	case "list_events":
		return t.listEvents(ctx, cred, count)
	default:
		return ErrorResult("Unknown action")
	}
}

func (t *GoogleTool) listEmails(ctx context.Context, cred *auth.AuthCredential, maxResults int) *ToolResult {
	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=%d", maxResults)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("API request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("Gmail API error (%d): %s", resp.StatusCode, string(body)))
	}

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to decode Gmail list: %v", err))
	}

	var emails []string
	for _, m := range listResp.Messages {
		msgURL := "https://gmail.googleapis.com/gmail/v1/users/me/messages/" + m.ID
		mReq, _ := http.NewRequestWithContext(ctx, "GET", msgURL, nil)
		mReq.Header.Set("Authorization", "Bearer "+cred.AccessToken)
		mResp, err := http.DefaultClient.Do(mReq)
		if err != nil {
			continue
		}
		var msg struct {
			Snippet string `json:"snippet"`
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
		}
		_ = json.NewDecoder(mResp.Body).Decode(&msg)
		mResp.Body.Close()

		subject := "No Subject"
		from := "Unknown"
		for _, h := range msg.Payload.Headers {
			if h.Name == "Subject" {
				subject = h.Value
			} else if h.Name == "From" {
				from = h.Value
			}
		}
		emails = append(emails, fmt.Sprintf("- From: %s\n  Subject: %s\n  Snippet: %s", from, subject, msg.Snippet))
	}

	if len(emails) == 0 {
		return SilentResult("No messages found.")
	}

	return SilentResult(fmt.Sprintf("Recent Emails:\n%s", join(emails, "\n\n")))
}

func (t *GoogleTool) listEvents(ctx context.Context, cred *auth.AuthCredential, maxResults int) *ToolResult {
	now := time.Now().Format(time.RFC3339)
	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/primary/events?timeMin=%s&maxResults=%d&singleEvents=true&orderBy=startTime", now, maxResults)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("API request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("Calendar API error (%d): %s", resp.StatusCode, string(body)))
	}

	var eventList struct {
		Items []struct {
			Summary string `json:"summary"`
			Start   struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventList); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to decode Calendar events: %v", err))
	}

	var events []string
	for _, it := range eventList.Items {
		start := it.Start.DateTime
		if start == "" {
			start = it.Start.Date
		}
		end := it.End.DateTime
		if end == "" {
			end = it.End.Date
		}
		events = append(events, fmt.Sprintf("- Event: %s\n  Start: %s\n  End:   %s", it.Summary, start, end))
	}

	if len(events) == 0 {
		return SilentResult("No upcoming events found.")
	}

	return SilentResult(fmt.Sprintf("Upcoming Calendar Events:\n%s", join(events, "\n")))
}

func join(s []string, sep string) string {
	res := ""
	for i, v := range s {
		if i > 0 {
			res += sep
		}
		res += v
	}
	return res
}
