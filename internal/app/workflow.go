package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
)

type workflowCatalogResponse struct {
	Workflows []workflowDefinition `json:"workflows"`
}

type workflowDefinition struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Inputs      []workflowInputField     `json:"inputs,omitempty"`
	Steps       []workflowStepDefinition `json:"steps,omitempty"`
}

type workflowInputField struct {
	Key         string                `json:"key"`
	Label       string                `json:"label"`
	Type        string                `json:"type"`
	Default     string                `json:"default,omitempty"`
	Placeholder string                `json:"placeholder,omitempty"`
	Help        string                `json:"help,omitempty"`
	Required    bool                  `json:"required,omitempty"`
	Options     []workflowFieldOption `json:"options,omitempty"`
	Min         *int                  `json:"min,omitempty"`
	Max         *int                  `json:"max,omitempty"`
}

type workflowFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type workflowStepDefinition struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Title             string `json:"title"`
	Prompt            string `json:"prompt"`
	TextModel         string `json:"text_model,omitempty"`
	ImageModel        string `json:"image_model,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	Count             string `json:"count,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression string `json:"output_compression,omitempty"`
	Background        string `json:"background,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	UseReferences     bool   `json:"use_references,omitempty"`
	UseMask           bool   `json:"use_mask,omitempty"`
}

type workflowStepResult struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Title  string   `json:"title"`
	Prompt string   `json:"prompt,omitempty"`
	Text   string   `json:"text,omitempty"`
	Images []string `json:"images,omitempty"`
	Saved  []string `json:"saved,omitempty"`
}

type workflowRunResult struct {
	WorkflowID   string               `json:"workflow_id"`
	WorkflowName string               `json:"workflow_name"`
	Prompt       string               `json:"prompt"`
	Params       map[string]string    `json:"params,omitempty"`
	NeedsReview  bool                 `json:"needs_review,omitempty"`
	Text         string               `json:"text,omitempty"`
	Images       []string             `json:"images,omitempty"`
	Saved        []string             `json:"saved,omitempty"`
	Steps        []workflowStepResult `json:"steps,omitempty"`
}

type workflowRuntimeInput struct {
	Prompt        string
	Params        map[string]string
	SelectedSteps map[string]bool
	StepTexts     map[string]string
	Mode          string
	References    []imageRef
	Mask          *imageRef
	BaseOptions   imageOptions
	ImageModel    string
	TextModel     string
	Endpoint      string
	SaveDir       string
}

type workflowTemplateData struct {
	Prompt    string
	Params    map[string]string
	Steps     map[string]workflowStepResult
	Overrides map[string]string
	StepIndex int
	StepCount int
}

var (
	builtinWorkflowCatalogOnce  sync.Once
	builtinWorkflowCatalogValue workflowCatalogResponse
	builtinWorkflowCatalogErr   error
)

func builtinWorkflowCatalog() (workflowCatalogResponse, error) {
	builtinWorkflowCatalogOnce.Do(func() {
		data, err := webUIAssets.ReadFile("webui/workflows.json")
		if err != nil {
			builtinWorkflowCatalogErr = err
			return
		}
		var catalog workflowCatalogResponse
		if err := json.Unmarshal(data, &catalog); err != nil {
			builtinWorkflowCatalogErr = err
			return
		}
		builtinWorkflowCatalogValue = catalog
	})
	return builtinWorkflowCatalogValue, builtinWorkflowCatalogErr
}

func workflowCatalogByID(id string) (workflowDefinition, bool, error) {
	id = strings.TrimSpace(id)
	catalog, err := builtinWorkflowCatalog()
	if err != nil {
		return workflowDefinition{}, false, err
	}
	for _, wf := range catalog.Workflows {
		if strings.EqualFold(wf.ID, id) {
			return wf, true, nil
		}
	}
	return workflowDefinition{}, false, nil
}

func (s *webUIServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	catalog, err := builtinWorkflowCatalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (s *webUIServer) handleWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.ParseMultipartForm(80 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workflowID := strings.TrimSpace(r.FormValue("workflow_id"))
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	workflow, ok, err := workflowCatalogByID(workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown workflow")
		return
	}
	params, err := workflowParamsFromForm(r, workflow)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if workflowID == "single" && prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	stepTexts, err := workflowStepTextsFromForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mode := strings.ToLower(strings.TrimSpace(r.FormValue("workflow_mode")))
	baseOpts, err := s.imageOptionsFromForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	needUploadClient := (len(r.MultipartForm.File["images"]) > 0 || len(r.MultipartForm.File["mask"]) > 0) && strings.EqualFold(strings.TrimSpace(s.fileFormat), fileFormatURL)
	var c *client
	if needUploadClient {
		c, err = s.clientOpts.newClient()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer c.Close()
	}
	refs, err := uploadedImageRefs(r, c, s.fileFormat)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mask, err := uploadedMaskRef(r, c, s.fileFormat)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if mask != nil && len(refs) == 0 {
		writeError(w, http.StatusBadRequest, "mask requires at least one reference image")
		return
	}
	if c == nil {
		c, err = s.clientOpts.newClient()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer c.Close()
	}

	saveDir, err := s.saveDirectoryFromForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	wantStream := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("stream")), "1") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("stream")), "true") ||
		strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")

	if wantStream {
		stream := newWebUIEventStream(w)
		selectedSteps := workflowSelectedStepsFromForm(r, workflow)
		result, err := s.runWorkflow(ctx, c, workflow, workflowRuntimeInput{
			Prompt:        prompt,
			Params:        params,
			SelectedSteps: selectedSteps,
			StepTexts:     stepTexts,
			Mode:          mode,
			References:    refs,
			Mask:          mask,
			BaseOptions:   baseOpts,
			ImageModel:    s.clientOpts.imageGenerationModel(),
			TextModel:     strings.TrimSpace(*s.clientOpts.model),
			Endpoint:      webUIImageEndpoint(r),
			SaveDir:       saveDir,
		}, stream)
		if err != nil {
			stream.send("error", map[string]string{"error": err.Error()})
			return
		}
		stream.send("done", result)
		return
	}

	selectedSteps := workflowSelectedStepsFromForm(r, workflow)
	result, err := s.runWorkflow(ctx, c, workflow, workflowRuntimeInput{
		Prompt:        prompt,
		Params:        params,
		SelectedSteps: selectedSteps,
		StepTexts:     stepTexts,
		Mode:          mode,
		References:    refs,
		Mask:          mask,
		BaseOptions:   baseOpts,
		ImageModel:    s.clientOpts.imageGenerationModel(),
		TextModel:     strings.TrimSpace(*s.clientOpts.model),
		Endpoint:      webUIImageEndpoint(r),
		SaveDir:       saveDir,
	}, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *webUIServer) runWorkflow(ctx context.Context, c *client, workflow workflowDefinition, input workflowRuntimeInput, stream *webUIEventStream) (workflowRunResult, error) {
	execSteps := workflowExecutableSteps(workflow, input.SelectedSteps)
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	draft := mode == "draft"
	if mode == "" && len(input.StepTexts) == 0 && workflowNeedsReview(workflow) {
		draft = true
	}
	result := workflowRunResult{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		Prompt:       input.Prompt,
		Params:       input.Params,
		Steps:        make([]workflowStepResult, 0, len(execSteps)),
	}
	stepIndex := make(map[string]workflowStepResult, len(execSteps))
	var textParts []string
	var saver *responseImageSaver
	if stream != nil {
		saver = newResponseImageSaver(ctx, c.httpClient, input.SaveDir)
	}

	for i, step := range execSteps {
		if stream != nil {
			stream.send("status", map[string]string{
				"message": fmt.Sprintf("正在执行 %d/%d：%s", i+1, len(execSteps), step.Title),
			})
			stream.send("step", map[string]any{
				"id":    step.ID,
				"title": step.Title,
				"kind":  step.Kind,
				"index": i + 1,
				"count": len(execSteps),
			})
		}

		data := workflowTemplateData{
			Prompt:    input.Prompt,
			Params:    input.Params,
			Steps:     stepIndex,
			Overrides: input.StepTexts,
			StepIndex: i + 1,
			StepCount: len(execSteps),
		}
		stepPrompt, err := renderWorkflowTemplate(step.Prompt, data)
		if err != nil {
			return result, err
		}
		stepResult := workflowStepResult{
			ID:     step.ID,
			Kind:   step.Kind,
			Title:  step.Title,
			Prompt: stepPrompt,
		}

		switch strings.ToLower(strings.TrimSpace(step.Kind)) {
		case "response":
			if override, ok := input.StepTexts[step.ID]; ok && !draft {
				stepResult.Text = strings.TrimSpace(override)
				if stepResult.Text != "" {
					textParts = append(textParts, stepResult.Text)
				}
				result.Steps = append(result.Steps, stepResult)
				stepIndex[step.ID] = stepResult
				continue
			}
			model := strings.TrimSpace(step.TextModel)
			if model == "" {
				model = input.TextModel
			}
			resp, err := func() (*responsePayload, error) {
				if stream != nil {
					return c.createResponseStreamWithImages(ctx, responseRequest{
						Model: model,
						Input: s.responseInput(stepPrompt, input.References),
					}, stream.textWriter(), nil)
				}
				return c.createResponse(ctx, responseRequest{
					Model: model,
					Input: s.responseInput(stepPrompt, input.References),
				})
			}()
			if err != nil {
				return result, err
			}
			stepResult.Text = strings.TrimSpace(resp.text())
			if stepResult.Text != "" {
				textParts = append(textParts, stepResult.Text)
			}
			result.Steps = append(result.Steps, stepResult)
			stepIndex[step.ID] = stepResult
			continue

		case "image":
			if draft {
				result.NeedsReview = true
				result.Text = strings.Join(textParts, "\n\n")
				result.Images = uniqueStrings(result.Images)
				result.Saved = uniqueStrings(result.Saved)
				return result, nil
			}
			stepOpts, err := workflowStepImageOptions(input.BaseOptions, step, data)
			if err != nil {
				return result, err
			}
			endpoint := strings.TrimSpace(step.Endpoint)
			if endpoint == "" {
				endpoint = input.Endpoint
			}
			useRefs := step.UseReferences && len(input.References) > 0
			var saved []string
			var text string
			if stream != nil {
				onImage := func(candidate imageCandidate) error {
					if saver == nil {
						return nil
					}
					path, err := saver.saveCandidate(candidate)
					if err != nil {
						return err
					}
					if path == "" {
						return nil
					}
					saved = append(saved, path)
					urls := s.publicImageURLs([]string{path}, input.SaveDir)
					stepResult.Saved = append(stepResult.Saved, path)
					stepResult.Images = append(stepResult.Images, urls...)
					result.Saved = append(result.Saved, path)
					result.Images = append(result.Images, urls...)
					if stream != nil && len(urls) > 0 {
						stream.send("image", map[string]string{"step_id": step.ID, "url": urls[0]})
					}
					return nil
				}
				var runErr error
				if endpoint == "response" {
					var resp *responsePayload
					resp, runErr = s.handleGenerateWithResponsesStream(ctx, c, stepOpts, imageModelOrDefault(step.ImageModel, input.ImageModel), stepPrompt, pickRefs(useRefs, input.References), pickMask(step.UseMask, input.Mask), stream, onImage)
					if runErr == nil && resp != nil {
						text = strings.TrimSpace(resp.OutputText)
					}
				} else {
					var resp *responsePayload
					resp, runErr = s.handleGenerateWithImageEndpointStream(ctx, c, stepOpts, imageModelOrDefault(step.ImageModel, input.ImageModel), stepPrompt, pickRefs(useRefs, input.References), pickMask(step.UseMask, input.Mask), stream, onImage)
					if runErr == nil && resp != nil {
						text = strings.TrimSpace(resp.OutputText)
					}
				}
				if runErr != nil {
					return result, runErr
				}
			} else {
				var runErr error
				if endpoint == "response" {
					saved, text, runErr = s.handleGenerateWithResponses(ctx, c, stepOpts, imageModelOrDefault(step.ImageModel, input.ImageModel), stepPrompt, pickRefs(useRefs, input.References), pickMask(step.UseMask, input.Mask), input.SaveDir)
				} else {
					var resp *responsePayload
					resp, runErr = s.handleGenerateWithImageEndpoint(ctx, c, stepOpts, imageModelOrDefault(step.ImageModel, input.ImageModel), stepPrompt, pickRefs(useRefs, input.References), pickMask(step.UseMask, input.Mask))
					if runErr == nil {
						saved, runErr = saveImagesFromResponse(ctx, c.httpClient, resp, input.SaveDir)
					}
					if runErr == nil && resp != nil {
						text = strings.TrimSpace(resp.OutputText)
					}
				}
				if runErr != nil {
					return result, runErr
				}
				stepResult.Saved = append(stepResult.Saved, saved...)
				stepResult.Images = append(stepResult.Images, s.publicImageURLs(saved, input.SaveDir)...)
				result.Saved = append(result.Saved, saved...)
				result.Images = append(result.Images, s.publicImageURLs(saved, input.SaveDir)...)
			}
			stepResult.Text = text
			if text != "" {
				textParts = append(textParts, text)
			}
			result.Steps = append(result.Steps, stepResult)
			stepIndex[step.ID] = stepResult
			continue

		default:
			return result, fmt.Errorf("unsupported workflow step kind: %s", step.Kind)
		}
	}

	result.Text = strings.Join(textParts, "\n\n")
	result.Images = uniqueStrings(result.Images)
	result.Saved = uniqueStrings(result.Saved)
	return result, nil
}

func workflowParamsFromForm(r *http.Request, workflow workflowDefinition) (map[string]string, error) {
	params := make(map[string]string, len(workflow.Inputs))
	for _, field := range workflow.Inputs {
		value := strings.TrimSpace(r.FormValue("workflow_" + field.Key))
		if value == "" {
			value = strings.TrimSpace(field.Default)
		}
		if field.Required && value == "" {
			return nil, fmt.Errorf("%s is required", field.Label)
		}
		params[field.Key] = value
	}
	if selected := workflowSelectedStepIDs(r, workflow); len(selected) > 0 {
		params["templates"] = strings.Join(selected, ",")
	}
	return params, nil
}

func workflowSelectedStepsFromForm(r *http.Request, workflow workflowDefinition) map[string]bool {
	selected := workflowSelectedStepIDs(r, workflow)
	if len(selected) == 0 {
		return nil
	}
	result := make(map[string]bool, len(selected))
	for _, id := range selected {
		result[id] = true
	}
	return result
}

func workflowSelectedStepIDs(r *http.Request, workflow workflowDefinition) []string {
	raw := strings.TrimSpace(r.FormValue("workflow_templates"))
	selected := make(map[string]bool)
	if raw != "" {
		for _, item := range strings.Split(raw, ",") {
			id := strings.TrimSpace(item)
			if id != "" {
				selected[id] = true
			}
		}
	}

	defaults := make([]string, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		if strings.EqualFold(strings.TrimSpace(step.Kind), "image") {
			defaults = append(defaults, step.ID)
		}
	}
	if len(selected) == 0 {
		return defaults
	}

	result := make([]string, 0, len(defaults))
	for _, step := range workflow.Steps {
		if !strings.EqualFold(strings.TrimSpace(step.Kind), "image") {
			continue
		}
		if selected[step.ID] {
			result = append(result, step.ID)
		}
	}
	if len(result) == 0 {
		return defaults
	}
	return result
}

func workflowExecutableSteps(workflow workflowDefinition, selected map[string]bool) []workflowStepDefinition {
	steps := make([]workflowStepDefinition, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		if strings.EqualFold(strings.TrimSpace(step.Kind), "image") && len(selected) > 0 && !selected[step.ID] {
			continue
		}
		steps = append(steps, step)
	}
	return steps
}

func renderWorkflowTemplate(tmplText string, data workflowTemplateData) (string, error) {
	tmplText = strings.TrimSpace(tmplText)
	if tmplText == "" {
		return "", nil
	}
	tmpl, err := template.New("workflow").Funcs(workflowTemplateFuncMap(data)).Parse(tmplText)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func workflowTemplateFuncMap(data workflowTemplateData) template.FuncMap {
	return template.FuncMap{
		"field": func(key string) string {
			return strings.TrimSpace(data.Params[key])
		},
		"stepText": func(key string) string {
			if override, ok := data.Overrides[key]; ok {
				return strings.TrimSpace(override)
			}
			return strings.TrimSpace(data.Steps[key].Text)
		},
		"stepPrompt": func(key string) string {
			return strings.TrimSpace(data.Steps[key].Prompt)
		},
		"default": func(value, fallback string) string {
			if strings.TrimSpace(value) != "" {
				return value
			}
			return fallback
		},
		"commerceSize": commerceWorkflowSize,
		"join": func(values []string, sep string) string {
			return strings.Join(values, sep)
		},
		"trim": strings.TrimSpace,
	}
}

func commerceWorkflowSize(resolution, kind string) string {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if resolution == "" {
		resolution = "1k"
	}
	mainSizes := map[string]string{
		"1k": "1024x1024",
		"2k": "2048x2048",
		"4k": "2880x2880",
	}
	detailSizes := map[string]string{
		"1k": "1024x1536",
		"2k": "1440x2560",
		"4k": "2160x3840",
	}
	if strings.EqualFold(strings.TrimSpace(kind), "detail") {
		if size := detailSizes[resolution]; size != "" {
			return size
		}
		return detailSizes["1k"]
	}
	if size := mainSizes[resolution]; size != "" {
		return size
	}
	return mainSizes["1k"]
}

func workflowStepImageOptions(base imageOptions, step workflowStepDefinition, data workflowTemplateData) (imageOptions, error) {
	opts := base
	var err error
	if strings.TrimSpace(step.Count) != "" {
		if opts.count == nil {
			opts.count = new(int)
		}
		*opts.count, err = workflowRenderInt(step.Count, data, *opts.count, 1)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.Size) != "" {
		if opts.size == nil {
			opts.size = new(string)
		}
		*opts.size, err = renderWorkflowTemplate(step.Size, data)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.Quality) != "" {
		if opts.quality == nil {
			opts.quality = new(string)
		}
		*opts.quality, err = renderWorkflowTemplate(step.Quality, data)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.OutputFormat) != "" {
		if opts.outputFormat == nil {
			opts.outputFormat = new(string)
		}
		*opts.outputFormat, err = renderWorkflowTemplate(step.OutputFormat, data)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.OutputCompression) != "" {
		if opts.outputCompression == nil {
			opts.outputCompression = new(int)
		}
		*opts.outputCompression, err = workflowRenderInt(step.OutputCompression, data, *opts.outputCompression, 0)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.Background) != "" {
		if opts.background == nil {
			opts.background = new(string)
		}
		*opts.background, err = renderWorkflowTemplate(step.Background, data)
		if err != nil {
			return opts, err
		}
	}
	if strings.TrimSpace(step.Moderation) != "" {
		if opts.moderation == nil {
			opts.moderation = new(string)
		}
		*opts.moderation, err = renderWorkflowTemplate(step.Moderation, data)
		if err != nil {
			return opts, err
		}
	}
	return opts, nil
}

func workflowRenderInt(tmplText string, data workflowTemplateData, fallback int, min int) (int, error) {
	rendered, err := renderWorkflowTemplate(tmplText, data)
	if err != nil {
		return 0, err
	}
	if rendered == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(rendered)
	if err != nil {
		return 0, fmt.Errorf("invalid workflow integer %q", rendered)
	}
	if value < min {
		return 0, errors.New("workflow integer out of range")
	}
	return value, nil
}

func workflowNeedsReview(workflow workflowDefinition) bool {
	seenResponse := false
	for _, step := range workflow.Steps {
		switch strings.ToLower(strings.TrimSpace(step.Kind)) {
		case "response":
			seenResponse = true
		case "image":
			if seenResponse {
				return true
			}
		}
	}
	return false
}

func workflowStepTextsFromForm(r *http.Request) (map[string]string, error) {
	raw := strings.TrimSpace(r.FormValue("workflow_step_texts"))
	if raw == "" {
		return map[string]string{}, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("invalid workflow_step_texts: %w", err)
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		result[key] = strings.TrimSpace(value)
	}
	return result, nil
}

func imageModelOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func pickRefs(use bool, refs []imageRef) []imageRef {
	if !use {
		return nil
	}
	return refs
}

func pickMask(use bool, mask *imageRef) *imageRef {
	if !use {
		return nil
	}
	return mask
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
