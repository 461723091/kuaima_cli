(function () {
  const q = (selector) => document.querySelector(selector);
  let activePromptRewriteController = null;
  let lastAutoPromptValue = "";

  function setAutoPromptButtonRunning(running) {
    const button = q("#autoPromptButton");
    if (!button) {
      return;
    }
    button.disabled = running;
    button.innerHTML = running
      ? '<span class="spinner small"></span><span>生成中...</span>'
      : "AI改写";
  }

  function setRewriteButtonRunning(button, running) {
    if (!button) {
      return;
    }
    button.disabled = running;
    button.innerHTML = running
      ? '<span class="spinner small"></span><span>生成中...</span>'
      : "AI改写";
  }

  function setAutoPromptButtonVisible(visible) {
    const button = q("#autoPromptButton");
    if (button) {
      button.hidden = !visible;
    }
  }

  function normalizeAutoPromptText(text) {
    let value = String(text || "").trim();
    value = value.replace(/^```(?:text)?\s*/i, "").replace(/\s*```$/, "");
    value = value.replace(/^(提示词|Prompt)\s*[:：]\s*/i, "");
    return value.trim().replace(/\s+/g, " ");
  }

  function updateStatus(message, className) {
    const status = q("#genStatus");
    if (!status) {
      return;
    }
    status.textContent = message;
    status.className = className;
  }

  function syncAutoPromptButtonVisibility() {
    const form = q("#genForm");
    const textarea = form && form.elements && form.elements.prompt;
    if (!textarea) {
      return;
    }
    const value = String(textarea.value || "").trim();
    setAutoPromptButtonVisible(Boolean(value) && value !== lastAutoPromptValue);
  }

  function setAutoPromptValue(text) {
    const form = q("#genForm");
    const textarea = form && form.elements && form.elements.prompt;
    if (!textarea) {
      return "";
    }
    const value = normalizeAutoPromptText(text);
    textarea.value = value;
    lastAutoPromptValue = value;
    syncAutoPromptButtonVisibility();
    return value;
  }

  async function autoWritePrompt() {
    const form = q("#genForm");
    const textarea = form && form.elements && form.elements.prompt;
    if (!textarea) {
      return;
    }
    const source = String(textarea.value || "").trim();
    if (!source) {
      updateStatus("先输入一句简单描述，再自动改写", "status gen-status");
      textarea.focus();
      return;
    }

    activePromptRewriteController = new AbortController();
    setAutoPromptButtonRunning(true);
    updateStatus("正在自动改写提示词...", "status gen-status loading");
    try {
      const formData = new FormData();
      formData.append("prompt", source);
      const files = typeof referenceFilesForSubmit === "function"
        ? await referenceFilesForSubmit(form)
        : [];
      files.forEach((file) => {
        formData.append("images", file, file.name || "reference-image.png");
      });
      await apiStream("/api/prompt/auto?stream=1", {
        method: "POST",
        body: formData,
        signal: activePromptRewriteController.signal,
      }, {
        status(payload) {
          updateStatus(payload.message || "正在自动改写提示词...", "status gen-status loading");
        },
        text(payload) {
          const current = String(payload.text || "");
          if (current) {
            textarea.value = normalizeAutoPromptText(textarea.value + current);
          }
        },
        done(payload) {
          const prompt = setAutoPromptValue(payload.prompt || payload.text || textarea.value);
          if (!prompt) {
            throw new Error("未生成提示词");
          }
          if (typeof saveSettings === "function") {
            saveSettings();
          }
          updateStatus("已自动改写提示词", "status gen-status");
        },
        error(payload) {
          throw new Error(payload.error || "自动改写失败");
        },
      });
      if (!String(textarea.value || "").trim()) {
        throw new Error("未生成提示词");
      }
    } catch (error) {
      if (error && error.name === "AbortError") {
        updateStatus("已取消改写", "status gen-status");
        return;
      }
      updateStatus(error.message, "status gen-status error");
    } finally {
      activePromptRewriteController = null;
      setAutoPromptButtonRunning(false);
    }
  }

  async function rewriteTextarea(textarea, button) {
    if (!textarea) {
      return;
    }
    const source = String(textarea.value || "").trim();
    if (!source) {
      updateStatus("先输入一句简单描述，再自动改写", "status gen-status");
      textarea.focus();
      return;
    }

    activePromptRewriteController = new AbortController();
    setRewriteButtonRunning(button, true);
    updateStatus("正在自动改写提示词...", "status gen-status loading");
    try {
      const formData = new FormData();
      formData.append("prompt", source);
      const form = q("#genForm");
      const files = typeof referenceFilesForSubmit === "function" && form
        ? await referenceFilesForSubmit(form)
        : [];
      files.forEach((file) => {
        formData.append("images", file, file.name || "reference-image.png");
      });
      await apiStream("/api/prompt/auto?stream=1", {
        method: "POST",
        body: formData,
        signal: activePromptRewriteController.signal,
      }, {
        status(payload) {
          updateStatus(payload.message || "正在自动改写提示词...", "status gen-status loading");
        },
        text(payload) {
          const current = String(payload.text || "");
          if (current) {
            textarea.value = normalizeAutoPromptText(textarea.value + current);
            textarea.dispatchEvent(new Event("input", { bubbles: true }));
          }
        },
        done(payload) {
          const value = normalizeAutoPromptText(payload.prompt || payload.text || textarea.value);
          if (!value) {
            throw new Error("未生成提示词");
          }
          textarea.value = value;
          textarea.dispatchEvent(new Event("input", { bubbles: true }));
          textarea.dispatchEvent(new Event("change", { bubbles: true }));
          if (typeof saveSettings === "function") {
            saveSettings();
          }
          updateStatus("已自动改写提示词", "status gen-status");
        },
        error(payload) {
          throw new Error(payload.error || "自动改写失败");
        },
      });
      if (!String(textarea.value || "").trim()) {
        throw new Error("未生成提示词");
      }
    } catch (error) {
      if (error && error.name === "AbortError") {
        updateStatus("已取消改写", "status gen-status");
        return;
      }
      updateStatus(error.message, "status gen-status error");
    } finally {
      activePromptRewriteController = null;
      setRewriteButtonRunning(button, false);
    }
  }

  const button = q("#autoPromptButton");
  if (button) {
    button.addEventListener("click", autoWritePrompt);
  }

  document.addEventListener("click", (event) => {
    const button = event.target.closest("[data-workflow-rewrite]");
    if (!button) {
      return;
    }
    const group = button.closest(".workflow-textarea-group");
    const textarea = group && group.querySelector("textarea[data-workflow-key]");
    rewriteTextarea(textarea, button);
  });

  const form = q("#genForm");
  const textarea = form && form.elements && form.elements.prompt;
  if (textarea) {
    textarea.addEventListener("input", syncAutoPromptButtonVisibility);
    textarea.addEventListener("change", syncAutoPromptButtonVisibility);
    syncAutoPromptButtonVisibility();
  }

  window.KuaimaPromptHelper = {
    sync: syncAutoPromptButtonVisibility,
    setValue: setAutoPromptValue,
    rewriteTextarea,
  };
})();
