const $ = (selector) => document.querySelector(selector);

const SIZE_MATRIX = {
  "1k": {
    "1:1": "1024x1024",
    "2:3": "1024x1536",
    "3:2": "1536x1024",
  },
  "2k": {
    "1:1": "2048x2048",
    "2:3": "1440x2560",
    "3:2": "2560x1440",
  },
  "4k": {
    "1:1": "2880x2880",
    "2:3": "2160x3840",
    "3:2": "3840x2160",
  },
};

const SIZE_LOOKUP = Object.entries(SIZE_MATRIX).reduce((lookup, [resolution, ratios]) => {
  Object.entries(ratios).forEach(([ratio, size]) => {
    lookup[size] = { resolution, ratio };
  });
  return lookup;
}, {});

const HISTORY_DB_NAME = "kuaima.webui.history.v1";
const HISTORY_STORE = "items";
const HISTORY_LIMIT = 100;
const SETTINGS_KEY = "kuaima.webui.settings.v1";
const REFERENCE_IMAGE_MAX_SIDE = 1536;
const REFERENCE_IMAGE_JPEG_QUALITY = 0.82;

let selectedPayment = null;
let selectedImages = [];
let useOriginalImages = false;
let draggedImageIndex = -1;
let activeRechargeTab = "amount";
let historyDBPromise = null;
let historyStoragePersistRequested = false;
let activeGenerationController = null;
const imagePreviewURLs = new WeakMap();
const historySourceURLs = new WeakMap();
const imageFingerprintCache = new WeakMap();
let draggedHistoryImageURL = "";

function isGenerationRunning() {
  return activeGenerationController !== null;
}

window.addEventListener("beforeunload", (event) => {
  if (!isGenerationRunning()) {
    return;
  }

  const message = "图片正在生成中，关闭页面会中断当前任务。确定要离开吗？";
  event.preventDefault();
  event.returnValue = message;
  return message;
});

function setResultsVisible(visible) {
  $("#resultsPanel").hidden = !visible;
}

function esc(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#39;",
  }[char]));
}
function unit(value) {
  if (value == 'month') return '月';
  else if (value == 'year') return '年';
  else if (value == 'day') return '天';
  else if (value == 'week') return '周';
  return value;
}

async function api(url, options) {
  const response = await fetch(url, options);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || response.statusText);
  }
  return body;
}

async function apiStream(url, options, handlers) {
  const response = await fetch(url, options);
  if (!response.ok || !response.body) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || response.statusText);
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split(/\r?\n\r?\n/);
    buffer = chunks.pop() || "";
    for (const chunk of chunks) {
      const lines = chunk.split(/\r?\n/);
      let event = "message";
      const data = [];
      lines.forEach((line) => {
        if (line.startsWith("event:")) {
          event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          data.push(line.slice(5).trimStart());
        }
      });
      if (data.length === 0) {
        continue;
      }
      const payload = JSON.parse(data.join("\n"));
      if (handlers[event]) {
        handlers[event](payload);
      }
    }
  }
}

function updateSize() {
  const ratio = $("#ratioSelect").value;
  const resolution = $("#resolutionSelect").value;
  const size = SIZE_MATRIX[resolution][ratio];
  $('[name="image_size"]').value = size;
  $("#sizePreview").textContent = size;
  document.querySelectorAll("[data-ratio]").forEach((button) => {
    button.classList.toggle("active", button.dataset.ratio === ratio);
  });
}

function applySize(size) {
  const match = SIZE_LOOKUP[String(size || "").trim()];
  if (!match) {
    updateSize();
    return;
  }
  $("#ratioSelect").value = match.ratio;
  $("#resolutionSelect").value = match.resolution;
  updateSize();
}

function syncImageInput() {
  const input = $('[name="images"]');
  if (typeof DataTransfer === "undefined") {
    return;
  }
  const transfer = new DataTransfer();
  selectedImages.forEach((file) => transfer.items.add(file));
  input.files = transfer.files;
}

function clipboardImageFiles(event) {
  const clipboard = event && event.clipboardData;
  if (!clipboard || !clipboard.items) {
    return [];
  }
  return Array.from(clipboard.items)
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file) => file && String(file.type || "").startsWith("image/"));
}

async function addReferenceFiles(files, sourceText) {
  const images = await dedupeReferenceFiles(files);
  if (images.length === 0) {
    $("#genStatus").textContent = "参考图已存在";
    $("#genStatus").className = "status gen-status";
    return false;
  }
  selectedImages = selectedImages.concat(images);
  syncImageInput();
  renderImagePreviews();
  $("#genStatus").textContent = "已" + (sourceText || "添加") + images.length + "张参考图";
  $("#genStatus").className = "status gen-status";
  return true;
}

function setRechargeTab(tab) {
  activeRechargeTab = tab === "plan" ? "plan" : "amount";
  const amountTab = $("#billingTabAmount");
  const planTab = $("#billingTabPlan");
  const amountPanel = $("#amountOptions") && $("#amountOptions").closest(".field");
  const planTitle = $("#rechargeStatus") && $("#rechargeStatus").closest(".section-title");
  const plansPanel = $("#plans");
  if (amountTab) {
    amountTab.classList.toggle("active", activeRechargeTab === "amount");
    amountTab.setAttribute("aria-selected", String(activeRechargeTab === "amount"));
  }
  if (planTab) {
    planTab.classList.toggle("active", activeRechargeTab === "plan");
    planTab.setAttribute("aria-selected", String(activeRechargeTab === "plan"));
  }
  if (amountPanel) {
    amountPanel.hidden = activeRechargeTab !== "amount";
  }
  if (planTitle) {
    planTitle.hidden = activeRechargeTab !== "plan";
  }
  if (plansPanel) {
    plansPanel.hidden = activeRechargeTab !== "plan";
  }
}

function imagePreviewURL(file) {
  if (!imagePreviewURLs.has(file)) {
    imagePreviewURLs.set(file, URL.createObjectURL(file));
  }
  return imagePreviewURLs.get(file);
}

