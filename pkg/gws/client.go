// Package gws provides a lightweight Google Workspace REST client.
// It uses stored OAuth credentials (google-antigravity) and makes
// direct HTTP calls to Google APIs without requiring the full Google SDK.
package gws

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
)

const (
	gmailBase    = "https://gmail.googleapis.com/gmail/v1/users/me"
	driveBase    = "https://www.googleapis.com/drive/v3"
	calBase      = "https://www.googleapis.com/calendar/v3"
	docsBase     = "https://docs.googleapis.com/v1"
	sheetsBase   = "https://sheets.googleapis.com/v4"
	providerKey  = "google-antigravity"
)

// Client is a thin GWS REST client backed by a stored OAuth token.
type Client struct {
	http  *http.Client
	token string
}

// New creates a GWS client from stored credentials. Returns an error if not authenticated.
func New() (*Client, error) {
	cred, err := auth.GetCredential(providerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load Google credentials: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("not authenticated. Use /glogin first")
	}
	if cred.IsExpired() {
		// Attempt token refresh
		cfg := auth.GoogleAntigravityOAuthConfig()
		refreshed, err := auth.RefreshAccessToken(cred, cfg)
		if err != nil {
			return nil, fmt.Errorf("token expired and refresh failed: %w. Use /glogin to re-authenticate", err)
		}
		if err := auth.SetCredential(providerKey, refreshed); err != nil {
			return nil, fmt.Errorf("failed to save refreshed token: %w", err)
		}
		cred = refreshed
	}
	return &Client{
		http:  &http.Client{Timeout: 15 * time.Second},
		token: cred.AccessToken,
	}, nil
}

// get performs an authenticated GET request and decodes JSON into dest.
func (c *Client) get(rawURL string, dest any) error {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if dest != nil {
		if err := json.Unmarshal(body, dest); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return nil
}

// post performs an authenticated POST request with a JSON body.
func (c *Client) post(rawURL string, body any, dest any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", rawURL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return nil
}

// ── Gmail ────────────────────────────────────────────────────────────────────

type GmailMessage struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
	Payload struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

type GmailListResponse struct {
	Messages           []struct{ ID string `json:"id"` } `json:"messages"`
	ResultSizeEstimate int                                `json:"resultSizeEstimate"`
}

func (c *Client) GmailList(query string, maxResults int) ([]GmailMessage, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	params := url.Values{
		"maxResults": {fmt.Sprintf("%d", maxResults)},
	}
	if query != "" {
		params.Set("q", query)
	}
	var listResp GmailListResponse
	if err := c.get(gmailBase+"/messages?"+params.Encode(), &listResp); err != nil {
		return nil, err
	}

	var messages []GmailMessage
	for i, m := range listResp.Messages {
		if i >= maxResults {
			break
		}
		var msg GmailMessage
		if err := c.get(fmt.Sprintf("%s/messages/%s?format=metadata&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Date", gmailBase, m.ID), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (c *Client) GmailRead(msgID string) (*GmailMessage, error) {
	var msg GmailMessage
	if err := c.get(fmt.Sprintf("%s/messages/%s?format=full", gmailBase, msgID), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ── Drive ────────────────────────────────────────────────────────────────────

type DriveFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	ModifiedTime string `json:"modifiedTime"`
	WebViewLink  string `json:"webViewLink"`
}

type DriveListResponse struct {
	Files         []DriveFile `json:"files"`
	NextPageToken string      `json:"nextPageToken"`
}

func (c *Client) DriveList(query string, mimeFilter string, maxResults int) ([]DriveFile, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	q := "trashed=false"
	if query != "" {
		q += fmt.Sprintf(" and name contains '%s'", strings.ReplaceAll(query, "'", "\\'"))
	}
	if mimeFilter != "" {
		q += fmt.Sprintf(" and mimeType='%s'", mimeFilter)
	}
	params := url.Values{
		"q":          {q},
		"pageSize":   {fmt.Sprintf("%d", maxResults)},
		"fields":     {"files(id,name,mimeType,modifiedTime,webViewLink)"},
		"orderBy":    {"modifiedTime desc"},
	}
	var resp DriveListResponse
	if err := c.get(driveBase+"/files?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return resp.Files, nil
}

// ── Calendar ─────────────────────────────────────────────────────────────────

type CalendarEvent struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Start   struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
	} `json:"start"`
	End struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
	} `json:"end"`
	HtmlLink    string `json:"htmlLink"`
	Description string `json:"description"`
}

type CalendarListResponse struct {
	Items []CalendarEvent `json:"items"`
}

func (c *Client) CalendarList(timeMin, timeMax time.Time, maxResults int) ([]CalendarEvent, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	params := url.Values{
		"timeMin":    {timeMin.UTC().Format(time.RFC3339)},
		"timeMax":    {timeMax.UTC().Format(time.RFC3339)},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
		"singleEvents": {"true"},
		"orderBy":    {"startTime"},
	}
	var resp CalendarListResponse
	if err := c.get(calBase+"/calendars/primary/events?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ── Docs ─────────────────────────────────────────────────────────────────────

type DocsDocument struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	RevisionID string `json:"revisionId"`
}

func (c *Client) DocsCreate(title string) (*DocsDocument, error) {
	var doc DocsDocument
	if err := c.post(docsBase+"/documents", map[string]string{"title": title}, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (c *Client) DocsGet(docID string) (*DocsDocument, error) {
	var doc DocsDocument
	if err := c.get(fmt.Sprintf("%s/documents/%s", docsBase, docID), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// HeaderValue extracts a Gmail message header value by name.
func HeaderValue(msg GmailMessage, name string) string {
	for _, h := range msg.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// FormatEventTime returns a human-readable event time string.
func FormatEventTime(ev CalendarEvent) string {
	dt := ev.Start.DateTime
	if dt == "" {
		return ev.Start.Date // all-day event
	}
	t, err := time.Parse(time.RFC3339, dt)
	if err != nil {
		return dt
	}
	return t.Local().Format("Mon Jan 2, 15:04")
}

// MimeTypeLabel returns a short display label for a Drive MIME type.
func MimeTypeLabel(mime string) string {
	switch mime {
	case "application/vnd.google-apps.document":
		return "Doc"
	case "application/vnd.google-apps.spreadsheet":
		return "Sheet"
	case "application/vnd.google-apps.presentation":
		return "Slides"
	case "application/vnd.google-apps.folder":
		return "📁"
	case "application/pdf":
		return "PDF"
	default:
		if strings.HasPrefix(mime, "image/") {
			return "Image"
		}
		return "File"
	}
}
