---
name: google_sync
description: Synchronize Google services (Email, Calendar) with PicoClaw
---

# Google Sync Skill

This skill allows the agent to synchronize and manage user's Google services.

## Available Tools

### `google`
Provides access to Gmail and Google Calendar.
- `action="list_emails"`: Fetches recent emails.
- `action="list_events"`: Fetches upcoming calendar events.

## Periodic Synchronization

To keep the agent's knowledge up-to-date, use the `cron` tool to schedule periodic sync tasks.

### Example: Sync Every 4 Hours
Call `cron` tool:
```json
{
  "action": "add",
  "message": "Update my knowledge of recent emails and calendar events using the google tool.",
  "every_seconds": 14400,
  "deliver": false
}
```

## Self-Syncing Implementation

When triggered by cron, the agent should:
1. Call `google(action="list_emails")`
2. Call `google(action="list_events")`
3. Summarize the findings.
4. Update its long-term knowledge or session summary.