function bytesToHex(bytes) {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

async function imageFingerprint(file) {
  if (!file) {
    return "";
  }
  const sourceURL = historySourceURLs.get(file);
  if (sourceURL) {
    return "url:" + sourceURL;
  }
  if (!imageFingerprintCache.has(file)) {
    imageFingerprintCache.set(file, (async () => {
      try {
        if (file.arrayBuffer && crypto && crypto.subtle && crypto.subtle.digest) {
          const buffer = await file.arrayBuffer();
          const digest = await crypto.subtle.digest("SHA-256", buffer);
          return "sha256:" + bytesToHex(new Uint8Array(digest));
        }
      } catch {
        // Fallback below.
      }
      return [
        "meta",
        file.name || "",
        file.size || 0,
        file.type || "",
        file.lastModified || 0,
      ].join(":");
    })());
  }
  return imageFingerprintCache.get(file);
}

async function dedupeReferenceFiles(files, existingFiles = selectedImages) {
  const unique = [];
  const seen = new Set();
  for (const file of existingFiles || []) {
    seen.add(await imageFingerprint(file));
  }
  for (const file of files || []) {
    if (!file || !String(file.type || "").startsWith("image/")) {
      continue;
    }
    const fingerprint = await imageFingerprint(file);
    if (seen.has(fingerprint)) {
      continue;
    }
    seen.add(fingerprint);
    unique.push(file);
  }
  return unique;
}

function formatFileSize(bytes) {
  const size = Number(bytes);
  if (!Number.isFinite(size)) {
    return "";
  }
  if (size < 1024 * 1024) {
    return Math.max(1, Math.round(size / 1024)) + " KB";
  }
  return (size / 1024 / 1024).toFixed(1) + " MB";
}

function useOriginalReferenceImages(form) {
  return useOriginalImages;
}

function referenceCompressionDisabled(form) {
  const hasMask = window.KuaimaMaskEditor && window.KuaimaMaskEditor.hasMask();
  return useOriginalReferenceImages(form) || hasMask;
}

function loadImageForCompression(file) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("参考图读取失败"));
    };
    img.src = url;
  });
}

function canvasToBlob(canvas, type, quality) {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (!blob) {
        reject(new Error("参考图压缩失败"));
        return;
      }
      resolve(blob);
    }, type, quality);
  });
}

function compressedReferenceName(file) {
  const name = String(file && file.name || "reference-image").replace(/\.[^.]*$/, "");
  return name + ".jpg";
}

async function compressReferenceImage(file) {
  if (!file || !String(file.type || "").startsWith("image/")) {
    return file;
  }
  if (String(file.type || "").toLowerCase() === "image/gif") {
    return file;
  }
  const img = await loadImageForCompression(file);
  const width = img.naturalWidth || img.width;
  const height = img.naturalHeight || img.height;
  if (!width || !height) {
    return file;
  }
  const scale = Math.min(1, REFERENCE_IMAGE_MAX_SIDE / Math.max(width, height));
  const targetWidth = Math.max(1, Math.round(width * scale));
  const targetHeight = Math.max(1, Math.round(height * scale));
  const canvas = document.createElement("canvas");
  canvas.width = targetWidth;
  canvas.height = targetHeight;
  const ctx = canvas.getContext("2d");
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, targetWidth, targetHeight);
  ctx.drawImage(img, 0, 0, targetWidth, targetHeight);
  const blob = await canvasToBlob(canvas, "image/jpeg", REFERENCE_IMAGE_JPEG_QUALITY);
  if (blob.size >= file.size && scale === 1) {
    return file;
  }
  return new File([blob], compressedReferenceName(file), {
    type: "image/jpeg",
    lastModified: file.lastModified || Date.now(),
  });
}

async function referenceFilesForSubmit(form) {
  if (referenceCompressionDisabled(form)) {
    return selectedImages;
  }
  const compressed = [];
  for (const file of selectedImages) {
    try {
      compressed.push(await compressReferenceImage(file));
    } catch (error) {
      console.warn("Reference image compression failed", error);
      compressed.push(file);
    }
  }
  return compressed;
}

async function buildGenerateFormData(form) {
  const formData = new FormData(form);
  formData.delete("images");
  const files = await referenceFilesForSubmit(form);
  files.forEach((file) => {
    formData.append("images", file, file.name || "reference-image.png");
  });
  return formData;
}

function renderImagePreviews() {
  const list = $("#imagePreviewList");
  if (selectedImages.length === 0) {
    list.className = "image-preview-list empty";
    list.textContent = "拖动历史图片到这里添加参考图";
    return;
  }

  list.className = "image-preview-list";
  list.innerHTML = (
    '<div class="reference-options">' +
    '<label class="original-image-toggle" title="开启后参考图按原始文件提交">' +
    '<input id="useOriginalImages" type="checkbox" value="1"' + (useOriginalImages ? ' checked' : '') + '>' +
    '<span>使用原图</span>' +
    '</label>' +
    '</div>'
  ) + selectedImages.map((file, index) => (
    (() => {
      const hasMask = index === 0 && window.KuaimaMaskEditor &&
        window.KuaimaMaskEditor.usesFile(file) && window.KuaimaMaskEditor.hasMask();
      return (
        '<div class="image-preview-item" draggable="true" data-image-index="' + index + '">' +
        '<img src="' + esc(imagePreviewURL(file)) + '" alt="' + esc(file.name) + '">' +
        '<div class="image-preview-meta"><strong>' + esc(file.name) + '</strong><span>' +
        esc(formatFileSize(file.size)) + (index === 0 ? ' · 可局部修改' : ' · 拖动可调整顺序') + '</span></div>' +
        '<div class="image-preview-actions">' +
        (index === 0 ? '<button type="button" class="image-preview-mask-btn' + (hasMask ? ' active' : '') +
          '" title="局部编辑" data-image-action="edit-mask" data-image-index="' + index + '">' +
          (hasMask ? '已涂抹' : '局部修改') + '</button>' : '') +
        '<button type="button" title="删除" data-image-action="remove" data-image-index="' + index + '">×</button>' +
        '</div>' +
        '</div>'
      );
    })()
  )).join("");
}

function syncMaskAfterImageChange() {
  if (!window.KuaimaMaskEditor) {
    return;
  }
  const first = selectedImages[0] || null;
  if (first && window.KuaimaMaskEditor.usesFile(first)) {
    return;
  }
  if (window.KuaimaMaskEditor.hasMask()) {
    window.KuaimaMaskEditor.disable();
  } else {
    window.KuaimaMaskEditor.loadReference(null);
  }
}

function moveImage(from, to) {
  if (from === to || to < 0 || to >= selectedImages.length) {
    return;
  }
  const [file] = selectedImages.splice(from, 1);
  selectedImages.splice(to, 0, file);
  syncImageInput();
  syncMaskAfterImageChange();
  renderImagePreviews();
}

