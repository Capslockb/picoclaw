package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
)

// gloginCommand starts a Google OAuth flow and saves the credential.
func gloginCommand() Definition {
	return Definition{
		Name:        "glogin",
		Description: "Authenticate with Google (GWS / Cloud). Opens browser for OAuth.",
		Usage:       "/glogin [antigravity|gemini]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			provider := strings.ToLower(nthToken(req.Text, 1))

			var cfg auth.OAuthProviderConfig
			var providerKey string
			switch provider {
			case "gemini", "gcloud", "cloud":
				cfg = auth.GeminiCLIOAuthConfig()
				providerKey = "google-gemini"
			default:
				// Default: Antigravity / full GWS scopes (Gmail, Drive, Calendar, etc.)
				cfg = auth.GoogleAntigravityOAuthConfig()
				providerKey = "google-antigravity"
			}

			_ = req.Reply(fmt.Sprintf(
				"🔐 Starting Google OAuth for *%s*...\nA browser window will open. Complete sign-in, then come back here.",
				providerKey,
			))

			cred, err := auth.LoginBrowser(cfg)
			if err != nil {
				return req.Reply(fmt.Sprintf("❌ Google login failed: %v", err))
			}

			if err := auth.SetCredential(providerKey, cred); err != nil {
				return req.Reply(fmt.Sprintf("❌ Failed to save credentials: %v", err))
			}

			email := cred.Email
			if email == "" {
				email = "(email not in token)"
			}
			return req.Reply(fmt.Sprintf("✅ Google login successful!\nProvider: %s\nAccount: %s", providerKey, email))
		},
	}
}

// gstatusCommand shows the current Google auth status.
func gstatusCommand() Definition {
	return Definition{
		Name:        "gstatus",
		Description: "Show current Google authentication status",
		Usage:       "/gstatus",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			providers := []string{"google-antigravity", "google-gemini"}
			var sb strings.Builder
			sb.WriteString("🔑 *Google Auth Status*\n\n")

			anyFound := false
			for _, p := range providers {
				cred, err := auth.GetCredential(p)
				if err != nil || cred == nil {
					continue
				}
				anyFound = true

				status := "✅ Active"
				if cred.IsExpired() {
					status = "⚠️ Expired"
				} else if cred.NeedsRefresh() {
					status = "🔄 Needs refresh soon"
				}

				sb.WriteString(fmt.Sprintf("*%s*\n", p))
				sb.WriteString(fmt.Sprintf("  Status: %s\n", status))
				if cred.Email != "" {
					sb.WriteString(fmt.Sprintf("  Account: %s\n", cred.Email))
				}
				if cred.ProjectID != "" {
					sb.WriteString(fmt.Sprintf("  Project: %s\n", cred.ProjectID))
				}
				if !cred.ExpiresAt.IsZero() {
					remaining := time.Until(cred.ExpiresAt).Round(time.Minute)
					sb.WriteString(fmt.Sprintf("  Expires in: %s\n", remaining))
				}
				sb.WriteString("\n")
			}

			if !anyFound {
				sb.WriteString("No Google credentials found.\nUse /glogin to authenticate.")
			}

			return req.Reply(sb.String())
		},
	}
}

// glogoutCommand removes stored Google credentials.
func glogoutCommand() Definition {
	return Definition{
		Name:        "glogout",
		Description: "Remove stored Google credentials",
		Usage:       "/glogout [antigravity|gemini|all]",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			which := strings.ToLower(nthToken(req.Text, 1))

			switch which {
			case "gemini", "cloud":
				if err := auth.DeleteCredential("google-gemini"); err != nil {
					return req.Reply(fmt.Sprintf("❌ Failed to remove google-gemini credentials: %v", err))
				}
				return req.Reply("✅ Removed google-gemini credentials.")

			case "all":
				errs := []string{}
				for _, p := range []string{"google-antigravity", "google-gemini"} {
					if err := auth.DeleteCredential(p); err != nil {
						errs = append(errs, fmt.Sprintf("%s: %v", p, err))
					}
				}
				if len(errs) > 0 {
					return req.Reply("⚠️ Some removals failed:\n" + strings.Join(errs, "\n"))
				}
				return req.Reply("✅ All Google credentials removed.")

			default:
				// Default: antigravity
				if err := auth.DeleteCredential("google-antigravity"); err != nil {
					return req.Reply(fmt.Sprintf("❌ Failed to remove google-antigravity credentials: %v", err))
				}
				return req.Reply("✅ Removed google-antigravity credentials.")
			}
		},
	}
}

// gprojectCommand sets the active GCP project ID on the stored credential.
func gprojectCommand() Definition {
	return Definition{
		Name:        "gproject",
		Description: "Set the active GCP project ID for Google cloud operations",
		Usage:       "/gproject <project-id>",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			projectID := nthToken(req.Text, 1)
			if projectID == "" {
				// Show current
				cred, err := auth.GetCredential("google-antigravity")
				if err != nil || cred == nil {
					return req.Reply("No Google credentials found. Use /glogin first.")
				}
				if cred.ProjectID == "" {
					return req.Reply("No GCP project set. Use: /gproject <project-id>")
				}
				return req.Reply(fmt.Sprintf("Current GCP project: `%s`", cred.ProjectID))
			}

			cred, err := auth.GetCredential("google-antigravity")
			if err != nil {
				return req.Reply(fmt.Sprintf("❌ Failed to load credentials: %v", err))
			}
			if cred == nil {
				return req.Reply("No Google credentials found. Use /glogin first.")
			}

			cred.ProjectID = projectID
			if err := auth.SetCredential("google-antigravity", cred); err != nil {
				return req.Reply(fmt.Sprintf("❌ Failed to save project: %v", err))
			}

			return req.Reply(fmt.Sprintf("✅ GCP project set to: `%s`", projectID))
		},
	}
}
