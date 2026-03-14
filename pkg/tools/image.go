package tools

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/sipeed/picoclaw/pkg/media"
)

// ImageTool provides advanced image manipulation capabilities using imaging.
type ImageTool struct {
	workspace  string
	mediaStore media.MediaStore
}

func NewImageTool(workspace string, store media.MediaStore) *ImageTool {
	return &ImageTool{
		workspace:  workspace,
		mediaStore: store,
	}
}

func (t *ImageTool) Name() string { return "image" }
func (t *ImageTool) Description() string {
	return "Inspect, resize, crop, and manipulate images. Supports media:// and local paths."
}

func (t *ImageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"inspect", "resize", "crop", "annotate"},
				"description": "The action to perform.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the input image (local path or media:// reference).",
			},
			"width": map[string]any{
				"type":        "integer",
				"description": "Target width for 'resize'.",
			},
			"height": map[string]any{
				"type":        "integer",
				"description": "Target height for 'resize'.",
			},
			"x": map[string]any{"type": "integer", "description": "X coordinate for 'crop'."},
			"y": map[string]any{"type": "integer", "description": "Y coordinate for 'crop'."},
			"w": map[string]any{"type": "integer", "description": "Width for 'crop'."},
			"h": map[string]any{"type": "integer", "description": "Height for 'crop'."},
			"output": map[string]any{
				"type":        "string",
				"description": "Output filename (e.g., 'edited.png').",
			},
		},
		"required": []string{"action", "path"},
	}
}

func (t *ImageTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, _ := args["action"].(string)
	inputPath, _ := args["path"].(string)

	resolvedPath, err := t.resolvePath(inputPath)
	if err != nil {
		return ErrorResult(err.Error())
	}

	src, err := imaging.Open(resolvedPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to open image: %v", err))
	}

	switch action {
	case "inspect":
		bounds := src.Bounds()
		return UserResult(fmt.Sprintf("Image dimensions: %dx%d px", bounds.Dx(), bounds.Dy()))

	case "resize":
		width, _ := args["width"].(float64)
		height, _ := args["height"].(float64)
		if width == 0 && height == 0 {
			return ErrorResult("width or height must be specified for resize")
		}
		dst := imaging.Resize(src, int(width), int(height), imaging.Lanczos)
		return t.saveAndReturn(ctx, dst, args)

	case "crop":
		x, _ := args["x"].(float64)
		y, _ := args["y"].(float64)
		w, _ := args["w"].(float64)
		h, _ := args["h"].(float64)
		if w == 0 || h == 0 {
			return ErrorResult("w and h must be specified for crop")
		}
		dst := imaging.Crop(src, image.Rect(int(x), int(y), int(x+w), int(y+h)))
		return t.saveAndReturn(ctx, dst, args)

	case "annotate":
		// Placeholder for complex annotation. 
		// We'll just draw a red border for now to demonstrate manipulation.
		bounds := src.Bounds()
		dst := image.NewRGBA(bounds)
		draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
		
		red := color.RGBA{255, 0, 0, 255}
		// Draw simple border lines
		for i := 0; i < 5; i++ {
			draw.Draw(dst, image.Rect(bounds.Min.X, bounds.Min.Y+i, bounds.Max.X, bounds.Min.Y+i+1), &image.Uniform{red}, image.Point{}, draw.Src)
			draw.Draw(dst, image.Rect(bounds.Min.X, bounds.Max.Y-i-1, bounds.Max.X, bounds.Max.Y-i), &image.Uniform{red}, image.Point{}, draw.Src)
			draw.Draw(dst, image.Rect(bounds.Min.X+i, bounds.Min.Y, bounds.Min.X+i+1, bounds.Max.Y), &image.Uniform{red}, image.Point{}, draw.Src)
			draw.Draw(dst, image.Rect(bounds.Max.X-i-1, bounds.Min.Y, bounds.Max.X-i, bounds.Max.Y), &image.Uniform{red}, image.Point{}, draw.Src)
		}
		return t.saveAndReturn(ctx, dst, args)

	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *ImageTool) resolvePath(input string) (string, error) {
	if strings.HasPrefix(input, "media://") {
		if t.mediaStore == nil {
			return "", fmt.Errorf("media store not configured")
		}
		return t.mediaStore.Resolve(input)
	}
	// Fallback to workspace path validation
	return validatePath(input, t.workspace, true)
}

func (t *ImageTool) saveAndReturn(ctx context.Context, img image.Image, args map[string]any) *ToolResult {
	output, _ := args["output"].(string)
	if output == "" {
		output = fmt.Sprintf("output-%d.png", time.Now().Unix())
	}
	if !strings.HasSuffix(strings.ToLower(output), ".png") && !strings.HasSuffix(strings.ToLower(output), ".jpg") {
		output += ".png"
	}

	path := filepath.Join(t.workspace, output)
	if err := imaging.Save(img, path); err != nil {
		return ErrorResult(fmt.Sprintf("failed to save image: %v", err))
	}

	channel := ToolChannel(ctx)
	chatID := ToolChatID(ctx)
	scope := fmt.Sprintf("tool:image:manipulate:%s:%s", channel, chatID)

	ref := path
	if t.mediaStore != nil {
		if r, err := t.mediaStore.Store(path, media.MediaMeta{
			Filename:    output,
			ContentType: "image/png",
			Source:      "tool:image",
		}, scope); err == nil {
			ref = r
		}
	}

	return MediaResult(fmt.Sprintf("Image %q processed successfully", output), []string{ref})
}