function formatAmount(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return String(value || "");
  }
  return Number.isInteger(number) ? String(number) : number.toFixed(2).replace(/0+$/, "").replace(/\.$/, "");
}

function discountText(discount) {
  const value = Number(discount);
  if (!value || value >= 1) {
    return "";
  }
  return (value * 10).toFixed(1).replace(/\.0$/, "") + " 折";
}

function formatQuota(value) {
  const quota = Number(value);
  if (!Number.isFinite(quota) || quota <= 0) {
    return "-";
  }
  return Math.round(quota) + " (￥" + (quota / 500000).toFixed(2) + ")";
}

function qualityLabel(value) {
  return ({
    auto: "自动",
    low: "快速",
    medium: "中等",
    high: "高清",
  })[value] || value || "-";
}

function endpointLabel(value) {
  return value === "response" ? "Response 接口" : "Image 接口";
}

function readSettings() {
  try {
    const value = JSON.parse(localStorage.getItem(SETTINGS_KEY) || "{}");
    return value && typeof value === "object" ? value : {};
  } catch {
    return {};
  }
}

function saveSettings() {
  const form = $("#genForm");
  const settings = {
    image_model: form.elements.image_model.value,
    image_count: form.elements.image_count.value,
    image_quality: form.elements.image_quality.value,
    image_endpoint: form.elements.image_endpoint.value || "response",
    use_original_images: useOriginalReferenceImages(form),
    ratio: $("#ratioSelect").value,
    resolution: $("#resolutionSelect").value,
  };
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
}

function applySettings(settings) {
  const form = $("#genForm");
  if (settings.image_model) {
    form.elements.image_model.value = settings.image_model;
  }
  if (settings.image_count) {
    form.elements.image_count.value = settings.image_count;
  }
  if (settings.image_quality) {
    form.elements.image_quality.value = settings.image_quality;
  }
  form.elements.image_endpoint.value = settings.image_endpoint || form.elements.image_endpoint.value || "response";
  useOriginalImages = !!settings.use_original_images;
  if (settings.ratio) {
    $("#ratioSelect").value = settings.ratio;
  }
  if (settings.resolution) {
    $("#resolutionSelect").value = settings.resolution;
  }
  updateSize();
}

function looksLikeInsufficientBalance(message) {
  const lower = String(message || "").toLowerCase();
  return [
    "insufficient_quota",
    "insufficient quota",
    "insufficient credits",
    "insufficient balance",
    "out of credits",
    "quota exceeded",
    "balance_not_enough",
    "not enough balance",
    "余额不足",
    "额度不足",
    "额度失败",
    "余额不够",
    "额度不够",
    "欠费",
    "recharge",
    "充值",
  ].some((needle) => lower.includes(needle));
}

function handleOperationError(error, statusElement) {
  statusElement.textContent = error.message;
  statusElement.className = "status error";
  if (looksLikeInsufficientBalance(error.message)) {
    setBillingOpen(true);
    setRechargeTab("plan");
    loadRechargeInfo();
  }
}

function setBillingOpen(open) {
  const dropdown = $("#billingDropdown");
  dropdown.hidden = !open;
  $("#toggleBillingTop").setAttribute("aria-expanded", String(open));
}

function toggleBilling() {
  setBillingOpen($("#billingDropdown").hidden);
}

function selectAmount(amount) {
  selectedPayment = { amount: Number(amount) };
  $("#amount").value = amount;
  setRechargeTab("amount");
  document.querySelectorAll("[data-amount],[data-plan-id]").forEach((item) => item.classList.remove("selected"));
  const chip = document.querySelector('[data-amount="' + CSS.escape(String(amount)) + '"]');
  if (chip) {
    chip.classList.add("selected");
  }
}

function activeSubscription(subscriptions) {
  return (Array.isArray(subscriptions) ? subscriptions : []).find((item) => {
    const status = String((item && item.status) || "").toLowerCase();
    return status === "active";
  }) || null;
}

function renderSubscriptions(subscriptions) {
  const list = Array.isArray(subscriptions) ? subscriptions : [];
  $("#subscriptionCount").textContent = list.length ? String(list.length) : "";
  $("#subscriptionList").innerHTML = list.length ? list.map((item) => {
    const available = Number(item.available) || 0;
    const used = Number(item.used) || 0;
    const total = Number(item.total) || (available + used);
    const usedPercent = total > 0 ? Math.max(0, Math.min(100, used / total * 100)) : 0;
    return (
      '<div class="subscription-item">' +
      '<div class="subscription-head">' +
      '<strong>' + esc(item.status || "subscription") + '</strong>' +
      '<span>' + esc(item.start || "-") + ' - ' + esc(item.end || "-") + '</span>' +
      '</div>' +
      '<div class="subscription-bar" aria-hidden="true">' +
      '<div class="subscription-bar-used" style="width:' + usedPercent.toFixed(2) + '%"></div>' +
      '</div>' +
      '<div class="subscription-row"><span>可用</span><b>' + esc(item.available_text || item.available || "-") + '</b></div>' +
      '<div class="subscription-row"><span>已用</span><b>' + esc(item.used_text || item.used || "-") + '</b></div>' +
      '</div>'
    );
  }).join("") : '<span class="status">暂无订阅套餐</span>';
}

function renderBalance(usage) {
  const subscriptions = usage.subscriptions || [];
  const active = activeSubscription(subscriptions);
  $("#balanceTopLabel").textContent = active ? "当前套餐" : "可用token";
  $("#balanceTop").textContent = active
    ? ("有效至 " + (active.end || "-"))
    : (usage.total_available_text || "-");
  $("#balanceTop2").textContent = usage.total_available_text || "-";
  $("#usedTop").textContent = usage.total_used_text || "-";
  $("#accountName").textContent = usage.name || "-";
  $("#accountBalanceSummary").hidden = !!active;
  renderSubscriptions(subscriptions);

  const available = Number(usage.total_available) || 0;
  const used = Number(usage.total_used) || 0;
  const total = available + used;
  const usedPercent = total > 0 ? Math.max(0, Math.min(100, used / total * 100)) : 0;
  const availablePercent = total > 0 ? Math.max(0, Math.min(100, available / total * 100)) : 0;
  $("#usageProgressUsed").style.width = usedPercent.toFixed(2) + "%";
  $("#usageProgressAvailableLabel").textContent = "可用 " + availablePercent.toFixed(0) + "%";
  $("#usageProgressUsedLabel").textContent = "已用 " + usedPercent.toFixed(0) + "%";
}

