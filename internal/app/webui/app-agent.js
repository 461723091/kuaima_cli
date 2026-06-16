(function () {
  const $ = (selector) => document.querySelector(selector);

  const state = {
    history: [],
    attachments: [],
    activeController: null,
    pendingAssistant: null,
  };

  function renderMode(mode) {
    document.querySelectorAll("[data-mode-tab]").forEach((button) => {
      const active = button.dataset.modeTab === mode;
      button.classList.toggle("active", active);
      button.setAttribute("aria-selected", String(active));
    });
    document.querySelectorAll("[data-mode-pane]").forEach((pane) => {
      pane.hidden = pane.dataset.modePane !== mode;
    });
  }

  function renderAttachmentList() {
    const list = $("#agentAttachmentList");
    if (!list) {
      return;
    }
    if (state.attachments.length === 0) {
      list.innerHTML = "";
      return;
    }
    list.innerHTML = state.attachments.map((item, index) => (
      '<div class="agent-attachment">' +
        '<img src="' + esc(item.url) + '" alt="' + esc(item.name || "image") + '">' +
        '<button type="button" class="icon-btn" data-agent-remove-attachment="' + index + '" aria-label="移除图片">×</button>' +
      '</div>'
    )).join("");
  }

  function renderChat() {
    const log = $("#agentChatLog");
    if (!log) {
      return;
    }
    if (state.history.length === 0) {
      log.innerHTML = "";
      return;
    }
    log.innerHTML = state.history.map((item, index) => renderMessageHTML(item, index)).join("");
    log.scrollTop = log.scrollHeight;
  }

  function renderMessageHTML(message, index) {
    const role = String(message.role || "assistant");
    const parts = Array.isArray(message.content) ? message.content : [];
    const textParts = parts.filter((part) => part && part.type === "input_text" && part.text).map((part) => part.text);
    const imageParts = parts.filter((part) => part && part.type === "input_image" && part.image_url);
    return (
      '<div class="chat-message ' + esc(role) + '" data-message-index="' + index + '">' +
        '<div class="chat-role">' + esc(role === "assistant" ? "assistant" : role) + '</div>' +
        (textParts.length ? '<div class="chat-text">' + esc(textParts.join("\n")) + '</div>' : "") +
        (imageParts.length ? '<div class="chat-images">' + imageParts.map((part) => (
          '<div class="chat-image"><img src="' + esc(part.image_url) + '" alt="image"></div>'
        )).join("") + '</div>' : "") +
      '</div>'
    );
  }

  function appendPendingAssistant() {
    const log = $("#agentChatLog");
    if (!log) {
      return null;
    }
    const el = document.createElement("div");
    el.className = "chat-message assistant";
    el.innerHTML = '<div class="chat-role">assistant</div><div class="chat-text"></div>';
    log.appendChild(el);
    log.scrollTop = log.scrollHeight;
    return el;
  }

  function setPendingAssistantText(text) {
    if (!state.pendingAssistant) {
      state.pendingAssistant = appendPendingAssistant();
    }
    if (!state.pendingAssistant) {
      return;
    }
    const textEl = state.pendingAssistant.querySelector(".chat-text");
    if (textEl) {
      textEl.textContent = text || "";
    }
    const log = $("#agentChatLog");
    if (log) {
      log.scrollTop = log.scrollHeight;
    }
  }

  function clearAttachments() {
    state.attachments = [];
    const input = $("#agentImageInput");
    if (input) {
      input.value = "";
    }
    renderAttachmentList();
  }

  async function fileToDataURL(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ""));
      reader.onerror = () => reject(reader.error || new Error("failed to read image"));
      reader.readAsDataURL(file);
    });
  }

  async function addAttachments(files) {
    const list = Array.from(files || []);
    if (list.length === 0) {
      return;
    }
    const mapped = [];
    for (const file of list) {
      if (!file || !file.type || !file.type.startsWith("image/")) {
        continue;
      }
      mapped.push({
        name: file.name || "image",
        url: await fileToDataURL(file),
      });
    }
    state.attachments = state.attachments.concat(mapped);
    renderAttachmentList();
  }

  function buildUserMessage(text) {
    const content = [];
    if (text) {
      content.push({ type: "input_text", text });
    }
    state.attachments.forEach((item) => {
      content.push({ type: "input_image", image_url: item.url });
    });
    return { role: "user", content };
  }

  function appendMessage(message) {
    const log = $("#agentChatLog");
    if (!log) {
      return;
    }
    const holder = document.createElement("div");
    holder.innerHTML = renderMessageHTML(message, Math.max(state.history.length - 1, 0));
    log.appendChild(holder.firstElementChild);
    log.scrollTop = log.scrollHeight;
  }

  function setStreaming(streaming) {
    const cancel = $("#cancelAgentSend");
    const send = $("#agentForm button[type=\"submit\"]");
    if (cancel) {
      cancel.hidden = !streaming;
    }
    if (send) {
      send.disabled = streaming;
    }
  }

  async function submitAgentMessage(event) {
    event.preventDefault();
    if (state.activeController) {
      return;
    }
    const prompt = ($("#agentPrompt") && $("#agentPrompt").value || "").trim();
    if (!prompt && state.attachments.length === 0) {
      return;
    }

    const userMessage = buildUserMessage(prompt);
    state.history.push(userMessage);
    appendMessage(userMessage);
    $("#agentPrompt").value = "";
    clearAttachments();

    const assistantMessage = { role: "assistant", content: [] };
    state.pendingAssistant = appendPendingAssistant();
    const controller = new AbortController();
    state.activeController = controller;
    setStreaming(true);

    let responseText = "";
    try {
      await apiStream("/api/agent/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ messages: state.history }),
        signal: controller.signal,
      }, {
        status(payload) {
          setPendingAssistantText(payload.message || "正在回复...");
        },
        text(payload) {
          if (payload.text) {
            responseText += payload.text;
            setPendingAssistantText(responseText);
          }
        },
        done(payload) {
          if (payload && payload.text) {
            responseText = payload.text;
            setPendingAssistantText(responseText);
          }
        },
        error(payload) {
          throw new Error(payload.error || "agent failed");
        },
      });
      assistantMessage.content = responseText ? [{ type: "input_text", text: responseText }] : [];
      state.history.push(assistantMessage);
    } catch (error) {
      if (error && error.name === "AbortError") {
        if (state.pendingAssistant) {
          state.pendingAssistant.remove();
        }
      } else {
        setPendingAssistantText("错误: " + (error && error.message ? error.message : "agent failed"));
      }
    } finally {
      state.pendingAssistant = null;
      state.activeController = null;
      setStreaming(false);
    }
  }

  document.querySelectorAll("[data-mode-tab]").forEach((button) => {
    button.addEventListener("click", () => renderMode(button.dataset.modeTab));
  });

  $("#agentImageInput").addEventListener("change", async (event) => {
    await addAttachments(Array.from(event.target.files || []));
  });

  $("#agentAttachmentList").addEventListener("click", (event) => {
    const button = event.target.closest("[data-agent-remove-attachment]");
    if (!button) {
      return;
    }
    const index = Number(button.dataset.agentRemoveAttachment);
    if (!Number.isFinite(index) || index < 0 || index >= state.attachments.length) {
      return;
    }
    state.attachments.splice(index, 1);
    renderAttachmentList();
  });

  $("#agentForm").addEventListener("submit", submitAgentMessage);

  $("#clearAgentChat").addEventListener("click", () => {
    if (state.activeController) {
      state.activeController.abort();
    }
    state.history = [];
    state.pendingAssistant = null;
    state.activeController = null;
    setStreaming(false);
    clearAttachments();
    $("#agentPrompt").value = "";
    renderChat();
  });

  $("#cancelAgentSend").addEventListener("click", () => {
    if (state.activeController) {
      state.activeController.abort();
    }
  });

  $("#agentPrompt").addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      $("#agentForm").requestSubmit();
    }
  });

  renderMode("generate");
  renderAttachmentList();
  renderChat();
})();
