package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const reportlabCheckScript = "import reportlab"

const writePDFScript = `
import os
import sys
import textwrap
from reportlab.lib.pagesizes import A4
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas

path, title, content = sys.argv[1], sys.argv[2], sys.argv[3]
os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
font_name = "Helvetica"
font_path = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
if os.path.exists(font_path):
    pdfmetrics.registerFont(TTFont("DejaVuSans", font_path))
    font_name = "DejaVuSans"

page_width, page_height = A4
left = 48
right = 48
usable_width = page_width - left - right
line_height = 15
c = canvas.Canvas(path, pagesize=A4)
y = page_height - 52

if title.strip():
    c.setFont(font_name, 16)
    c.drawString(left, y, title)
    y -= 28

c.setFont(font_name, 11)
max_chars = max(30, int(usable_width / 6.2))
for para in content.splitlines():
    lines = textwrap.wrap(para, width=max_chars, replace_whitespace=False, drop_whitespace=False) or [""]
    for line in lines:
        if y < 60:
            c.showPage()
            c.setFont(font_name, 11)
            y = page_height - 52
        c.drawString(left, y, line.rstrip())
        y -= line_height
    y -= 4

c.save()
print(path)
`

type WritePDFTool struct {
	workspace string
	restrict  bool
}

func NewWritePDFTool(workspace string, restrict bool) *WritePDFTool {
	return &WritePDFTool{workspace: workspace, restrict: restrict}
}

func (t *WritePDFTool) Name() string {
	return "write_pdf"
}

func (t *WritePDFTool) Description() string {
	return "Create a simple PDF file from text content. Use for exports after translating or formatting content."
}

func (t *WritePDFTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Destination PDF path. Relative paths are resolved from the workspace.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional document title rendered at the top of the PDF.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Document body text to place into the PDF.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WritePDFTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	pathArg, _ := args["path"].(string)
	if strings.TrimSpace(pathArg) == "" {
		return ErrorResult("path is required")
	}
	content, _ := args["content"].(string)
	if strings.TrimSpace(content) == "" {
		return ErrorResult("content is required")
	}
	title, _ := args["title"].(string)

	resolved, err := validatePath(pathArg, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(fmt.Sprintf("invalid path: %v", err))
	}
	if filepath.Ext(strings.ToLower(resolved)) != ".pdf" {
		resolved += ".pdf"
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return ErrorResult(fmt.Sprintf("create directory: %v", err))
	}

	if _, err := exec.LookPath("python3"); err != nil {
		return ErrorResult("python3 is required to generate PDFs")
	}
	check := exec.CommandContext(ctx, "python3", "-c", reportlabCheckScript)
	if out, err := check.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			msg = ": " + msg
		}
		return ErrorResult("reportlab is not installed; install python3-reportlab and retry" + msg)
	}

	cmd := exec.CommandContext(ctx, "python3", "-c", writePDFScript, resolved, title, content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return ErrorResult(fmt.Sprintf("write_pdf failed: %v: %s", err, strings.TrimSpace(string(out))))
	}

	return NewToolResult(fmt.Sprintf("PDF written to %s", resolved))
}