async function loadBalance() {
  try {
    const usage = await api("/api/balance");
    renderBalance(usage);
  } catch (error) {
    $("#balanceTop").textContent = "读取失败";
    $("#usedTop").textContent = error.message;
  }
}

async function loadAccountLink() {
  const result = await api("/api/account/link");
  return result.url || "";
}

function renderAmountOptions(info) {
  const discounts = info.discount || {};
  $("#amountOptions").innerHTML = (info.amount_options || []).map((amount) => {
    const key = formatAmount(amount);
    const label = discountText(discounts[key]);
    return '<button class="chip" type="button" data-amount="' + esc(amount) + '">' +
      '<strong>￥' + esc(key) + '</strong>' +
      (label ? '<span>' + esc(label) + '</span>' : '<span>原价</span>') +
      '</button>';
  }).join("") || '<span class="status">暂无充值档位</span>';
}

function renderPlans(info) {
  $("#plans").innerHTML = (info.plans || []).filter((plan) => plan.enabled).map((plan) => (
    '<div class="plan-card">' +
    '<div><b>' + esc(plan.title) + '</b><p>' + esc(plan.subtitle || "订阅套餐") + '</p></div>' +
    '<div class="plan-quota"><span>额度</span><strong>' + esc(formatQuota(plan.total_amount)) + '</strong></div>' +
    '<div class="plan-price"><strong>￥' + esc(formatAmount(plan.price_amount)) + '</strong><span>' +
    esc(plan.duration_value) + unit(plan.duration_unit) + '</span></div>' +
    '<button class="btn small primary" type="button" data-plan-id="' + Number(plan.id) +
    '" data-plan-title="' + esc(plan.title) + '">订阅</button>' +
    '</div>'
  )).join("") || '<span class="status">暂无套餐</span>';
}

async function loadRechargeInfo() {
  const status = $("#rechargeStatus");
  status.textContent = "读取中...";
  status.className = "status";
  try {
    const info = await api("/api/recharge/info");
    renderAmountOptions(info);
    renderPlans(info);
    setRechargeTab(activeRechargeTab);
    status.textContent = "";
  } catch (error) {
    status.textContent = error.message;
    status.className = "status error";
  }
}

