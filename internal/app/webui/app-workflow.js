(function () {
  const STORAGE_KEY = "kuaima.webui.workflow.v1";
  const state = {
    catalog: [],
    cache: readWorkflowCache(),
    activeStepId: "",
    pendingReview: null,
  };

  function readWorkflowCache() {
    try {
      const value = JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}");
      return value && typeof value === "object" ? value : {};
    } catch {
      return {};
    }
  }

  function saveWorkflowCache() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state.cache));
    } catch {
      // Ignore storage failures.
    }
  }

  function workflowIdInput() {
    return $("#workflowId");
  }

  function workflowTabs() {
    return $("#workflowTabs");
  }

  function workflowTemplatesInput() {
    return $("#workflowTemplates");
  }

  function workflowDescription() {
    return $("#workflowDescription");
  }

  function workflowFields() {
    return $("#workflowFields");
  }

  function workflowTemplatePicker() {
    return $("#workflowTemplatePicker");
  }

  function promptTextGroup() {
    return $("#promptTextGroup");
  }

  function singleGenerationControls() {
    return $("#singleGenerationControls");
  }

  function workflowSteps() {
    return $("#workflowSteps");
  }

  function workflowReview() {
    return $("#workflowReview");
  }

  function promptFieldLabel() {
    return $("#promptFieldLabel");
  }

  function referenceUploadLabel() {
    return $("#referenceUploadLabel");
  }

  function workflowDef(id) {
    return state.catalog.find((item) => String(item.id || "") === String(id || ""));
  }

  function currentWorkflowId() {
    const input = workflowIdInput();
    return input ? String(input.value || "single") : "single";
  }

  function workflowLabel(id) {
    if (String(id || "") === "single") {
      return "图片生成";
    }
    const def = workflowDef(id);
    return def ? def.name : String(id || "");
  }

  function currentWorkflowValues() {
    const fields = workflowFields();
    const values = {};
    if (!fields || fields.hidden) {
      const templates = currentWorkflowTemplateIDs();
      if (templates.length > 0) {
        values.templates = templates.join(",");
      }
      return values;
    }
    fields.querySelectorAll("[data-workflow-key]").forEach((input) => {
      const key = input.dataset.workflowKey;
      values[key] = String(input.value || "").trim();
    });
    const templates = currentWorkflowTemplateIDs();
    if (templates.length > 0) {
      values.templates = templates.join(",");
    }
    return values;
  }

  function renderWorkflowTabs() {
    const container = workflowTabs();
    if (!container) {
      return;
    }
    const current = currentWorkflowId();
    const agentPane = document.querySelector('[data-mode-pane="agent"]');
    const agentActive = Boolean(agentPane && !agentPane.hidden);
    const items = [{ id: "single", name: "图片生成" }];
    const workflowButtons = items.map((item) => (
      '<button class="workflow-tab' + (!agentActive && String(item.id) === String(current) ? " active" : "") + '" type="button" role="tab" ' +
      'aria-selected="' + String(!agentActive && String(item.id) === String(current)) + '" data-mode-tab="generate" data-workflow-id="' + esc(item.id) + '">' +
      iconHTML("image-plus") + '<span>' + esc(item.name) + '</span></button>'
    )).join("");
    const catalogButtons = state.catalog.map((item) => (
      '<button class="workflow-tab' + (!agentActive && String(item.id) === String(current) ? " active" : "") + '" type="button" role="tab" ' +
      'aria-selected="' + String(!agentActive && String(item.id) === String(current)) + '" data-mode-tab="generate" data-workflow-id="' + esc(item.id) + '">' +
      iconHTML("workflow") + '<span>' + esc(item.name) + '</span></button>'
    )).join("");
    container.innerHTML = workflowButtons +
      '<button class="workflow-tab' + (agentActive ? " active" : "") + '" type="button" role="tab" aria-selected="' +
      String(agentActive) + '" data-mode-tab="agent">' + iconHTML("bot-message-square") + '<span>Agent 模式</span></button>' +
      catalogButtons;
  }

  function buildInputValue(field, cached) {
    if (cached != null && String(cached).trim() !== "") {
      return String(cached);
    }
    if (field && field.default != null && String(field.default).trim() !== "") {
      return String(field.default);
    }
    return "";
  }

  function renderWorkflowFields(def) {
    const container = workflowFields();
    const description = workflowDescription();
    if (!container || !description) {
      return;
    }
    if (!def || String(def.id || "") === "single") {
      container.hidden = true;
      container.innerHTML = "";
      description.textContent = "图片生成";
      return;
    }

    const cached = state.cache[def.id] || {};
    description.textContent = def.description || "";
    container.hidden = false;
    container.innerHTML = (def.inputs || []).map((field) => {
      const name = "workflow_" + field.key;
      const value = buildInputValue(field, cached[field.key]);
      const help = field.help ? '<div class="workflow-help">' + esc(field.help) + '</div>' : "";
      let control = "";
      if (field.type === "textarea") {
        control = '<div class="workflow-textarea-group">' +
          '<textarea data-workflow-key="' + esc(field.key) + '" name="' + esc(name) + '" placeholder="' +
          esc(field.placeholder || "") + '"' + (field.required ? " required" : "") + ">" + esc(value) + "</textarea>" +
          '<button class="btn small workflow-rewrite" type="button" data-workflow-rewrite>' +
          iconHTML("wand-sparkles") + '<span>AI改写</span></button>' +
          '</div>';
      } else if (field.type === "combo") {
        control = '<div class="workflow-combo" data-workflow-combo>' +
          '<div class="workflow-combo-control">' +
          '<input data-workflow-key="' + esc(field.key) + '" name="' + esc(name) + '" type="text" value="' +
          esc(value) + '" placeholder="' + esc(field.placeholder || "") + '"' +
          (field.required ? " required" : "") + ' autocomplete="off">' +
          '<button class="workflow-combo-toggle" type="button" aria-label="展开选项" data-workflow-combo-toggle>v</button>' +
          '</div>' +
          '<div class="workflow-combo-menu" data-workflow-combo-menu hidden>' +
          (field.options || []).map((option) => (
            '<button type="button" data-workflow-combo-option="' + esc(option.value) + '">' +
            esc(option.label || option.value) + '</button>'
          )).join("") + "</div></div>";
      } else if (field.type === "select") {
        control = '<select data-workflow-key="' + esc(field.key) + '" name="' + esc(name) + '">' +
          (field.options || []).map((option) => (
            '<option value="' + esc(option.value) + '"' + (String(option.value) === String(value) ? " selected" : "") + '>' +
            esc(option.label || option.value) + "</option>"
          )).join("") + "</select>";
      } else if (field.type === "number") {
        control = '<input data-workflow-key="' + esc(field.key) + '" name="' + esc(name) + '" type="number" value="' +
          esc(value) + '"' +
          (field.min != null ? ' min="' + esc(field.min) + '"' : "") +
          (field.max != null ? ' max="' + esc(field.max) + '"' : "") +
          (field.required ? " required" : "") +
          '>';
      } else {
        control = '<input data-workflow-key="' + esc(field.key) + '" name="' + esc(name) + '" type="' +
          esc(field.type || "text") + '" value="' + esc(value) + '" placeholder="' + esc(field.placeholder || "") + '"' +
          (field.required ? " required" : "") + ">";
      }
      return '<div class="workflow-input">' +
        '<label>' + esc(field.label || field.key) + (field.required ? '<span class="workflow-required">*</span>' : "") + "</label>" +
        control +
        help +
        "</div>";
    }).join("");
  }

  function workflowImageSteps(def) {
    return (def && Array.isArray(def.steps) ? def.steps : []).filter((step) => (
      String(step.kind || "").toLowerCase() === "image"
    ));
  }

  function workflowResponseSteps(def) {
    return (def && Array.isArray(def.steps) ? def.steps : []).filter((step) => (
      String(step.kind || "").toLowerCase() === "response"
    ));
  }

  function workflowNeedsReview(def) {
    let seenResponse = false;
    for (const step of (def && Array.isArray(def.steps) ? def.steps : [])) {
      const kind = String(step.kind || "").toLowerCase();
      if (kind === "response") {
        seenResponse = true;
      } else if (kind === "image" && seenResponse) {
        return true;
      }
    }
    return false;
  }

  function workflowDefaultTemplateIDs(def) {
    return workflowImageSteps(def).map((step) => String(step.id || "").trim()).filter(Boolean);
  }

  function currentWorkflowTemplateIDs() {
    const input = workflowTemplatesInput();
    const def = workflowDef(currentWorkflowId());
    const defaults = workflowDefaultTemplateIDs(def);
    const raw = input ? String(input.value || "").trim() : "";
    if (!raw) {
      return defaults;
    }
    const allowed = new Set(defaults);
    const selected = raw.split(",").map((item) => String(item || "").trim()).filter((item) => item && allowed.has(item));
    return selected.length > 0 ? selected : defaults;
  }

  function setWorkflowTemplateIDs(ids) {
    const input = workflowTemplatesInput();
    const def = workflowDef(currentWorkflowId());
    const allowed = new Set(workflowImageSteps(def).map((step) => String(step.id || "").trim()).filter(Boolean));
    const values = Array.isArray(ids) ? ids.map((item) => String(item || "").trim()).filter((item) => item && allowed.has(item)) : [];
    const finalValues = values.length > 0 ? values : Array.from(allowed);
    if (input) {
      input.value = finalValues.join(",");
    }
    const container = workflowTemplatePicker();
    if (container) {
      container.querySelectorAll("[data-workflow-template]").forEach((box) => {
        const selected = finalValues.includes(box.dataset.workflowTemplate || "");
        box.checked = selected;
        const card = box.closest(".workflow-template-card");
        if (card) {
          card.classList.toggle("active", selected);
        }
      });
    }
  }

  function renderWorkflowTemplates(def) {
    const container = workflowTemplatePicker();
    if (!container) {
      return;
    }
    const templates = workflowImageSteps(def);
    if (!def || String(def.id || "") === "single" || templates.length === 0) {
      container.hidden = true;
      container.innerHTML = "";
      return;
    }
    container.hidden = false;
    container.innerHTML = '<div class="workflow-template-grid">' + templates.map((step) => (
      '<label class="workflow-template-card" data-workflow-template-card="' + esc(step.id) + '">' +
      '<input type="checkbox" data-workflow-template="' + esc(step.id) + '" checked>' +
      '<div><strong>' + esc(step.title || step.id || "模板") + '</strong>' +
      '<p>' + esc(workflowStepMeta(step)) + '</p></div>' +
      '</label>'
    )).join("") + "</div>";
    setWorkflowTemplateIDs(currentWorkflowTemplateIDs());
  }

  function workflowStepMeta(step) {
    const title = String(step && step.title || "");
    const type = title.includes("主图") ? "主图 1:1" : title.includes("详情图") ? "详情图 3:4" : "图片";
    return type + " · 生成 1 张";
  }

  function syncWorkflowMode() {
    const id = currentWorkflowId();
    const def = workflowDef(id);
    renderWorkflowFields(def || { id: "single" });
    renderWorkflowTemplates(def || { id: "single" });
    const promptGroup = promptTextGroup();
    const promptTextarea = $("#genForm") && $("#genForm").elements && $("#genForm").elements.prompt;
    if (promptGroup) {
      promptGroup.hidden = String(id || "") !== "single";
    }
    if (promptTextarea) {
      promptTextarea.required = String(id || "") === "single";
      promptTextarea.disabled = String(id || "") !== "single";
    }
    const promptLabel = promptFieldLabel();
    if (promptLabel) {
      promptLabel.textContent = String(id || "") === "single" ? "提示词" : "产品参考图片";
    }
    const uploadLabel = referenceUploadLabel();
    if (uploadLabel) {
      uploadLabel.textContent = String(id || "") === "single" ? "参考图" : "产品参考图";
    }
    const controls = singleGenerationControls();
    if (controls) {
      controls.hidden = String(id || "") !== "single";
    }
    const review = workflowReview();
    if (review) {
      review.hidden = true;
      review.innerHTML = "";
    }
    state.pendingReview = null;
    const submit = $("#genForm") && $("#genForm").querySelector('button[type="submit"]');
    if (submit) {
      submit.innerHTML = iconHTML(String(id || "") === "single" ? "image-plus" : "workflow") +
        '<span>' + (String(id || "") === "single" ? "生成图片" : "运行工作流") + '</span>';
    }
    renderWorkflowTabs();
  }

  function persistCurrentValues(options = {}) {
    const id = currentWorkflowId();
    if (String(id || "") === "single") {
      return;
    }
    state.cache[id] = currentWorkflowValues();
    if (!options.keepActiveId) {
      state.cache.activeWorkflowId = id;
    }
    saveWorkflowCache();
  }

  function setWorkflow(id, values) {
    const input = workflowIdInput();
    if (!input) {
      return;
    }
    input.value = String(id || "single");
    const def = workflowDef(input.value);
    renderWorkflowTabs();
    renderWorkflowFields(def || { id: "single" });
    const container = workflowFields();
    if (container && values) {
      container.querySelectorAll("[data-workflow-key]").forEach((input) => {
        const key = input.dataset.workflowKey;
        if (values[key] != null) {
          input.value = values[key];
        }
      });
    }
    const templateValues = String(values && (values.templates || values.workflow_templates) || "").trim();
    if (templateValues) {
      setWorkflowTemplateIDs(templateValues.split(","));
    } else {
      setWorkflowTemplateIDs(currentWorkflowTemplateIDs());
    }
    syncWorkflowMode();
  }

  function renderWorkflowSteps(result) {
    const container = workflowSteps();
    if (!container) {
      return;
    }
    const steps = Array.isArray(result && result.steps) ? result.steps : [];
    if (steps.length === 0) {
      container.hidden = true;
      container.innerHTML = "";
      return;
    }
    container.hidden = false;
    container.innerHTML = steps.map((step) => {
      const images = Array.isArray(step.images) ? step.images : [];
      const text = String(step.text || "").trim();
      return '<section class="workflow-step">' +
        '<div class="workflow-step-head">' +
        '<strong>' + esc(step.title || step.id || "步骤") + "</strong>" +
        '<span class="workflow-step-kind">' + esc(step.kind || "") + "</span>" +
        "</div>" +
        (text ? '<pre class="workflow-step-text">' + esc(text) + "</pre>" : "") +
        (images.length ? '<div class="workflow-step-images">' + images.map((url) => (
          '<button class="workflow-step-image" type="button" data-preview-image="' + esc(url) + '">' +
          '<img src="' + esc(url) + '" alt="' + esc(step.title || "workflow image") + '">' +
          "</button>"
        )).join("") + "</div>" : "") +
        "</section>";
    }).join("");
  }

  function currentWorkflowReviewTexts() {
    const review = workflowReview();
    const values = {};
    if (!review) {
      return values;
    }
    review.querySelectorAll("[data-workflow-review-step]").forEach((input) => {
      const key = String(input.dataset.workflowReviewStep || "").trim();
      if (key) {
        values[key] = String(input.value || "").trim();
      }
    });
    return values;
  }

  function renderWorkflowReview(result) {
    const container = workflowReview();
    if (!container) {
      return;
    }
    const steps = Array.isArray(result && result.steps) ? result.steps.filter((step) => String(step.kind || "").toLowerCase() === "response") : [];
    if (!result || !result.needs_review || steps.length === 0) {
      container.hidden = true;
      container.innerHTML = "";
      return;
    }
    container.hidden = false;
    container.innerHTML = (
      '<div class="workflow-review-head">' +
      '<strong>确认文案后继续</strong>' +
      '<span>可直接调整卖点、痛点、参数等内容，再开始后续生图。</span>' +
      '</div>' +
      steps.map((step) => {
        const value = String(step.text || "");
        return (
          '<label class="workflow-review-item">' +
          '<span>' + esc(step.title || step.id || "文案") + '</span>' +
          '<textarea data-workflow-review-step="' + esc(step.id || "") + '">' + esc(value) + '</textarea>' +
          '</label>'
        );
      }).join("") +
      '<div class="workflow-review-actions">' +
      '<button class="btn primary" type="button" data-workflow-review-action="continue">' +
      iconHTML("circle-check") + '<span>确认并继续</span></button>' +
      '</div>'
    );
  }

  async function loadCatalog() {
    try {
      const data = await api("/api/workflows");
      state.catalog = Array.isArray(data.workflows) ? data.workflows : [];
    } catch (error) {
      console.warn("workflow catalog load failed", error);
      state.catalog = [];
    }
    renderWorkflowTabs();
    const activeId = state.cache.activeWorkflowId || "single";
    const values = state.cache[activeId] || {};
    setWorkflow(activeId, values);
    state.cache.activeWorkflowId = activeId;
    saveWorkflowCache();
  }

  async function generateWorkflowStream(form, signal, options = {}) {
    const result = { workflow_id: currentWorkflowId(), workflow_name: workflowLabel(currentWorkflowId()), prompt: "", params: {}, steps: [], images: [], saved: [], text: "", timing: null, needs_review: false };
    const startedAt = performance.now();
    let firstResponseAt = 0;
    let activeStep = null;
    const markFirstResponse = () => {
      if (!firstResponseAt) {
        firstResponseAt = performance.now();
      }
    };

    setResultsVisible(true);
    workflowSteps().hidden = false;
    workflowSteps().innerHTML = '<div class="generation-placeholder"><span class="spinner"></span><span>正在执行工作流...</span></div>';
    const review = workflowReview();
    if (review) {
      review.hidden = true;
      review.innerHTML = "";
    }
    $("#gallery").classList.remove("empty");
    $("#gallery").innerHTML = '<div class="generation-placeholder"><span class="spinner"></span><span>正在等待步骤输出...</span></div>';
    renderResultTiming(null);

    const formData = await buildGenerateFormData(form);
    const mode = String(options.mode || "").trim();
    if (mode) {
      formData.set("workflow_mode", mode);
    }
    const stepTexts = options.stepTexts && typeof options.stepTexts === "object" ? options.stepTexts : null;
    if (stepTexts) {
      formData.set("workflow_step_texts", JSON.stringify(stepTexts));
    }
    await apiStream("/api/workflow/run?stream=1", {
      method: "POST",
      body: formData,
      signal,
    }, {
      status(payload) {
        $("#genStatus").textContent = payload.message || "工作流运行中...";
      },
      step(payload) {
        markFirstResponse();
        activeStep = {
          id: payload.id || "",
          kind: payload.kind || "",
          title: payload.title || "",
          text: "",
          images: [],
          saved: [],
        };
        result.steps.push(activeStep);
        renderWorkflowSteps(result);
      },
      text(payload) {
        markFirstResponse();
        const text = String(payload.text || "");
        if (!text) {
          return;
        }
        if (activeStep) {
          activeStep.text += text;
        } else {
          result.text += text;
        }
        renderWorkflowSteps(result);
      },
      image(payload) {
        markFirstResponse();
        const url = String(payload.url || "");
        if (!url) {
          return;
        }
        if (!result.images.includes(url)) {
          result.images.push(url);
        }
        const stepId = String(payload.step_id || "");
        const target = stepId && result.steps.find((item) => item.id === stepId) || activeStep;
        if (target) {
          if (!target.images.includes(url)) {
            target.images.push(url);
          }
        }
        renderWorkflowSteps(result);
        renderGalleryImages(result.images);
      },
      done(payload) {
        markFirstResponse();
        result.workflow_id = payload.workflow_id || result.workflow_id;
        result.workflow_name = payload.workflow_name || result.workflow_name;
        result.prompt = payload.prompt || "";
        result.params = payload.params || {};
        result.steps = Array.isArray(payload.steps) ? payload.steps : result.steps;
        result.images = Array.isArray(payload.images) ? payload.images : result.images;
        result.saved = Array.isArray(payload.saved) ? payload.saved : result.saved;
        result.text = payload.text || result.text;
        result.needs_review = Boolean(payload.needs_review);
        renderWorkflowSteps(result);
        if (result.needs_review) {
          $("#gallery").classList.add("empty");
          $("#gallery").innerHTML = '<div class="status">文案已生成，请确认后继续</div>';
        } else {
          renderGalleryImages(result.images);
        }
        renderWorkflowReview(result);
      },
      error(payload) {
        throw new Error(payload.error || "工作流运行失败");
      },
    });

    const finishedAt = performance.now();
    result.timing = {
      response_ms: Math.round((firstResponseAt || finishedAt) - startedAt),
      total_ms: Math.round(finishedAt - startedAt),
    };
    renderResultTiming(result.timing);
    return result;
  }

  async function runWorkflowSubmission(form, options = {}) {
    const submitButton = form.querySelector('button[type="submit"]');
    const cancelButton = $("#cancelGenerate");
    activeGenerationController = new AbortController();
    const isReviewContinue = String(options.mode || "") === "final" && options.stepTexts && Object.keys(options.stepTexts).length > 0;
    const runningLabel = isReviewContinue ? "继续生成" : "运行工作流";
    if (submitButton) {
      submitButton.disabled = true;
      submitButton.innerHTML = '<span class="spinner small"></span><span>' + runningLabel + '...</span>';
    }
    cancelButton.hidden = false;
    $("#genStatus").textContent = runningLabel + "...";
    $("#genStatus").className = "status gen-status loading";
    try {
      const result = await generateWorkflowStream(form, activeGenerationController.signal, options);
      if (result.needs_review) {
        state.pendingReview = result;
        $("#genStatus").textContent = "文案已生成，请确认后继续";
        $("#genStatus").className = "status gen-status";
        return result;
      }
      state.pendingReview = null;
      const historyError = await saveHistorySafely(result);
      $("#genStatus").textContent = "已保存" + result.saved.length + " 张" +
        (historyError ? "，历史保存失败：" + historyError : "");
      $("#genStatus").className = "status gen-status";
      loadBalance();
      return result;
    } catch (error) {
      const canceled = error && error.name === "AbortError";
      if ($("#gallery").querySelector(".generation-placeholder")) {
        renderGalleryMessage(canceled ? "已取消工作流" : "工作流失败：" + error.message, canceled ? "status" : "status error");
      }
      if (canceled) {
        $("#genStatus").textContent = "已取消工作流";
        $("#genStatus").className = "status gen-status";
        return null;
      }
      await saveHistorySafely({ images: [] }, { status: "failed", error: error.message });
      handleOperationError(error, $("#genStatus"));
      return null;
    } finally {
      activeGenerationController = null;
      cancelButton.hidden = true;
      if (submitButton) {
        submitButton.disabled = false;
        submitButton.innerHTML = iconHTML(String(currentWorkflowId() || "") === "single" ? "image-plus" : "workflow") +
          '<span>' + (String(currentWorkflowId() || "") === "single" ? "生成图片" : "运行工作流") + '</span>';
      }
    }
  }

  async function continueWorkflowReview() {
    const form = $("#genForm");
    if (!form || !state.pendingReview) {
      return;
    }
    updateSize();
    syncImageInput();
    saveSettings();
    persistCurrentValues();
    const stepTexts = currentWorkflowReviewTexts();
    await runWorkflowSubmission(form, {
      mode: "final",
      stepTexts,
    });
  }

  function restoreWorkflow(workflowId, values) {
    setWorkflow(workflowId || "single", values || {});
  }

  function init() {
    const tabs = workflowTabs();
    if (!tabs) {
      return;
    }
    tabs.addEventListener("click", (event) => {
      const button = event.target.closest("[data-workflow-id]");
      if (!button) {
        return;
      }
      const id = String(button.dataset.workflowId || "single");
      if (id === currentWorkflowId()) {
        return;
      }
      persistCurrentValues({ keepActiveId: true });
      setWorkflow(id, state.cache[id] || {});
      state.cache.activeWorkflowId = id;
      saveWorkflowCache();
    });
    const fields = workflowFields();
    if (fields) {
      fields.addEventListener("input", persistCurrentValues);
      fields.addEventListener("change", persistCurrentValues);
      fields.addEventListener("click", (event) => {
        const toggle = event.target.closest("[data-workflow-combo-toggle]");
        if (toggle) {
          const combo = toggle.closest("[data-workflow-combo]");
          const menu = combo && combo.querySelector("[data-workflow-combo-menu]");
          if (menu) {
            menu.hidden = !menu.hidden;
          }
          return;
        }
        const option = event.target.closest("[data-workflow-combo-option]");
        if (!option) {
          return;
        }
        const combo = option.closest("[data-workflow-combo]");
        const input = combo && combo.querySelector("[data-workflow-key]");
        const menu = combo && combo.querySelector("[data-workflow-combo-menu]");
        if (input) {
          input.value = option.dataset.workflowComboOption || "";
          input.dispatchEvent(new Event("input", { bubbles: true }));
          input.dispatchEvent(new Event("change", { bubbles: true }));
          input.focus();
        }
        if (menu) {
          menu.hidden = true;
        }
      });
      fields.addEventListener("input", (event) => {
        const input = event.target.closest("[data-workflow-combo] [data-workflow-key]");
        if (!input) {
          return;
        }
        const combo = input.closest("[data-workflow-combo]");
        const menu = combo && combo.querySelector("[data-workflow-combo-menu]");
        if (!menu) {
          return;
        }
        const needle = String(input.value || "").trim().toLowerCase();
        menu.hidden = false;
        menu.querySelectorAll("[data-workflow-combo-option]").forEach((option) => {
          option.hidden = Boolean(needle) && !String(option.textContent || "").toLowerCase().includes(needle) &&
            !String(option.dataset.workflowComboOption || "").toLowerCase().includes(needle);
        });
      });
    }
    document.addEventListener("click", (event) => {
      if (event.target.closest("[data-workflow-combo]")) {
        return;
      }
      document.querySelectorAll("[data-workflow-combo-menu]").forEach((menu) => {
        menu.hidden = true;
      });
    });
    const templatePicker = workflowTemplatePicker();
    if (templatePicker) {
      templatePicker.addEventListener("change", () => {
        const selected = Array.from(templatePicker.querySelectorAll("[data-workflow-template]"))
          .filter((input) => input.checked)
          .map((input) => String(input.dataset.workflowTemplate || "").trim())
          .filter(Boolean);
        setWorkflowTemplateIDs(selected.length > 0 ? selected : workflowDefaultTemplateIDs(workflowDef(currentWorkflowId())));
        persistCurrentValues();
      });
    }
    const review = workflowReview();
    if (review) {
      review.addEventListener("click", async (event) => {
        const button = event.target.closest("[data-workflow-review-action]");
        if (!button) {
          return;
        }
        if (button.dataset.workflowReviewAction === "continue") {
          event.preventDefault();
          await continueWorkflowReview();
        }
      });
    }
    const form = $("#genForm");
    if (form) {
      form.addEventListener("submit", async (event) => {
        if (currentWorkflowId() === "single") {
          return;
        }
        event.preventDefault();
        event.stopImmediatePropagation();
        setWorkflowTemplateIDs(currentWorkflowTemplateIDs());
        updateSize();
        syncImageInput();
        saveSettings();
        persistCurrentValues();
        const def = workflowDef(currentWorkflowId());
        const mode = workflowNeedsReview(def) ? "draft" : "final";
        await runWorkflowSubmission(form, { mode });
      }, true);
    }
    loadCatalog();
  }

  window.KuaimaWorkflow = {
    init,
    currentValues: currentWorkflowValues,
    currentWorkflowId,
    currentWorkflowLabel: workflowLabel,
    generateWorkflowStream,
    restore: restoreWorkflow,
    sync: syncWorkflowMode,
    renderSteps: renderWorkflowSteps,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init, { once: true });
  } else {
    init();
  }
})();
