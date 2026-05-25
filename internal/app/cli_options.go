package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (opts clientOptions) newClient() (*client, error) {
	return newClient(*opts.baseURL, *opts.ossURL, *opts.apiKey, *opts.username, *opts.password, *opts.verbose, *opts.logFile)
}

func (opts clientOptions) imageGenerationModel() string {
	if opts.imageModel != nil && strings.TrimSpace(*opts.imageModel) != "" {
		return *opts.imageModel
	}
	if opts.model != nil {
		return *opts.model
	}
	return ""
}

func (opts inputOptions) buildInput(ctx context.Context, c *client, prompt, system string) ([]inputMessage, error) {
	images, textFiles, err := collectInputAttachments(prompt, *opts.attachments)
	if err != nil {
		return nil, err
	}
	return buildInputWithFileFormat(ctx, c, prompt, system, images, textFiles, *opts.fileFormat)
}

func (opts inputOptions) hasPromptOrAttachments(prompt string) bool {
	return strings.TrimSpace(prompt) != "" || len(*opts.attachments) > 0
}

func (opts imageOptions) validate() error {
	if opts.count != nil && (*opts.count < 1 || *opts.count > 10) {
		return fmt.Errorf("invalid -image-count %d: expected 1-10", *opts.count)
	}
	if opts.outputCompression != nil && *opts.outputCompression != -1 && (*opts.outputCompression < 0 || *opts.outputCompression > 100) {
		return fmt.Errorf("invalid -image-output-compression %d: expected -1 or 0-100", *opts.outputCompression)
	}
	return nil
}

func (opts imageOptions) responseTool() map[string]any {
	tool := map[string]any{"type": "image_generation"}
	addAutoString(tool, "size", opts.size)
	addAutoString(tool, "quality", opts.quality)
	addString(tool, "output_format", opts.outputFormat)
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		tool["output_compression"] = *opts.outputCompression
	}
	addAutoString(tool, "background", opts.background)
	addAutoString(tool, "action", opts.action)

	return tool
}

func (opts imageOptions) responseRunCount() int {
	if opts.count != nil && *opts.count > 1 {
		return *opts.count
	}
	return 1
}

func (opts imageOptions) generationRequest(model, prompt string) imageGenerationRequest {
	req := imageGenerationRequest{
		Model:  model,
		Prompt: prompt,
	}
	if opts.count != nil && *opts.count > 1 {
		req.N = *opts.count
	}
	if opts.size != nil && strings.TrimSpace(*opts.size) != "" && !strings.EqualFold(strings.TrimSpace(*opts.size), "auto") {
		req.Size = strings.TrimSpace(*opts.size)
	}
	if opts.quality != nil && strings.TrimSpace(*opts.quality) != "" && !strings.EqualFold(strings.TrimSpace(*opts.quality), "auto") {
		req.Quality = strings.TrimSpace(*opts.quality)
	}
	if opts.outputFormat != nil {
		req.OutputFormat = strings.TrimSpace(*opts.outputFormat)
	}
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		req.OutputCompression = opts.outputCompression
	}
	if opts.background != nil && strings.TrimSpace(*opts.background) != "" && !strings.EqualFold(strings.TrimSpace(*opts.background), "auto") {
		req.Background = strings.TrimSpace(*opts.background)
	}
	if opts.moderation != nil {
		req.Moderation = strings.TrimSpace(*opts.moderation)
	}
	return req
}

func (opts imageOptions) editRequest(model, prompt string, images []imageRef, mask *imageRef) imageEditRequest {
	req := imageEditRequest{
		Model:  model,
		Prompt: prompt,
		Images: images,
		Mask:   mask,
	}
	if opts.count != nil && *opts.count > 1 {
		req.N = *opts.count
	}
	if opts.size != nil && strings.TrimSpace(*opts.size) != "" && !strings.EqualFold(strings.TrimSpace(*opts.size), "auto") {
		req.Size = strings.TrimSpace(*opts.size)
	}
	if opts.quality != nil && strings.TrimSpace(*opts.quality) != "" && !strings.EqualFold(strings.TrimSpace(*opts.quality), "auto") {
		req.Quality = strings.TrimSpace(*opts.quality)
	}
	if opts.outputFormat != nil {
		req.OutputFormat = strings.TrimSpace(*opts.outputFormat)
	}
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		req.OutputCompression = opts.outputCompression
	}
	if opts.background != nil && strings.TrimSpace(*opts.background) != "" && !strings.EqualFold(strings.TrimSpace(*opts.background), "auto") {
		req.Background = strings.TrimSpace(*opts.background)
	}
	if opts.moderation != nil {
		req.Moderation = strings.TrimSpace(*opts.moderation)
	}
	return req
}

func imageRefs(ctx context.Context, c *client, images []string, fileFormat string) ([]imageRef, error) {
	if len(images) > 16 {
		return nil, fmt.Errorf("too many image inputs: got %d, expected at most 16", len(images))
	}
	refs := make([]imageRef, 0, len(images))
	for _, image := range images {
		ref, err := imageRefFromInput(ctx, c, image, fileFormat)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func imageRefFromInput(ctx context.Context, c *client, value, fileFormat string) (imageRef, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return imageRef{}, errors.New("empty image input")
	}
	imageURL, err := imageInputURL(ctx, c, value, fileFormat)
	if err != nil {
		return imageRef{}, err
	}
	return imageRef{ImageURL: imageURL}, nil
}

func addString(values map[string]any, key string, value *string) {
	if value != nil && strings.TrimSpace(*value) != "" {
		values[key] = strings.TrimSpace(*value)
	}
}

func addAutoString(values map[string]any, key string, value *string) {
	if value != nil && strings.TrimSpace(*value) != "" && !strings.EqualFold(strings.TrimSpace(*value), "auto") {
		values[key] = strings.TrimSpace(*value)
	}
}

func promptWithTextFiles(prompt string, files []textFileContent) string {
	parts := []string{}
	if strings.TrimSpace(prompt) != "" {
		parts = append(parts, strings.TrimSpace(prompt))
	}
	for _, file := range files {
		parts = append(parts, formatTextFileContent(file))
	}
	return strings.Join(parts, "\n\n")
}