async function pay(body) {
  openPaymentModal('<span class="status">正在创建支付订单...</span>');
  const payment = await api("/api/recharge/pay", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  setBillingOpen(false);
  openPaymentModal('<a class="paybox modal-paybox" href="' + esc(payment.url) +
    '" target="_blank"><img src="' + esc(payment.qr) +
    '" alt="支付二维码"><div><span>微信扫一扫支付</span><strong>订单号 ' +
    esc(payment.trade_no || "-") + '</strong></div></a>');
}

function openPaymentModal(content) {
  $("#paymentModalBody").innerHTML = content;
  $("#paymentModal").hidden = false;
}

function closePaymentModal() {
  $("#paymentModal").hidden = true;
}

function openHistoryDB() {
  if (!("indexedDB" in window)) {
    return Promise.resolve(null);
  }
  if (!historyDBPromise) {
    historyDBPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(HISTORY_DB_NAME, 1);
      request.onupgradeneeded = () => {
        const db = request.result;
        if (!db.objectStoreNames.contains(HISTORY_STORE)) {
          db.createObjectStore(HISTORY_STORE, { keyPath: "id" });
        }
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
  }
  return historyDBPromise;
}

async function requestPersistentHistoryStorage() {
  if (historyStoragePersistRequested || !navigator.storage || !navigator.storage.persist) {
    return false;
  }
  historyStoragePersistRequested = true;
  try {
    return await navigator.storage.persist();
  } catch {
    return false;
  }
}

async function withHistoryStore(mode, callback) {
  const db = await openHistoryDB();
  if (!db) {
    return callback(null);
  }
  return new Promise((resolve, reject) => {
    const tx = db.transaction(HISTORY_STORE, mode);
    const store = tx.objectStore(HISTORY_STORE);
    let result;
    tx.oncomplete = () => resolve(result);
    tx.onerror = () => reject(tx.error);
    result = callback(store);
  });
}

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

async function readHistory() {
  const db = await openHistoryDB();
  if (!db) {
    return [];
  }
  const items = await requestResult(db.transaction(HISTORY_STORE, "readonly").objectStore(HISTORY_STORE).getAll());
  return (Array.isArray(items) ? items : []).sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")));
}

async function cleanupHistoryImageData() {
  const items = await readHistory();
  const dirty = items.filter((item) => (item.references || []).some((ref) => ref && "blob" in ref));
  if (dirty.length === 0) {
    return;
  }
  await withHistoryStore("readwrite", (store) => {
    dirty.forEach((item) => {
      item.references = (item.references || []).map((ref) => ({
        name: ref && ref.name || "reference-image.png",
        type: ref && ref.type || "image/png",
        size: ref && ref.size || 0,
        url: ref && ref.url || "",
      }));
      store.put(item);
    });
  });
}

async function trimHistory() {
  const items = await readHistory();
  const stale = items.slice(HISTORY_LIMIT);
  if (stale.length === 0) {
    return;
  }
  await withHistoryStore("readwrite", (store) => {
    stale.forEach((item) => store.delete(item.id));
  });
}

async function putHistoryItem(item) {
  const db = await openHistoryDB();
  if (!db) {
    return;
  }
  const existing = await readHistory();
  const duplicate = existing.find((entry) => historyKey(entry) === historyKey(item));
  const previous = existing[0];
  const replacePreviousFailure = previous &&
    previous.status === "failed" &&
    item.status === "failed" &&
    historyFailureKey(previous) === historyFailureKey(item);
  await withHistoryStore("readwrite", (store) => {
    if (replacePreviousFailure) {
      store.delete(previous.id);
    } else if (duplicate) {
      store.delete(duplicate.id);
    }
    store.put(item);
  });
  await trimHistory();
}

async function deleteHistoryItem(id) {
  const db = await openHistoryDB();
  if (!db) {
    return;
  }
  await withHistoryStore("readwrite", (store) => {
    store.delete(id);
  });
}

function historyKey(item) {
  return [
    item.status || "success",
    item.prompt || "",
    item.image_model || "",
    item.image_endpoint || "",
    item.image_size || "",
    item.image_quality || "",
    item.image_count || "",
    item.reference_count || 0,
    (item.images || []).join(","),
    item.error || "",
  ].join("\n");
}

function historyFailureKey(item) {
  const references = (item.references || []).map((ref) => ({
    name: ref && ref.name || "",
    type: ref && ref.type || "",
    size: ref && ref.size || 0,
    url: ref && ref.url || "",
  }));
  return [
    item.prompt || "",
    item.image_model || "",
    item.image_endpoint || "",
    item.image_size || "",
    item.image_quality || "",
    item.image_count || "",
    item.ratio || "",
    item.resolution || "",
    item.reference_count || 0,
    JSON.stringify(references),
  ].join("\n");
}

function historyReferences() {
  return selectedImages.map((file) => ({
    name: file.name || "reference-image.png",
    type: file.type || "image/png",
    size: file.size || 0,
    url: historySourceURLs.get(file) || "",
  }));
}

async function uploadHistoryReferences(files) {
  const pending = Array.from(files || []).filter((file) => file && !historySourceURLs.get(file));
  if (pending.length === 0) {
    return [];
  }
  const form = new FormData();
  pending.forEach((file) => {
    form.append("images", file, file.name || "reference-image.png");
  });
  const result = await api("/api/history/upload", {
    method: "POST",
    body: form,
  });
  return Array.isArray(result.urls) ? result.urls : [];
}

async function ensureHistoryReferenceURLs() {
  const refs = historyReferences();
  const missing = [];
  const missingIndexes = [];
  selectedImages.forEach((file, index) => {
    if (!historySourceURLs.get(file)) {
      missing.push(file);
      missingIndexes.push(index);
    }
  });
  if (missing.length === 0) {
    return refs;
  }
  try {
    const urls = await uploadHistoryReferences(missing);
    missingIndexes.forEach((index, i) => {
      const url = urls[i] || "";
      if (url) {
        refs[index].url = url;
        historySourceURLs.set(selectedImages[index], url);
      }
    });
  } catch (error) {
    console.warn("History reference upload failed", error);
  }
  return refs;
}

async function saveHistory(result, extra = {}) {
  const form = $("#genForm");
  const references = await ensureHistoryReferenceURLs();
  const timing = result && result.timing ? result.timing : null;
  const item = {
    id: String(Date.now()),
    created_at: new Date().toISOString(),
    status: extra.status || "success",
    prompt: form.elements.prompt.value,
    image_model: form.elements.image_model.value,
    image_count: form.elements.image_count.value,
    image_quality: form.elements.image_quality.value,
    image_endpoint: form.elements.image_endpoint.value,
    ratio: $("#ratioSelect").value,
    resolution: $("#resolutionSelect").value,
    image_size: $('[name="image_size"]').value,
    reference_count: selectedImages.length,
    references,
    images: result.images || [],
    response_ms: timing && Number.isFinite(Number(timing.response_ms)) ? Math.round(Number(timing.response_ms)) : null,
    total_ms: timing && Number.isFinite(Number(timing.total_ms)) ? Math.round(Number(timing.total_ms)) : null,
    error: extra.error || "",
  };
  await putHistoryItem(item);
  await renderHistory();
}

async function saveHistorySafely(result, extra = {}) {
  try {
    await saveHistory(result, extra);
    return "";
  } catch (error) {
    console.warn("History save failed", error);
    return error && error.message ? error.message : "unknown history error";
  }
}

function formatHistoryTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatDuration(value) {
  const duration = Number(value);
  if (!Number.isFinite(duration) || duration < 0) {
    return "-";
  }
  if (duration < 1000) {
    return Math.max(1, Math.round(duration)) + "ms";
  }
  return (duration / 1000).toFixed(duration >= 10000 ? 0 : 1).replace(/\.0$/, "") + "s";
}

function formatTimingSummary(item) {
  const parts = [];
  if (Number.isFinite(Number(item && item.response_ms))) {
    parts.push("响应 " + formatDuration(item.response_ms));
  }
  if (Number.isFinite(Number(item && item.total_ms))) {
    parts.push("总计 " + formatDuration(item.total_ms));
  }
  return parts.join(" · ");
}

function renderResultTiming(timing) {
  const label = $("#resultTiming");
  if (!label) {
    return;
  }
  const summary = timing ? formatTimingSummary(timing) : "";
  label.textContent = summary;
  label.hidden = !summary;
}

function historyReferencePreview(ref) {
  return ref && ref.url ? ref.url : "";
}

async function renderHistory() {
  const panel = $("#historyPanel");
  const history = await readHistory();
  if (history.length === 0) {
    panel.innerHTML = '<div class="history-empty">暂无历史记录</div>';
    return;
  }
  panel.innerHTML = history.map((item) => (
    '<article class="history-item ' + esc(item.status === "failed" ? "failed" : "") + '" data-history-id="' + esc(item.id) + '">' +
    '<div class="history-head"><div><strong>' + esc(formatHistoryTime(item.created_at)) +
    (item.status === "failed" ? ' · 失败' : '') +
    '</strong><p>' + esc(item.prompt || "") + '</p></div>' +
    '<div class="history-actions">' +
    '<button class="btn small" type="button" data-history-action="restore" data-history-id="' + esc(item.id) +
    '">复用</button>' +
    '<button class="btn small danger" type="button" data-history-action="delete" data-history-id="' + esc(item.id) +
    '">删除</button></div></div>' +
    '<div class="history-meta">' + esc(item.image_model || "-") + ' · ' +
    esc(item.image_size || "-") + ' · ' + esc(qualityLabel(item.image_quality)) +
    ' · 参考图 ' + esc(item.reference_count || 0) + ' 张' +
    (formatTimingSummary(item) ? ' · ' + esc(formatTimingSummary(item)) : '') + '</div>' +
    (item.error ? '<div class="history-error">' + esc(item.error) + '</div>' : '') +
    '<div class="history-images">' + (item.references || []).map((ref, index) => ({ ref, index })).filter((entry) => entry.ref && entry.ref.url).map((entry) => {
      const ref = entry.ref;
      const preview = historyReferencePreview(ref);
      return '<div class="history-image-card">' +
        '<button type="button" class="history-image-preview" draggable="true" data-preview-image="' + esc(preview) + '" data-history-drag-url="' + esc(preview) + '" title="点击放大预览，拖到参考区添加">' +
        '<img src="' + esc(preview) + '" alt="历史参考图预览">' +
        '<span>点击放大预览</span>' +
        '</button>' +
        '</div>';
    }).join("") + (item.images || []).map((url) => (
      '<div class="history-image-card">' +
      '<button type="button" class="history-image-preview" draggable="true" data-preview-image="' + esc(url) + '" data-history-drag-url="' + esc(url) + '" title="点击放大预览，拖到参考区添加">' +
      '<img src="' + esc(url) + '" alt="历史图片预览">' +
      '<span>点击放大预览</span>' +
      '</button>' +
      '</div>'
    )).join("") + '</div>' +
    '</article>'
  )).join("");
}

async function restoreHistory(id) {
  const item = (await readHistory()).find((entry) => entry.id === id);
  if (!item) {
    return;
  }
  const form = $("#genForm");
  form.elements.prompt.value = item.prompt || "";
  form.elements.image_model.value = item.image_model || form.elements.image_model.value;
  form.elements.image_count.value = item.image_count || form.elements.image_count.value;
  form.elements.image_quality.value = item.image_quality || form.elements.image_quality.value;
  form.elements.image_endpoint.value = item.image_endpoint || form.elements.image_endpoint.value;
  if (item.ratio) {
    $("#ratioSelect").value = item.ratio;
  }
  if (item.resolution) {
    $("#resolutionSelect").value = item.resolution;
  }
  if (window.KuaimaPromptHelper && typeof window.KuaimaPromptHelper.sync === "function") {
    window.KuaimaPromptHelper.sync();
  }
  selectedImages = await historyReferenceFiles(item);
  syncImageInput();
  syncMaskAfterImageChange();
  renderImagePreviews();
  updateSize();
  $("#genStatus").textContent = selectedImages.length > 0 ? "已恢复参数和参考图" : "已恢复参数";
  $("#genStatus").className = "status gen-status";
}

async function historyReferenceFiles(item) {
  const refs = (item.references || []).filter((ref) => ref && ref.url);
  const files = [];
  for (const ref of refs) {
    files.push(await fileFromHistoryURL(ref.url, ref.name, ref.type));
  }
  return files;
}

async function fileFromHistoryURL(url, name, type) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error("历史图片读取失败");
  }
  const blob = await response.blob();
  const file = new File([blob], name || decodeURIComponent(String(url).split("/").pop() || "history-image.png"), {
    type: type || blob.type || "image/png",
  });
  historySourceURLs.set(file, url);
  return file;
}

