(function () {
  const q = (selector) => document.querySelector(selector);
  let activePromptRewriteController = null;

  function setAutoPromptButtonRunning(running) {
    const button = q("#autoPromptButton");
    if (!button) {
      return;
    }
    button.disabled = running;
    button.innerHTML = running
      ? '<span class="spinner small"></span><span>改写中...</span>'
      : "自动写";
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
      const result = await api("/api/prompt/auto", {
        method: "POST",
        body: formData,
        signal: activePromptRewriteController.signal,
      });
      const prompt = normalizeAutoPromptText(result.prompt);
      if (!prompt) {
        throw new Error("未生成提示词");
      }
      textarea.value = prompt;
      if (typeof saveSettings === "function") {
        saveSettings();
      }
      updateStatus("已自动改写提示词", "status gen-status");
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

  const button = q("#autoPromptButton");
  if (button) {
    button.addEventListener("click", autoWritePrompt);
  }
})();
