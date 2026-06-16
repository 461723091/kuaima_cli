package app

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestWorkflowNeedsReview(t *testing.T) {
	t.Run("response before image", func(t *testing.T) {
		wf := workflowDefinition{
			Steps: []workflowStepDefinition{
				{ID: "brief", Kind: "response"},
				{ID: "image", Kind: "image"},
			},
		}
		if !workflowNeedsReview(wf) {
			t.Fatal("expected workflow to need review")
		}
	})

	t.Run("image before response", func(t *testing.T) {
		wf := workflowDefinition{
			Steps: []workflowStepDefinition{
				{ID: "image", Kind: "image"},
				{ID: "brief", Kind: "response"},
			},
		}
		if workflowNeedsReview(wf) {
			t.Fatal("expected workflow to skip review")
		}
	})
}

func TestWorkflowStepTextsFromForm(t *testing.T) {
	form := url.Values{}
	form.Set("workflow_step_texts", `{"brief":"  refined copy  ","empty":""}`)
	req := httptest.NewRequest("POST", "/", nil)
	req.Form = form
	req.PostForm = form

	values, err := workflowStepTextsFromForm(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := values["brief"]; got != "refined copy" {
		t.Fatalf("unexpected brief text: %q", got)
	}
	if got := values["empty"]; got != "" {
		t.Fatalf("unexpected empty text: %q", got)
	}
}

func TestWorkflowTemplateStepTextOverride(t *testing.T) {
	data := workflowTemplateData{
		Steps: map[string]workflowStepResult{
			"brief": {Text: "generated copy"},
		},
		Overrides: map[string]string{
			"brief": "edited copy",
		},
	}
	fn := workflowTemplateFuncMap(data)["stepText"].(func(string) string)
	if got := fn("brief"); got != "edited copy" {
		t.Fatalf("unexpected step text: %q", got)
	}
}

func TestCommerceWorkflowSizes(t *testing.T) {
	if got := commerceWorkflowSize("2k", "main"); got != "2048x2048" {
		t.Fatalf("unexpected main size: %q", got)
	}
	if got := commerceWorkflowSize("4k", "detail"); got != "2160x3840" {
		t.Fatalf("unexpected detail size: %q", got)
	}
	if got := commerceWorkflowSize("unknown", "detail"); got != "1024x1536" {
		t.Fatalf("unexpected fallback detail size: %q", got)
	}
}

func TestBuiltinWorkflowTemplatesRender(t *testing.T) {
	data := workflowTemplateData{
		Prompt: "",
		Params: map[string]string{
			"product_brief":    "test product",
			"platform":         "淘宝",
			"copy_language":    "简体中文",
			"audience":         "年轻女性",
			"visual_style":     "高级感",
			"image_resolution": "1k",
			"tone":             "真实",
			"image_count":      "1",
		},
		Steps: map[string]workflowStepResult{
			"brief": {Text: "core selling point"},
			"copy":  {Text: "copy text"},
		},
		Overrides: map[string]string{},
	}
	for _, workflow := range builtinWorkflowCatalog().Workflows {
		for _, step := range workflow.Steps {
			if _, err := renderWorkflowTemplate(step.Prompt, data); err != nil {
				t.Fatalf("render prompt for %s/%s: %v", workflow.ID, step.ID, err)
			}
			if step.Size != "" {
				if _, err := renderWorkflowTemplate(step.Size, data); err != nil {
					t.Fatalf("render size for %s/%s: %v", workflow.ID, step.ID, err)
				}
			}
		}
	}
}
