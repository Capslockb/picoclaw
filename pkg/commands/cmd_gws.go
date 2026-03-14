package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/gws"
)

// ── /gmail ───────────────────────────────────────────────────────────────────

func gmailCommand() Definition {
	return Definition{
		Name:        "gmail",
		Description: "Gmail: list, search, or read emails",
		Usage:       "/gmail [list|search <query>|read <id>|help]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			action := strings.ToLower(nthToken(req.Text, 1))

			if action == "" || action == "help" {
				return req.Reply(
					"📧 *Gmail Commands*\n\n" +
						"• `/gmail list` — last 10 inbox messages\n" +
						"• `/gmail search <query>` — search by subject/sender/content\n" +
						"• `/gmail unread` — unread messages only\n" +
						"• `/gmail read <message-id>` — open a specific message\n",
				)
			}

			c, err := gws.New()
			if err != nil {
				return req.Reply("❌ " + err.Error())
			}

			switch action {
			case "list":
				msgs, err := c.GmailList("in:inbox", 10)
				if err != nil {
					return req.Reply("❌ Gmail list failed: " + err.Error())
				}
				return req.Reply(formatGmailList(msgs, "Inbox"))

			case "unread":
				msgs, err := c.GmailList("is:unread in:inbox", 10)
				if err != nil {
					return req.Reply("❌ Gmail unread failed: " + err.Error())
				}
				return req.Reply(formatGmailList(msgs, "Unread"))

			case "search":
				query := strings.Join(tailTokens(req.Text, 2), " ")
				if query == "" {
					return req.Reply("Usage: /gmail search <query>")
				}
				msgs, err := c.GmailList(query, 10)
				if err != nil {
					return req.Reply("❌ Gmail search failed: " + err.Error())
				}
				return req.Reply(formatGmailList(msgs, "Search: "+query))

			case "read":
				msgID := nthToken(req.Text, 2)
				if msgID == "" {
					return req.Reply("Usage: /gmail read <message-id>")
				}
				msg, err := c.GmailRead(msgID)
				if err != nil {
					return req.Reply("❌ Gmail read failed: " + err.Error())
				}
				from := gws.HeaderValue(*msg, "From")
				subject := gws.HeaderValue(*msg, "Subject")
				date := gws.HeaderValue(*msg, "Date")
				return req.Reply(fmt.Sprintf(
					"📧 *%s*\nFrom: %s\nDate: %s\n\n%s",
					subject, from, date, truncateStr(msg.Snippet, 500),
				))

			default:
				return req.Reply(fmt.Sprintf("Unknown gmail action: %s\nUse /gmail help.", action))
			}
		},
	}
}

func formatGmailList(msgs []gws.GmailMessage, title string) string {
	if len(msgs) == 0 {
		return fmt.Sprintf("📧 *%s*\n\nNo messages found.", title)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📧 *%s* (%d)\n\n", title, len(msgs)))
	for _, m := range msgs {
		subject := gws.HeaderValue(m, "Subject")
		from := gws.HeaderValue(m, "From")
		if subject == "" {
			subject = "(no subject)"
		}
		sb.WriteString(fmt.Sprintf("• [%s] %s\n  `%s`\n", from, subject, m.ID))
	}
	return sb.String()
}

// ── /drive ───────────────────────────────────────────────────────────────────

func driveCommand() Definition {
	return Definition{
		Name:        "drive",
		Description: "Google Drive: list or search files",
		Usage:       "/drive [list|search <query>|docs|sheets|help]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			action := strings.ToLower(nthToken(req.Text, 1))

			if action == "" || action == "help" {
				return req.Reply(
					"💾 *Drive Commands*\n\n" +
						"• `/drive list` — recent files\n" +
						"• `/drive search <query>` — search by name\n" +
						"• `/drive docs` — recent Google Docs\n" +
						"• `/drive sheets` — recent Sheets\n",
				)
			}

			c, err := gws.New()
			if err != nil {
				return req.Reply("❌ " + err.Error())
			}

			switch action {
			case "list":
				files, err := c.DriveList("", "", 10)
				if err != nil {
					return req.Reply("❌ Drive list failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Recent Files"))

			case "search":
				query := strings.Join(tailTokens(req.Text, 2), " ")
				if query == "" {
					return req.Reply("Usage: /drive search <query>")
				}
				files, err := c.DriveList(query, "", 10)
				if err != nil {
					return req.Reply("❌ Drive search failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Search: "+query))

			case "docs":
				files, err := c.DriveList("", "application/vnd.google-apps.document", 10)
				if err != nil {
					return req.Reply("❌ Drive docs failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Recent Docs"))

			case "sheets":
				files, err := c.DriveList("", "application/vnd.google-apps.spreadsheet", 10)
				if err != nil {
					return req.Reply("❌ Drive sheets failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Recent Sheets"))

			default:
				return req.Reply(fmt.Sprintf("Unknown drive action: %s\nUse /drive help.", action))
			}
		},
	}
}

