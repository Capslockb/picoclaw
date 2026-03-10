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

type GoogleTool struct {
	manager ChannelManagerGetter
}

func NewGoogleTool(manager ChannelManagerGetter) *GoogleTool {
	return &GoogleTool{manager: manager}
}

func (t *GoogleTool) Name() string {
	return "google"
}

func (t *GoogleTool) Description() string {
	return "Access Google Workspace (GWS) services like Gmail and Calendar. Actions: 'list_emails', 'search_emails', 'read_thread', 'list_events', 'sync'. Use 'sync' with 'search_emails' or 'read_thread' to save results into the agent's contextual memory."
}

func (t *GoogleTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_emails", "search_emails", "read_thread", "list_events", "sync"},
				"description": "The service action to perform.",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Query string for 'search_emails'.",
			},
			"thread_id": map[string]any{
				"type":        "string",
				"description": "Thread ID for 'read_thread'.",
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
	case "search_emails":
		query, _ := args["query"].(string)
		return t.searchEmails(ctx, cred, query, count)
	case "read_thread":
		threadID, _ := args["thread_id"].(string)
		return t.readThread(ctx, cred, threadID)
	case "list_events":
		return t.listEvents(ctx, cred, count)
	case "sync":
		return t.syncCommunications(ctx, cred, args)
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

func (t *GoogleTool) searchEmails(ctx context.Context, cred *auth.AuthCredential, query string, maxResults int) *ToolResult {
	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?q=%s&maxResults=%d", url.QueryEscape(query), maxResults)
	return t.fetchAndFormatMessages(ctx, cred, url)
}

func (t *GoogleTool) fetchAndFormatMessages(ctx context.Context, cred *auth.AuthCredential, url string) *ToolResult {
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
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
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
		emails = append(emails, fmt.Sprintf("- From: %s\n  Subject: %s\n  Snippet: %s\n  ThreadID: %s", from, subject, msg.Snippet, m.ThreadID))
	}

	if len(emails) == 0 {
		return SilentResult("No messages found.")
	}

	return SilentResult(fmt.Sprintf("Gmail Search Results:\n%s", join(emails, "\n\n")))
}

func (t *GoogleTool) readThread(ctx context.Context, cred *auth.AuthCredential, threadID string) *ToolResult {
	url := "https://gmail.googleapis.com/gmail/v1/users/me/threads/" + threadID
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

	var threadResp struct {
		Messages []struct {
			Snippet string `json:"snippet"`
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&threadResp); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to decode Gmail thread: %v", err))
	}

	var threadMsgs []string
	for _, msg := range threadResp.Messages {
		from := "Unknown"
		date := "Unknown Date"
		for _, h := range msg.Payload.Headers {
			if h.Name == "From" {
				from = h.Value
			} else if h.Name == "Date" {
				date = h.Value
			}
		}
		threadMsgs = append(threadMsgs, fmt.Sprintf("[%s] From: %s\n%s", date, from, msg.Snippet))
	}

	return SilentResult(fmt.Sprintf("Gmail Thread %s:\n%s", threadID, join(threadMsgs, "\n\n---\n\n")))
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

func (t *GoogleTool) syncCommunications(ctx context.Context, cred *auth.AuthCredential, args map[string]any) *ToolResult {
	query, _ := args["query"].(string)
	threadID, _ := args["thread_id"].(string)
	count := 10
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	var result *ToolResult
	var syncKey string

	if threadID != "" {
		result = t.readThread(ctx, cred, threadID)
		syncKey = fmt.Sprintf("Gmail Thread (%s)", threadID)
	} else if query != "" {
		result = t.searchEmails(ctx, cred, query, count)
		syncKey = fmt.Sprintf("Gmail Search (%q)", query)
	} else {
		result = t.listEmails(ctx, cred, count)
		syncKey = "Recent Gmail"
	}

	if result.IsError {
		return result
	}

	if t.manager == nil {
		return result
	}

	if getter, ok := t.manager.(interface{ GetMemoryStore() any }); ok {
		if ms := getter.GetMemoryStore(); ms != nil {
			if writer, ok := ms.(interface{ AppendCommunications(string) error }); ok {
				err := writer.AppendCommunications(fmt.Sprintf("Gmail Sync (%s) @ %s:\n%s", syncKey, time.Now().Format("2006-01-02 15:04"), result.ForLLM))
				if err != nil {
					return ErrorResult(fmt.Sprintf("Failed to sync to memory: %v", err))
				}
				return SilentResult(fmt.Sprintf("%s synced to contextual memory.", syncKey))
			}
		}
	}

	return result
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
