package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/sipeed/picoclaw/pkg/media"
)

// BrowserTool provides browser automation capabilities using chromedp.
type BrowserTool struct {
	workspace  string
	mediaStore media.MediaStore
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewBrowserTool(workspace string, store media.MediaStore) *BrowserTool {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.DisableGPU,
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, _ := chromedp.NewContext(allocCtx)

	return &BrowserTool{
		workspace:  workspace,
		mediaStore: store,
		ctx:        browserCtx,
		cancel:     cancel,
	}
}

func (t *BrowserTool) Name() string { return "browser" }
func (t *BrowserTool) Description() string {
	return "Automate a web browser to navigate, click, type, and take screenshots."
}

func (t *BrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"navigate", "click", "type", "screenshot", "get_html"},
				"description": "The action to perform.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL for 'navigate' action.",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector for 'click' or 'type' action.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text to type for 'type' action.",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Output filename for 'screenshot' action (e.g., 'view.png').",
			},
		},
		"required": []string{"action"},
	}
}

func (t *BrowserTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)

	// Ensure the browser context is still active
	if t.ctx.Err() != nil {
		allocCtx, _ := chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
		t.ctx, _ = chromedp.NewContext(allocCtx)
	}

	switch action {
	case "navigate":
		urlStr, _ := args["url"].(string)
		if urlStr == "" {
			return ErrorResult("url is required for navigate")
		}
		if err := chromedp.Run(t.ctx, chromedp.Navigate(urlStr)); err != nil {
			return ErrorResult(fmt.Sprintf("navigation failed: %v", err))
		}
		return UserResult(fmt.Sprintf("Navigated to %s", urlStr))

	case "click":
		selector, _ := args["selector"].(string)
		if selector == "" {
			return ErrorResult("selector is required for click")
		}
		if err := chromedp.Run(t.ctx, chromedp.Click(selector)); err != nil {
			return ErrorResult(fmt.Sprintf("click failed: %v", err))
		}
		return UserResult(fmt.Sprintf("Clicked element: %s", selector))

	case "type":
		selector, _ := args["selector"].(string)
		text, _ := args["text"].(string)
		if selector == "" || text == "" {
			return ErrorResult("selector and text are required for type")
		}
		if err := chromedp.Run(t.ctx, chromedp.SendKeys(selector, text)); err != nil {
			return ErrorResult(fmt.Sprintf("typing failed: %v", err))
		}
		return UserResult(fmt.Sprintf("Typed into %s", selector))

	case "screenshot":
		filename, _ := args["filename"].(string)
		if filename == "" {
			filename = fmt.Sprintf("screenshot-%d.png", time.Now().Unix())
		}
		if !strings.HasSuffix(strings.ToLower(filename), ".png") {
			filename += ".png"
		}

		path := filepath.Join(t.workspace, filename)
		var buf []byte
		if err := chromedp.Run(t.ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return ErrorResult(fmt.Sprintf("screenshot failed: %v", err))
		}

		if err := os.WriteFile(path, buf, 0o644); err != nil {
			return ErrorResult(fmt.Sprintf("failed to save screenshot: %v", err))
		}

		channel := ToolChannel(ctx)
		chatID := ToolChatID(ctx)
		scope := fmt.Sprintf("tool:browser:screenshot:%s:%s", channel, chatID)

		ref := path
		if t.mediaStore != nil {
			if r, err := t.mediaStore.Store(path, media.MediaMeta{
				Filename:    filename,
				ContentType: "image/png",
				Source:      "tool:browser",
			}, scope); err == nil {
				ref = r
			}
		}

		return MediaResult(fmt.Sprintf("Screenshot captured: %s", filename), []string{ref})

	case "get_html":
		var html string
		if err := chromedp.Run(t.ctx, chromedp.OuterHTML("html", &html)); err != nil {
			return ErrorResult(fmt.Sprintf("failed to get HTML: %v", err))
		}
		// Truncate if too long
		if len(html) > 50000 {
			html = html[:50000] + "\n... (truncated)"
		}
		return &ToolResult{ForLLM: html, ForUser: "Captured page HTML"}

	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *BrowserTool) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}