function renderGalleryImages(images) {
  const unique = Array.from(new Set(images || []));
  setResultsVisible(true);
  if (unique.length === 0) {
    $("#gallery").classList.add("empty");
    $("#gallery").innerHTML = '<div class="status">没有返回图片</div>';
    return;
  }
  $("#gallery").classList.remove("empty");
  $("#gallery").innerHTML = unique.map((url) => (
    '<div class="gallery-item"><button class="gallery-preview" type="button" data-preview-image="' + esc(url) +
    '"><img src="' + esc(url) +
    '" alt="生成结果"><span class="image-size-label" data-image-size>读取尺寸...</span></button>' +
    '<button class="gallery-use" type="button" data-history-action="use-image" data-image-url="' +
    esc(url) + '">作为参考图</button></div>'
  )).join("");
}

function renderGalleryMessage(message, className = "status") {
  setResultsVisible(Boolean(message));
  $("#gallery").classList.add("empty");
  $("#gallery").innerHTML = '<div class="' + esc(className) + '">' + esc(message) + '</div>';
}

function imageDimensionsText(img) {
  if (!img || !img.naturalWidth || !img.naturalHeight) {
    return "尺寸读取中";
  }
  return img.naturalWidth + " × " + img.naturalHeight + " px";
}

function updateLoadedImageSize(img) {
  const item = img.closest(".gallery-item");
  const label = item && item.querySelector("[data-image-size]");
  if (label) {
    label.textContent = imageDimensionsText(img);
  }
}

function openImagePreview(button) {
  const img = button.querySelector("img");
  $("#imageModalImg").src = button.dataset.previewImage || (img && img.src) || "";
  $("#imageModalSize").textContent = imageDimensionsText(img);
  $("#imageModal").hidden = false;
}

function updateSaveDirStatus(message, isError = false) {
  const status = $("#saveDirStatus");
  status.textContent = message;
  status.className = isError ? "status error" : "status";
}

function currentOutputDir() {
  return String($("#genForm").elements.save_dir.value || "").trim();
}

async function chooseOutputDir() {
  const saveDir = currentOutputDir();
  if (!saveDir) {
    updateSaveDirStatus("请输入图片生成结果目录", true);
    return;
  }
  try {
    const result = await api("/api/output/set", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ save_dir: saveDir }),
    });
    $("#genForm").elements.save_dir.value = result.save_dir || saveDir;
    updateSaveDirStatus("已应用：" + (result.save_dir || saveDir));
  } catch (error) {
    updateSaveDirStatus(error.message, true);
  }
}

async function openDefaultOutputDir() {
  const saveDir = currentOutputDir();
  updateSaveDirStatus("正在打开输出目录...");
  try {
    const result = await api("/api/output/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ save_dir: saveDir }),
    });
    $("#genForm").elements.save_dir.value = result.save_dir || saveDir;
    updateSaveDirStatus("已打开：" + (result.save_dir || saveDir));
  } catch (error) {
    updateSaveDirStatus(error.message, true);
  }
}

