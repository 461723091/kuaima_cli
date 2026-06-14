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