func formatDriveList(files []gws.DriveFile, title string) string {
	if len(files) == 0 {
		return fmt.Sprintf("💾 *%s*\n\nNo files found.", title)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💾 *%s* (%d)\n\n", title, len(files)))
	for _, f := range files {
		label := gws.MimeTypeLabel(f.MimeType)
		sb.WriteString(fmt.Sprintf("• [%s] %s\n  `%s`\n", label, f.Name, f.ID))
	}
	return sb.String()
}

// ── /docs ────────────────────────────────────────────────────────────────────

func docsCommand() Definition {
	return Definition{
		Name:        "docs",
		Description: "Google Docs: create or open a document",
		Usage:       "/docs [create <title>|open <id>|help]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			action := strings.ToLower(nthToken(req.Text, 1))

			if action == "" || action == "help" {
				return req.Reply(
					"📄 *Docs Commands*\n\n" +
						"• `/docs create <title>` — create a new Google Doc\n" +
						"• `/docs open <document-id>` — get document info\n" +
						"• `/docs list` — recent docs (via Drive)\n",
				)
			}

			c, err := gws.New()
			if err != nil {
				return req.Reply("❌ " + err.Error())
			}

			switch action {
			case "create":
				title := strings.Join(tailTokens(req.Text, 2), " ")
				if title == "" {
					title = fmt.Sprintf("Document %s", time.Now().Format("2006-01-02"))
				}
				doc, err := c.DocsCreate(title)
				if err != nil {
					return req.Reply("❌ Docs create failed: " + err.Error())
				}
				return req.Reply(fmt.Sprintf(
					"📄 *Document created!*\nTitle: %s\nID: `%s`\nOpen: https://docs.google.com/document/d/%s/edit",
					doc.Title, doc.DocumentID, doc.DocumentID,
				))

			case "open":
				docID := nthToken(req.Text, 2)
				if docID == "" {
					return req.Reply("Usage: /docs open <document-id>")
				}
				doc, err := c.DocsGet(docID)
				if err != nil {
					return req.Reply("❌ Docs open failed: " + err.Error())
				}
				return req.Reply(fmt.Sprintf(
					"📄 *%s*\nID: `%s`\nRevision: %s\nOpen: https://docs.google.com/document/d/%s/edit",
					doc.Title, doc.DocumentID, doc.RevisionID, doc.DocumentID,
				))

			case "list":
				files, err := c.DriveList("", "application/vnd.google-apps.document", 10)
				if err != nil {
					return req.Reply("❌ Docs list failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Recent Docs"))

			default:
				return req.Reply(fmt.Sprintf("Unknown docs action: %s\nUse /docs help.", action))
			}
		},
	}
}

// ── /cal ─────────────────────────────────────────────────────────────────────

func calCommand() Definition {
	return Definition{
		Name:        "cal",
		Description: "Google Calendar: list upcoming events",
		Usage:       "/cal [today|week|month|help]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			action := strings.ToLower(nthToken(req.Text, 1))

			if action == "help" {
				return req.Reply(
					"📅 *Calendar Commands*\n\n" +
						"• `/cal` or `/cal today` — events today\n" +
						"• `/cal week` — next 7 days\n" +
						"• `/cal month` — next 30 days\n",
				)
			}

			c, err := gws.New()
			if err != nil {
				return req.Reply("❌ " + err.Error())
			}

			now := time.Now()
			var timeMax time.Time
			var label string

			switch action {
			case "week":
				timeMax = now.Add(7 * 24 * time.Hour)
				label = "Next 7 Days"
			case "month":
				timeMax = now.Add(30 * 24 * time.Hour)
				label = "Next 30 Days"
			default: // today or empty
				timeMax = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
				label = "Today"
			}

			events, err := c.CalendarList(now, timeMax, 15)
			if err != nil {
				return req.Reply("❌ Calendar failed: " + err.Error())
			}

			if len(events) == 0 {
				return req.Reply(fmt.Sprintf("📅 *%s* — No events.", label))
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📅 *%s* (%d events)\n\n", label, len(events)))
			for _, ev := range events {
				t := gws.FormatEventTime(ev)
				sb.WriteString(fmt.Sprintf("• %s — %s\n", t, ev.Summary))
			}
			return req.Reply(sb.String())
		},
	}
}

// ── /sheets ──────────────────────────────────────────────────────────────────

func sheetsCommand() Definition {
	return Definition{
		Name:        "sheets",
		Description: "Google Sheets: list or search spreadsheets",
		Usage:       "/sheets [list|search <query>|help]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			action := strings.ToLower(nthToken(req.Text, 1))

			if action == "help" {
				return req.Reply(
					"📊 *Sheets Commands*\n\n" +
						"• `/sheets list` — recent spreadsheets\n" +
						"• `/sheets search <query>` — search by name\n",
				)
			}

			c, err := gws.New()
			if err != nil {
				return req.Reply("❌ " + err.Error())
			}

			const sheetMime = "application/vnd.google-apps.spreadsheet"
			switch action {
			case "search":
				query := strings.Join(tailTokens(req.Text, 2), " ")
				if query == "" {
					return req.Reply("Usage: /sheets search <query>")
				}
				files, err := c.DriveList(query, sheetMime, 10)
				if err != nil {
					return req.Reply("❌ Sheets search failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Sheets: "+query))

			default: // list
				files, err := c.DriveList("", sheetMime, 10)
				if err != nil {
					return req.Reply("❌ Sheets list failed: " + err.Error())
				}
				return req.Reply(formatDriveList(files, "Recent Sheets"))
			}
		},
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// tailTokens returns all tokens starting at index n (0-indexed), joined as-is.
func tailTokens(text string, n int) []string {
	parts := strings.Fields(strings.TrimSpace(text))
	if n >= len(parts) {
		return nil
	}
	return parts[n:]
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