async function generateStream(form, signal) {
  const images = [];
  const result = { images, saved: [], text: "", timing: null };
  const startedAt = performance.now();
  let firstResponseAt = 0;
  const markFirstResponse = () => {
    if (!firstResponseAt) {
      firstResponseAt = performance.now();
    }
  };
  setResultsVisible(true);
  $("#gallery").classList.remove("empty");
  $("#gallery").innerHTML = '<div class="generation-placeholder"><span class="spinner"></span><span>正在等待首张图片...</span></div>';
  renderResultTiming(null);
  if (selectedImages.length > 0 && !referenceCompressionDisabled(form)) {
    $("#genStatus").textContent = "正在压缩参考图...";
  }
  const formData = await buildGenerateFormData(form);
  if (window.KuaimaMaskEditor) {
    await window.KuaimaMaskEditor.appendMask(formData);
  }
  await apiStream("/api/generate/stream", {
    method: "POST",
    body: formData,
    signal,
  }, {
    status(payload) {
      $("#genStatus").textContent = payload.message || "生成中...";
    },
    text(payload) {
      if (payload.text) {
        markFirstResponse();
        result.text += payload.text;
        $("#genStatus").textContent = result.text;
      }
    },
    image(payload) {
      if (payload.url && !images.includes(payload.url)) {
        markFirstResponse();
        images.push(payload.url);
        renderGalleryImages(images);
      }
    },
    done(payload) {
      markFirstResponse();
      result.saved = payload.saved || [];
      result.text = payload.text || result.text;
      (payload.images || []).forEach((url) => {
        if (!images.includes(url)) {
          images.push(url);
        }
      });
      renderGalleryImages(images);
    },
    error(payload) {
      throw new Error(payload.error || "生成失败");
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

async function addImageURLAsReference(url) {
  $("#genStatus").textContent = "正在添加历史图片...";
  $("#genStatus").className = "status gen-status";
  if (selectedImages.some((file) => historySourceURLs.get(file) === url)) {
    $("#genStatus").textContent = "历史图片已存在";
    $("#genStatus").className = "status gen-status";
    return;
  }
  await addReferenceFiles([await fileFromHistoryURL(url)], "添加历史图片");
}

async function initConfig() {
  const config = await api("/api/config");
  const versionLabel = $("#appVersion");
  if (versionLabel) {
    versionLabel.textContent = config.version ? "v" + config.version : "";
    versionLabel.hidden = !config.version;
  }
  for (const [key, value] of Object.entries(config)) {
    const input = document.querySelector('[name="' + key + '"]');
    if (input && value !== "" && value != null) {
      input.value = value;
    }
  }
  applySize(config.image_size);
  applySettings(readSettings());
  if (window.KuaimaMaskEditor) {
    window.KuaimaMaskEditor.init();
  }
}

$("#ratioSelect").onchange = updateSize;
$("#resolutionSelect").onchange = updateSize;
document.querySelectorAll("[data-ratio]").forEach((button) => {
  button.onclick = () => {
    $("#ratioSelect").value = button.dataset.ratio;
    updateSize();
  };
});

$('[name="images"]').onclick = (event) => {
  event.target.value = "";
  setTimeout(syncImageInput, 1000);
};

$('[name="images"]').onchange = async (event) => {
  await addReferenceFiles(Array.from(event.target.files || []), "添加");
};

$("#genForm").onsubmit = async (event) => {
  event.preventDefault();
  updateSize();
  syncImageInput();
  saveSettings();
  const submitButton = event.target.querySelector('button[type="submit"]');
  const cancelButton = $("#cancelGenerate");
  activeGenerationController = new AbortController();
  if (submitButton) {
    submitButton.disabled = true;
    submitButton.innerHTML = '<span class="spinner small"></span><span>生成中...</span>';
  }
  cancelButton.hidden = false;
  $("#genStatus").textContent = "生成中...";
  $("#genStatus").className = "status gen-status loading";
  try {
    const result = await generateStream(event.target, activeGenerationController.signal);
    const historyError = await saveHistorySafely(result);
    $("#genStatus").textContent = "已保存 " + result.saved.length + " 张" +
      (historyError ? "，历史保存失败：" + historyError : "");
    $("#genStatus").className = "status gen-status";
    loadBalance();
  } catch (error) {
    const canceled = error && error.name === "AbortError";
    if ($("#gallery").querySelector(".generation-placeholder")) {
      renderGalleryMessage(canceled ? "已取消生成" : "生成失败：" + error.message, canceled ? "status" : "status error");
    }
    if (canceled) {
      $("#genStatus").textContent = "已取消生成";
      $("#genStatus").className = "status gen-status";
      return;
    }
    await saveHistorySafely({ images: [] }, { status: "failed", error: error.message });
    handleOperationError(error, $("#genStatus"));
  } finally {
    activeGenerationController = null;
    cancelButton.hidden = true;
    if (submitButton) {
      submitButton.disabled = false;
      submitButton.textContent = "生成图片";
    }
  }
};

$("#cancelGenerate").onclick = () => {
  if (activeGenerationController) {
    activeGenerationController.abort();
  }
};

["change", "input"].forEach((eventName) => {
  $("#genForm").addEventListener(eventName, (event) => {
    if (event.target.matches("#useOriginalImages")) {
      useOriginalImages = event.target.checked;
      saveSettings();
      return;
    }
    if (event.target.matches("select,input[type='number']")) {
      saveSettings();
    }
  });
});

$("#amountPay").onclick = async () => {
  try {
    const typedAmount = Number($("#amount").value || 0);
    const body = typedAmount > 0 ? { amount: typedAmount } : selectedPayment;
    await pay(body || {});
  } catch (error) {
    openPaymentModal('<span class="status error">' + esc(error.message) + '</span>');
  }
};

document.querySelectorAll("[data-billing-tab]").forEach((button) => {
  button.onclick = () => setRechargeTab(button.dataset.billingTab);
});

document.addEventListener("dragstart", (event) => {
  const item = event.target.closest(".image-preview-item");
  if (!item) {
    const historyPreview = event.target.closest(".history-image-preview");
    if (!historyPreview) {
      return;
    }
    draggedHistoryImageURL = historyPreview.dataset.historyDragUrl || historyPreview.dataset.previewImage || "";
    if (event.dataTransfer && draggedHistoryImageURL) {
      event.dataTransfer.effectAllowed = "copy";
      event.dataTransfer.setData("text/plain", draggedHistoryImageURL);
    }
    historyPreview.classList.add("dragging");
    return;
  }
  draggedImageIndex = Number(item.dataset.imageIndex);
  item.classList.add("dragging");
  event.dataTransfer.effectAllowed = "move";
});

document.addEventListener("dragend", (event) => {
  const item = event.target.closest(".image-preview-item");
  if (item) {
    item.classList.remove("dragging");
  } else {
    const historyPreview = event.target.closest(".history-image-preview");
    if (historyPreview) {
      historyPreview.classList.remove("dragging");
    }
  }
  draggedImageIndex = -1;
  draggedHistoryImageURL = "";
  $("#imagePreviewList").classList.remove("drag-over");
});

document.addEventListener("dragover", (event) => {
  const previewList = event.target.closest("#imagePreviewList");
  if (previewList && draggedHistoryImageURL) {
    event.preventDefault();
    previewList.classList.add("drag-over");
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "copy";
    }
    return;
  }
  if (event.target.closest(".image-preview-item")) {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    $("#imagePreviewList").classList.remove("drag-over");
  }
});

document.addEventListener("drop", (event) => {
  const previewList = event.target.closest("#imagePreviewList");
  if (previewList && draggedHistoryImageURL) {
    event.preventDefault();
    previewList.classList.remove("drag-over");
    addImageURLAsReference(draggedHistoryImageURL).catch((error) => {
      $("#genStatus").textContent = error.message;
      $("#genStatus").className = "status error";
    });
    return;
  }
  const item = event.target.closest(".image-preview-item");
  if (!item || draggedImageIndex < 0) {
    $("#imagePreviewList").classList.remove("drag-over");
    return;
  }
  event.preventDefault();
  moveImage(draggedImageIndex, Number(item.dataset.imageIndex));
  $("#imagePreviewList").classList.remove("drag-over");
});

document.addEventListener("paste", async (event) => {
  const files = clipboardImageFiles(event);
  if (files.length === 0) {
    return;
  }
  event.preventDefault();
  await addReferenceFiles(files, "粘贴");
});

document.addEventListener("load", (event) => {
  if (event.target.matches(".gallery-preview img")) {
    updateLoadedImageSize(event.target);
  }
}, true);

document.addEventListener("kuaima-mask-change", () => {
  renderImagePreviews();
});

document.addEventListener("click", async (event) => {
  const previewButton = event.target.closest("[data-preview-image]");
  if (previewButton) {
    openImagePreview(previewButton);
    return;
  }

  const imageButton = event.target.closest("[data-image-action]");
  if (imageButton) {
    const index = Number(imageButton.dataset.imageIndex);
    if (imageButton.dataset.imageAction === "remove") {
      selectedImages.splice(index, 1);
      syncImageInput();
      syncMaskAfterImageChange();
      renderImagePreviews();
    } else if (imageButton.dataset.imageAction === "edit-mask") {
      if (index !== 0) {
        $("#genStatus").textContent = "只能对第一张参考图使用局部编辑";
        $("#genStatus").className = "status gen-status";
      } else if (window.KuaimaMaskEditor) {
        window.KuaimaMaskEditor.open(selectedImages[0]);
      }
    }
    return;
  }

  const usageLink = event.target.closest("#usageDetailLink");
  if (usageLink) {
    event.preventDefault();
    const originalText = usageLink.textContent;
    usageLink.textContent = "正在刷新链接...";
    usageLink.classList.remove("error");
    try {
      const url = await loadAccountLink();
      if (!url) {
        throw new Error("使用详情链接为空");
      }
      window.open(url, "_blank", "noopener");
      usageLink.textContent = originalText;
    } catch (error) {
      usageLink.textContent = "使用详情链接获取失败";
      usageLink.classList.add("error");
    }
    return;
  }

  const historyButton = event.target.closest("[data-history-action]");
  if (historyButton) {
    const action = historyButton.dataset.historyAction;
    try {
      if (action === "restore") {
        await restoreHistory(historyButton.dataset.historyId);
      } else if (action === "delete") {
        await deleteHistoryItem(historyButton.dataset.historyId);
        await renderHistory();
      } else if (action === "use-image") {
        await addImageURLAsReference(historyButton.dataset.imageUrl);
      }
    } catch (error) {
      $("#genStatus").textContent = error.message;
      $("#genStatus").className = "status error";
    }
    return;
  }

  const amountButton = event.target.closest("[data-amount]");
  const planButton = event.target.closest("[data-plan-id]");
  if (!amountButton && !planButton) {
    return;
  }
  if (amountButton) {
    selectAmount(amountButton.dataset.amount);
  } else {
    try {
      setRechargeTab("plan");
      await pay({ plan_id: Number(planButton.dataset.planId) });
    } catch (error) {
      openPaymentModal('<span class="status error">' + esc(error.message) + '</span>');
    }
  }
});

document.addEventListener("click", (event) => {
  if (!event.target.closest(".billing-menu")) {
    setBillingOpen(false);
  }
});

$("#amount").oninput = () => {
  const amount = Number($("#amount").value || 0);
  if (amount > 0) {
    selectedPayment = { amount };
    document.querySelectorAll("[data-amount],[data-plan-id]").forEach((item) => item.classList.remove("selected"));
  }
};

$("#toggleBillingTop").onclick = toggleBilling;
$("#toggleHistory").onclick = () => {
  const sidebar = $("#historySidebar");
  const collapsed = !sidebar.classList.contains("collapsed");
  sidebar.classList.toggle("collapsed", collapsed);
  $("#toggleHistory").setAttribute("aria-expanded", String(!collapsed));
  $("#toggleHistory").setAttribute("aria-label", collapsed ? "展开历史记录" : "收起历史记录");
  $("#toggleHistory").textContent = collapsed ? "›" : "‹";
};
$("#chooseOutputDir").onclick = chooseOutputDir;
$("#openDefaultOutputDir").onclick = openDefaultOutputDir;
$("#closePaymentModal").onclick = closePaymentModal;
$("#closeImageModal").onclick = () => {
  $("#imageModal").hidden = true;
  $("#imageModalImg").removeAttribute("src");
};
$("#paymentModal").onclick = (event) => {
  if (event.target === $("#paymentModal")) {
    closePaymentModal();
  }
};
$("#imageModal").onclick = (event) => {
  if (event.target === $("#imageModal")) {
    $("#closeImageModal").click();
  }
};

document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") {
    return;
  }
  if (!$("#imageModal").hidden) {
    $("#closeImageModal").click();
  } else if (window.KuaimaMaskEditor && !$("#maskEditor").hidden) {
    window.KuaimaMaskEditor.close();
  } else if (!$("#paymentModal").hidden) {
    closePaymentModal();
  }
});

setRechargeTab("amount");
initConfig().catch((error) => {
  $("#genStatus").textContent = error.message;
  $("#genStatus").className = "status error";
});
requestPersistentHistoryStorage();
cleanupHistoryImageData().then(renderHistory).catch((error) => {
  $("#historyPanel").innerHTML = '<div class="history-empty">历史读取失败：' + esc(error.message) + '</div>';
});
loadBalance();
loadRechargeInfo();
