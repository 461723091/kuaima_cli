(function () {
  const state = {
    file: null,
    dirty: false,
    enabled: false,
    drawing: false,
    mode: "paint",
    zoom: 1,
    width: 0,
    height: 0,
  };

  const $ = (selector) => document.querySelector(selector);

  function elements() {
    return {
      root: $("#maskEditor"),
      imageCanvas: $("#maskImageCanvas"),
      paintCanvas: $("#maskPaintCanvas"),
      brush: $("#maskBrushSize"),
      zoom: $("#maskZoom"),
      clear: $("#clearMask"),
      close: $("#closeMaskEditor"),
      disable: $("#disableMask"),
      hint: $("#maskEditorHint"),
      modeButtons: document.querySelectorAll("[data-mask-mode]"),
    };
  }

  function emitChange() {
    document.dispatchEvent(new CustomEvent("kuaima-mask-change"));
  }

  function applyZoom() {
    const { imageCanvas, paintCanvas } = elements();
    if (!state.width || !state.height) {
      return;
    }
    const displayWidth = Math.round(state.width * state.zoom);
    const displayHeight = Math.round(state.height * state.zoom);
    [imageCanvas, paintCanvas].forEach((canvas) => {
      canvas.style.width = displayWidth + "px";
      canvas.style.height = displayHeight + "px";
    });
  }

  function setCanvasSize(canvas, width, height) {
    canvas.width = width;
    canvas.height = height;
    canvas.style.aspectRatio = width + " / " + height;
  }

  function drawBaseImage(img) {
    const { imageCanvas, paintCanvas } = elements();
    state.width = img.naturalWidth || img.width;
    state.height = img.naturalHeight || img.height;
    setCanvasSize(imageCanvas, state.width, state.height);
    setCanvasSize(paintCanvas, state.width, state.height);

    const imageCtx = imageCanvas.getContext("2d");
    imageCtx.clearRect(0, 0, state.width, state.height);
    imageCtx.drawImage(img, 0, 0, state.width, state.height);

    const paintCtx = paintCanvas.getContext("2d");
    paintCtx.clearRect(0, 0, state.width, state.height);
    applyZoom();
  }

  function loadReference(file) {
    const { hint } = elements();
    if (!file) {
      state.file = null;
      state.dirty = false;
      state.enabled = false;
      return;
    }
    if (state.file === file) {
      return;
    }

    state.file = file;
    state.dirty = false;
    hint.textContent = "正在读取参考图...";

    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      drawBaseImage(img);
      hint.textContent = "涂抹需要重绘的区域";
      emitChange();
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      hint.textContent = "参考图读取失败";
    };
    img.src = url;
  }

  function open(file) {
    const { root } = elements();
    if (!file) {
      return;
    }
    state.enabled = true;
    root.hidden = false;
    loadReference(file);
    emitChange();
  }

  function close() {
    const { root } = elements();
    state.drawing = false;
    root.hidden = true;
  }

  function disableMask() {
    clearMask();
    state.enabled = false;
    close();
    emitChange();
  }

  function pointerPosition(event) {
    const { paintCanvas } = elements();
    const rect = paintCanvas.getBoundingClientRect();
    return {
      x: (event.clientX - rect.left) * paintCanvas.width / rect.width,
      y: (event.clientY - rect.top) * paintCanvas.height / rect.height,
    };
  }

  function brushSize() {
    const { brush } = elements();
    return Math.max(1, Number(brush.value) || 36);
  }

  function paintAt(event) {
    const { paintCanvas } = elements();
    const wasMasked = hasMask();
    const point = pointerPosition(event);
    const ctx = paintCanvas.getContext("2d");
    ctx.save();
    ctx.globalCompositeOperation = state.mode === "erase" ? "destination-out" : "source-over";
    ctx.fillStyle = "rgba(255,255,255,0.72)";
    ctx.beginPath();
    ctx.arc(point.x, point.y, brushSize() / 2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    state.dirty = true;
    state.enabled = true;
    if (!wasMasked) {
      emitChange();
    }
  }

  function clearMask() {
    const { paintCanvas, hint } = elements();
    const wasMasked = hasMask();
    paintCanvas.getContext("2d").clearRect(0, 0, paintCanvas.width, paintCanvas.height);
    state.dirty = false;
    hint.textContent = "涂抹需要重绘的区域";
    if (wasMasked) {
      emitChange();
    }
  }

  function whiteMaskCanvas() {
    const { paintCanvas } = elements();
    const output = document.createElement("canvas");
    output.width = paintCanvas.width;
    output.height = paintCanvas.height;
    const outputCtx = output.getContext("2d");
    const paintCtx = paintCanvas.getContext("2d");
    const image = paintCtx.getImageData(0, 0, paintCanvas.width, paintCanvas.height);
    for (let i = 0; i < image.data.length; i += 4) {
      const alpha = image.data[i + 3] > 0 ? 0 : 255;
      image.data[i] = 255;
      image.data[i + 1] = 255;
      image.data[i + 2] = 255;
      image.data[i + 3] = alpha;
    }
    outputCtx.putImageData(image, 0, 0);
    return output;
  }

  function appendMask(formData) {
    if (!state.enabled || !state.dirty || !state.width || !state.height) {
      return Promise.resolve(false);
    }
    return new Promise((resolve, reject) => {
      whiteMaskCanvas().toBlob((blob) => {
        if (!blob) {
          reject(new Error("生成蒙版失败"));
          return;
        }
        formData.append("mask", blob, "mask.png");
        resolve(true);
      }, "image/png");
    });
  }

  function hasMask() {
    return !!state.enabled && !!state.dirty;
  }

  function usesFile(file) {
    return !!file && state.file === file;
  }

  function setMode(mode) {
    const { modeButtons, paintCanvas } = elements();
    state.mode = mode === "erase" ? "erase" : "paint";
    paintCanvas.style.cursor = state.mode === "erase" ? "cell" : "crosshair";
    modeButtons.forEach((button) => {
      button.classList.toggle("active", button.dataset.maskMode === state.mode);
    });
  }

  function init() {
    const { root, paintCanvas, clear, close: closeButton, disable, zoom, modeButtons } = elements();
    if (!paintCanvas || !clear) {
      return;
    }
    paintCanvas.addEventListener("pointerdown", (event) => {
      event.preventDefault();
      state.drawing = true;
      paintCanvas.setPointerCapture(event.pointerId);
      paintAt(event);
    });
    paintCanvas.addEventListener("pointermove", (event) => {
      if (!state.drawing) {
        return;
      }
      event.preventDefault();
      paintAt(event);
    });
    paintCanvas.addEventListener("pointerup", () => {
      state.drawing = false;
    });
    paintCanvas.addEventListener("pointercancel", () => {
      state.drawing = false;
    });
    clear.addEventListener("click", clearMask);
    closeButton.addEventListener("click", close);
    disable.addEventListener("click", disableMask);
    root.addEventListener("click", (event) => {
      if (event.target === root) {
        close();
      }
    });
    zoom.addEventListener("input", () => {
      state.zoom = Math.max(0.5, Math.min(3, Number(zoom.value) / 100 || 1));
      applyZoom();
    });
    modeButtons.forEach((button) => {
      button.addEventListener("click", () => setMode(button.dataset.maskMode));
    });
    setMode("paint");
  }

  window.KuaimaMaskEditor = {
    init,
    loadReference,
    open,
    close,
    disable: disableMask,
    clear: clearMask,
    appendMask,
    hasMask,
    usesFile,
  };
})();
