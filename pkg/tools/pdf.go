package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf/v2"
	"github.com/sipeed/picoclaw/pkg/media"
)

// PDFTool provides capabilities for creating, translating, and extracting PDF documents.
type PDFTool struct {
	workspace  string
	restrict   bool
	mediaStore media.MediaStore
}

func NewPDFTool(workspace string, restrict bool, store media.MediaStore) *PDFTool {
	return &PDFTool{
		workspace:  workspace,
		restrict:   restrict,
		mediaStore: store,
	}
}

func (t *PDFTool) Name() string { return "pdf" }
func (t *PDFTool) Description() string {
	return "Create and export PDF documents. Use 'create' to generate a PDF from text (e.g. translations). " +
		"For translation tasks: translate the full text first, then call pdf with action='create', content=<full translation>, output=<filename.pdf>."
}

func (t *PDFTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"create", "extract"},
				"description": "Action: 'create' builds a PDF from text content, 'extract' reads a PDF file path and returns its text.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Text content to write into the PDF (for 'create'). Pass the COMPLETE translated text — never truncate.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title displayed at the top of the PDF (for 'create').",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File path or media:// ref of a PDF to extract text from (for 'extract').",
			},
			"output": map[string]any{
				"type":        "string",
				"description": "Output filename for 'create' (e.g. 'translation_ro.pdf'). Defaults to 'document.pdf'.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *PDFTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)

	switch action {
	case "create":
		return t.handleCreate(ctx, args)
	case "extract":
		return t.handleExtract(args)
	default:
		return ErrorResult(fmt.Sprintf("unknown pdf action: %s. Use 'create' or 'extract'.", action))
	}
}

// handleCreate builds a properly paginated, Unicode-safe PDF from the given content.
func (t *PDFTool) handleCreate(_ context.Context, args map[string]any) *ToolResult {
	content, _ := args["content"].(string)
	title, _ := args["title"].(string)
	output, _ := args["output"].(string)

	if content == "" {
		return ErrorResult("content is required for 'create' action")
	}

	if output == "" {
		output = "document.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(output), ".pdf") {
		output += ".pdf"
	}

	outPath := filepath.Join(t.workspace, output)
	if t.restrict {
		var err error
		outPath, err = validatePath(output, t.workspace, true)
		if err != nil {
			return ErrorResult(err.Error())
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create output directory: %v", err))
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)

	// Use ISO-8859-2 encoder for full coverage of EU Latin languages (Romanian, etc.)
	pdf.SetFont("Helvetica", "", 12)

	tr := pdf.UnicodeTranslatorFromDescriptor("iso-8859-2")

	pdf.AddPage()

	// Optional title
	if title != "" {
		pdf.SetFont("Helvetica", "B", 16)
		pdf.MultiCell(170, 10, tr(title), "", "C", false)
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "", 12)
	}

	// Write body — split on blank lines to preserve paragraph structure
	paragraphs := strings.Split(content, "\n\n")
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			pdf.Ln(4)
			continue
		}
		// Within a paragraph, keep newlines as line breaks
		lines := strings.Split(para, "\n")
		for _, line := range lines {
			pdf.MultiCell(170, 7, tr(line), "", "L", false)
		}
		pdf.Ln(4) // paragraph spacing
	}

	if err := pdf.OutputFileAndClose(outPath); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write PDF: %v", err))
	}

	scope := "tool:pdf:create"

	ref := outPath
	if t.mediaStore != nil {
		if r, storeErr := t.mediaStore.Store(outPath, media.MediaMeta{
			Filename:    output,
			ContentType: "application/pdf",
			Source:      "tool:pdf",
		}, scope); storeErr == nil {
			ref = r
		}
	}

	return MediaResult(fmt.Sprintf("PDF %q created (%d pages). Sending now.", output, pdf.PageCount()), []string{ref})
}

// handleExtract returns a prompt message telling the agent to read the file as an attachment.
func (t *PDFTool) handleExtract(args map[string]any) *ToolResult {
	path, _ := args["path"].(string)
	if path == "" {
		return ErrorResult("path is required for 'extract' action")
	}

	// Resolve media refs
	resolved := path
	if t.restrict && !strings.HasPrefix(path, "media://") {
		var err error
		resolved, err = validatePath(path, t.workspace, true)
		if err != nil {
			return ErrorResult(err.Error())
		}
	}

	return UserResult(fmt.Sprintf(
		"To extract text from %q, attach the file directly in the chat — I can read it using my vision capability. "+
			"Alternatively, if you share the file path I can attempt to parse it as a document.",
		filepath.Base(resolved),
	))
}
