package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func builtinWorkflowCatalog() workflowCatalogResponse {
	return workflowCatalogResponse{
		Workflows: []workflowDefinition{
			{
				ID:          "product-pack",
				Name:        "电商套图",
				Description: "上传产品参考图，配置平台、语言、受众和视觉风格，勾选需要的主图或详情图模板。",
				Inputs: []workflowInputField{
					{Key: "product_brief", Label: "商品信息", Type: "textarea", Required: true, Placeholder: "描述商品卖点、材质、用途、规格等"},
					{Key: "platform", Label: "平台", Type: "combo", Default: "淘宝", Placeholder: "选择或输入平台", Options: []workflowFieldOption{
						{Value: "淘宝", Label: "淘宝"},
						{Value: "抖音", Label: "抖音"},
						{Value: "微信小店", Label: "微信小店"},
						{Value: "拼多多", Label: "拼多多"},
						{Value: "亚马逊", Label: "亚马逊"},
						{Value: "TikTok", Label: "TikTok"},
					}},
					{Key: "copy_language", Label: "文案语言", Type: "combo", Default: "简体中文", Placeholder: "选择或输入语言", Options: []workflowFieldOption{
						{Value: "简体中文", Label: "简体中文"},
						{Value: "英文", Label: "英文"},
						{Value: "繁体中文", Label: "繁体中文"},
						{Value: "日文", Label: "日文"},
						{Value: "韩文", Label: "韩文"},
					}},
					{Key: "audience", Label: "受众目标", Type: "combo", Default: "年轻女性", Placeholder: "选择或输入受众", Options: []workflowFieldOption{
						{Value: "年轻女性", Label: "年轻女性"},
						{Value: "商务人士", Label: "商务人士"},
						{Value: "老年群体", Label: "老年群体"},
						{Value: "新手妈妈", Label: "新手妈妈"},
						{Value: "学生群体", Label: "学生群体"},
						{Value: "户外运动人群", Label: "户外运动人群"},
					}},
					{Key: "visual_style", Label: "视觉风格", Type: "combo", Default: "高级感、干净背景、商业摄影", Placeholder: "选择或输入视觉风格", Options: []workflowFieldOption{
						{Value: "高级感、干净背景、商业摄影", Label: "高级商业摄影"},
						{Value: "极简白底、清晰产品轮廓", Label: "极简白底"},
						{Value: "年轻潮流、强对比色、社媒感", Label: "年轻潮流"},
						{Value: "科技感、冷色调、精密质感", Label: "科技感"},
						{Value: "温暖生活方式、自然光、真实场景", Label: "生活方式"},
						{Value: "跨境电商、干净排版、英文卖点", Label: "跨境电商"},
					}},
					{Key: "image_resolution", Label: "工作流分辨率", Type: "select", Default: "1k", Options: []workflowFieldOption{
						{Value: "1k", Label: "1k"},
						{Value: "2k", Label: "2k"},
						{Value: "4k", Label: "4k"},
					}},
				},
				Steps: []workflowStepDefinition{
					{
						ID:        "brief",
						Kind:      "response",
						Title:     "提炼卖点",
						Prompt:    "根据商品信息为电商套图提炼核心卖点、目标受众、平台表达重点和文案关键词。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。受众目标：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						TextModel: "",
					},
					{
						ID:            "white_bg_main",
						Kind:          "image",
						Title:         "主图 · 产品白底图",
						Prompt:        "生成电商主图：产品白底图。画面比例 1:1，产品居中，占画面主体，背景纯净，边缘清晰，适合{{field \"platform\"}}商品主图。文案语言：{{field \"copy_language\"}}。受众：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"main\"}}",
						UseReferences: true,
					},
					{
						ID:            "hero_main",
						Kind:          "image",
						Title:         "主图 · 首屏主视觉",
						Prompt:        "生成电商主图：首屏主视觉。画面比例 1:1，突出第一眼点击欲望和核心购买理由，适合{{field \"platform\"}}列表或首屏。文案语言：{{field \"copy_language\"}}。受众：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"main\"}}",
						UseReferences: true,
					},
					{
						ID:            "multi_angle",
						Kind:          "image",
						Title:         "详情图 · 多角度展示",
						Prompt:        "生成电商详情图：多角度展示。画面比例 3:4，用清晰分区展示产品正面、侧面、背面或局部角度，强调真实结构和质感。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。受众：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "specs",
						Kind:          "image",
						Title:         "详情图 · 规格参数",
						Prompt:        "生成电商详情图：规格参数。画面比例 3:4，排版清晰，展示尺寸、材质、容量、型号或关键参数，信息层级明确。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "effect_compare",
						Kind:          "image",
						Title:         "详情图 · 效果对比",
						Prompt:        "生成电商详情图：效果对比。画面比例 3:4，用对比结构呈现使用前后、普通方案和本商品方案的差异，转化导向明确。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。受众：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "craft",
						Kind:          "image",
						Title:         "详情图 · 工艺制作图",
						Prompt:        "生成电商详情图：工艺制作图。画面比例 3:4，展示材质、工艺、制作流程或细节放大，强调品质可信度。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "accessories",
						Kind:          "image",
						Title:         "详情图 · 配件赠品",
						Prompt:        "生成电商详情图：配件赠品。画面比例 3:4，清楚展示包装内含物、配件、赠品和数量关系，画面整洁可信。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "series",
						Kind:          "image",
						Title:         "详情图 · 系列展示图",
						Prompt:        "生成电商详情图：系列展示图。画面比例 3:4，展示多规格、多颜色、多款式或系列组合，统一视觉秩序。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "package",
						Kind:          "image",
						Title:         "详情图 · 包装展示图",
						Prompt:        "生成电商详情图：包装展示图。画面比例 3:4，展示外包装、开箱内容、包装质感和送礼属性。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "ingredients",
						Kind:          "image",
						Title:         "详情图 · 商品成分图",
						Prompt:        "生成电商详情图：商品成分图。画面比例 3:4，适合展示成分、材质构成、配方或核心模块，表达专业、清晰、可信。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "service",
						Kind:          "image",
						Title:         "详情图 · 售后保障图",
						Prompt:        "生成电商详情图：售后保障图。画面比例 3:4，呈现质保、退换、客服、配送或服务承诺，风格可靠克制。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "usage_advice",
						Kind:          "image",
						Title:         "详情图 · 使用建议图",
						Prompt:        "生成电商详情图：使用建议图。画面比例 3:4，展示使用步骤、适用场景、注意事项或搭配建议，让用户快速理解怎么用。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。受众：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "brand_story",
						Kind:          "image",
						Title:         "详情图 · 品牌故事图",
						Prompt:        "生成电商详情图：品牌故事图。画面比例 3:4，表达品牌理念、产地、研发背景或品质主张，避免空泛，用视觉建立信任。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
					{
						ID:            "buyer_show",
						Kind:          "image",
						Title:         "详情图 · 买家秀图",
						Prompt:        "生成电商详情图：买家秀图。画面比例 3:4，模拟真实买家使用或展示商品的自然场景，真实、可信、有生活感，符合{{field \"audience\"}}审美。平台：{{field \"platform\"}}。文案语言：{{field \"copy_language\"}}。商品信息：\n{{field \"product_brief\"}}\n卖点提炼：\n{{stepText \"brief\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "1",
						Size:          "{{commerceSize (field \"image_resolution\") \"detail\"}}",
						UseReferences: true,
					},
				},
			},
			{
				ID:          "social-commerce",
				Name:        "图文带货",
				Description: "生成带货文案和配图，适合小红书、抖音图文和投放种草。",
				Inputs: []workflowInputField{
					{Key: "product_brief", Label: "商品信息", Type: "textarea", Required: true, Placeholder: "描述商品卖点、价格带、目标人群"},
					{Key: "audience", Label: "目标人群", Type: "text", Default: "25-35岁女性消费人群", Placeholder: "例如：新手妈妈、露营用户"},
					{Key: "tone", Label: "文案语气", Type: "text", Default: "真实、种草、带一点转化导向", Placeholder: "例如：轻松、专业、强转化"},
					{Key: "image_count", Label: "配图数量", Type: "number", Default: "3"},
					{Key: "visual_style", Label: "视觉风格", Type: "text", Default: "生活方式感、真实场景、干净明亮", Placeholder: "例如：种草风、生活感、氛围感"},
				},
				Steps: []workflowStepDefinition{
					{
						ID:        "copy",
						Kind:      "response",
						Title:     "生成文案",
						Prompt:    "根据商品信息生成图文带货文案，包含标题、正文、3个短卖点和5个标签。语气：{{field \"tone\"}}。目标人群：{{field \"audience\"}}。商品信息：\n{{field \"product_brief\"}}\n",
						TextModel: "",
					},
					{
						ID:            "images",
						Kind:          "image",
						Title:         "生成配图",
						Prompt:        "根据以下商品信息和文案生成图文带货配图，要求真实、清晰、适合社媒传播。商品信息：\n{{field \"product_brief\"}}\n文案：\n{{stepText \"copy\"}}\n{{with field \"visual_style\"}}视觉风格：{{.}}\n{{end}}",
						Count:         "{{field \"image_count\"}}",
						UseReferences: true,
					},
				},
			},
		},
	}
}

func workflowCatalogByID(id string) (workflowDefinition, bool) {
	id = strings.TrimSpace(id)
	for _, wf := range builtinWorkflowCatalog().Workflows {
		if strings.EqualFold(wf.ID, id) {
			return wf, true
		}
	}
	return workflowDefinition{}, false
}

func (s *webUIServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, builtinWorkflowCatalog())
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
	workflow, ok := workflowCatalogByID(workflowID)
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
