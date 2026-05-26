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

const HISTORY_KEY = "kuaima.webui.imageHistory.v1";
const HISTORY_DB_NAME = "kuaima.webui.history.v1";
const HISTORY_STORE = "items";
const HISTORY_LIMIT = 50;
const SETTINGS_KEY = "kuaima.webui.settings.v1";

let selectedPayment = null;
let selectedImages = [];
let draggedImageIndex = -1;
let historyDBPromise = null;
let activeGenerationController = null;
const imagePreviewURLs = new WeakMap();

function esc(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#39;",
  }[char]));
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

function imagePreviewURL(file) {
  if (!imagePreviewURLs.has(file)) {
    imagePreviewURLs.set(file, URL.createObjectURL(file));
  }
  return imagePreviewURLs.get(file);
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

function renderImagePreviews() {
  const list = $("#imagePreviewList");
  if (selectedImages.length === 0) {
    list.className = "image-preview-list empty";
    list.textContent = "上传参考图后可预览，拖动调整顺序";
    return;
  }

  list.className = "image-preview-list";
  list.innerHTML = selectedImages.map((file, index) => (
    '<div class="image-preview-item" draggable="true" data-image-index="' + index + '">' +
    '<img src="' + esc(imagePreviewURL(file)) + '" alt="' + esc(file.name) + '">' +
    '<div class="image-preview-meta"><strong>' + esc(file.name) + '</strong><span>' +
    esc(formatFileSize(file.size)) + ' · 拖动调整顺序</span></div>' +
    '<div class="image-preview-actions">' +
    '<button type="button" title="删除" data-image-action="remove" data-image-index="' + index + '">×</button>' +
    '</div>' +
    '</div>'
  )).join("");
}

function moveImage(from, to) {
  if (from === to || to < 0 || to >= selectedImages.length) {
    return;
  }
  const [file] = selectedImages.splice(from, 1);
  selectedImages.splice(to, 0, file);
  syncImageInput();
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
    high: "高",
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
  document.querySelectorAll("[data-amount],[data-plan-id]").forEach((item) => item.classList.remove("selected"));
  const chip = document.querySelector('[data-amount="' + CSS.escape(String(amount)) + '"]');
  if (chip) {
    chip.classList.add("selected");
  }
}

function renderBalance(usage) {
  $("#balanceTop").textContent = usage.total_available_text || "-";
  $("#usedTop").textContent = usage.total_used_text || "-";
  $("#accountName").textContent = usage.name || "-";
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
  try {
    const result = await api("/api/account/link");
    $("#usageDetailLink").href = result.url || "#";
  } catch (error) {
    $("#usageDetailLink").removeAttribute("href");
    $("#usageDetailLink").textContent = "使用详情链接获取失败";
    $("#usageDetailLink").classList.add("error");
  }
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
    esc(plan.duration_value) + esc(plan.duration_unit) + '</span></div>' +
    '<button class="btn small" type="button" data-plan-id="' + Number(plan.id) +
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

async function readLegacyHistory() {
  try {
    const value = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
    return Array.isArray(value) ? value : [];
  } catch {
    return [];
  }
}

async function readHistory() {
  const db = await openHistoryDB();
  if (!db) {
    return readLegacyHistory();
  }
  const items = await requestResult(db.transaction(HISTORY_STORE, "readonly").objectStore(HISTORY_STORE).getAll());
  return (Array.isArray(items) ? items : []).sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")));
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

async function writeHistory(items) {
  const limited = items.slice(0, HISTORY_LIMIT);
  const db = await openHistoryDB();
  if (!db) {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(limited.map(({ references, ...item }) => item)));
    return;
  }
  await withHistoryStore("readwrite", (store) => {
    store.clear();
    limited.forEach((item) => store.put(item));
  });
}

async function putHistoryItem(item) {
  const db = await openHistoryDB();
  if (!db) {
    const deduped = (await readLegacyHistory()).filter((entry) => historyKey(entry) !== historyKey(item));
    const legacyItem = { ...item, references: [] };
    localStorage.setItem(HISTORY_KEY, JSON.stringify([legacyItem].concat(deduped).slice(0, HISTORY_LIMIT)));
    return;
  }
  const existing = await readHistory();
  const duplicate = existing.find((entry) => historyKey(entry) === historyKey(item));
  await withHistoryStore("readwrite", (store) => {
    if (duplicate) {
      store.delete(duplicate.id);
    }
    store.put(item);
  });
  await trimHistory();
}

async function deleteHistoryItem(id) {
  const db = await openHistoryDB();
  if (!db) {
    const next = (await readLegacyHistory()).filter((entry) => entry.id !== id);
    localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
    return;
  }
  await withHistoryStore("readwrite", (store) => {
    store.delete(id);
  });
}

async function migrateLegacyHistory() {
  const legacy = await readLegacyHistory();
  if (legacy.length === 0 || !(await openHistoryDB())) {
    return;
  }
  const existing = await readHistory();
  if (existing.length > 0) {
    return;
  }
  await writeHistory(legacy);
  localStorage.removeItem(HISTORY_KEY);
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

function historyReferences() {
  return selectedImages.map((file) => ({
    name: file.name || "reference-image.png",
    type: file.type || "image/png",
    size: file.size || 0,
    blob: file,
  }));
}

async function saveHistory(result, extra = {}) {
  const form = $("#genForm");
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
    references: historyReferences(),
    images: result.images || [],
    error: extra.error || "",
  };
  await putHistoryItem(item);
  await renderHistory();
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

function historyReferencePreview(ref) {
  if (!ref || !ref.blob) {
    return "";
  }
  return URL.createObjectURL(ref.blob);
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
    '">恢复参数</button>' +
    '<button class="btn small danger" type="button" data-history-action="delete" data-history-id="' + esc(item.id) +
    '">删除</button></div></div>' +
    '<div class="history-meta">' + esc(item.image_model || "-") + ' · ' +
    esc(item.image_size || "-") + ' · ' + esc(qualityLabel(item.image_quality)) +
    ' · 参考图 ' + esc(item.reference_count || 0) + ' 张</div>' +
    (item.error ? '<div class="history-error">' + esc(item.error) + '</div>' : '') +
    '<div class="history-images">' + (item.references || []).map((ref, index) => {
      const preview = historyReferencePreview(ref);
      return '<button type="button" data-history-action="use-reference" data-history-id="' + esc(item.id) +
        '" data-reference-index="' + index + '" title="使用历史参考图"><img src="' + esc(preview) +
        '" alt="历史参考图"><span>历史参考图</span></button>';
    }).join("") + (item.images || []).map((url) => (
      '<button type="button" data-history-action="use-image" data-image-url="' + esc(url) +
      '" title="作为参考图"><img src="' + esc(url) + '" alt="历史图片"><span>作为参考图</span></button>'
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
  selectedImages = historyReferenceFiles(item);
  syncImageInput();
  renderImagePreviews();
  updateSize();
  $("#genStatus").textContent = selectedImages.length > 0 ? "已恢复参数和参考图" : "已恢复参数";
  $("#genStatus").className = "status gen-status";
}

function historyReferenceFiles(item) {
  return (item.references || []).filter((ref) => ref && ref.blob).map((ref) => (
    new File([ref.blob], ref.name || "history-reference.png", { type: ref.type || ref.blob.type || "image/png" })
  ));
}

async function addHistoryReferenceAsImage(id, index) {
  const item = (await readHistory()).find((entry) => entry.id === id);
  const ref = item && item.references && item.references[Number(index)];
  if (!ref || !ref.blob) {
    throw new Error("历史参考图不存在");
  }
  selectedImages.push(new File([ref.blob], ref.name || "history-reference.png", { type: ref.type || ref.blob.type || "image/png" }));
  syncImageInput();
  renderImagePreviews();
  $("#genStatus").textContent = "已添加历史参考图";
  $("#genStatus").className = "status gen-status";
}

function renderGalleryImages(images) {
  const unique = Array.from(new Set(images || []));
  $("#gallery").classList.toggle("empty", unique.length === 0);
  $("#gallery").innerHTML = unique.map((url) => (
    '<div class="gallery-item"><a href="' + esc(url) +
    '" target="_blank"><img src="' + esc(url) +
    '" alt="生成结果"></a><button type="button" data-history-action="use-image" data-image-url="' +
    esc(url) + '">作为参考图</button></div>'
  )).join("") || '<div class="status">没有返回图片</div>';
}

function renderGalleryMessage(message, className = "status") {
  $("#gallery").classList.add("empty");
  $("#gallery").innerHTML = '<div class="' + esc(className) + '">' + esc(message) + '</div>';
}

async function generateStream(form, signal) {
  const images = [];
  const result = { images, saved: [], text: "" };
  $("#gallery").classList.remove("empty");
  $("#gallery").innerHTML = '<div class="generation-placeholder"><span class="spinner"></span><span>正在等待首张图片...</span></div>';
  await apiStream("/api/generate/stream", {
    method: "POST",
    body: new FormData(form),
    signal,
  }, {
    status(payload) {
      $("#genStatus").textContent = payload.message || "生成中...";
    },
    text(payload) {
      if (payload.text) {
        result.text += payload.text;
        $("#genStatus").textContent = result.text;
      }
    },
    image(payload) {
      if (payload.url && !images.includes(payload.url)) {
        images.push(payload.url);
        renderGalleryImages(images);
      }
    },
    done(payload) {
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
  return result;
}

async function addImageURLAsReference(url) {
  $("#genStatus").textContent = "正在添加历史图片...";
  $("#genStatus").className = "status gen-status";
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error("历史图片读取失败");
  }
  const blob = await response.blob();
  const name = decodeURIComponent(String(url).split("/").pop() || "history-image.png");
  selectedImages.push(new File([blob], name, { type: blob.type || "image/png" }));
  syncImageInput();
  renderImagePreviews();
  $("#genStatus").textContent = "已添加历史图片";
  $("#genStatus").className = "status gen-status";
}

async function showResultView(view) {
  const isHistory = view === "history";
  $("#gallery").hidden = isHistory;
  $("#historyPanel").hidden = !isHistory;
  document.querySelectorAll("[data-result-view]").forEach((button) => {
    button.classList.toggle("active", button.dataset.resultView === view);
  });
  if (isHistory) {
    await renderHistory();
  }
}

async function initConfig() {
  const config = await api("/api/config");
  for (const [key, value] of Object.entries(config)) {
    const input = document.querySelector('[name="' + key + '"]');
    if (input && value !== "" && value != null) {
      input.value = value;
    }
  }
  applySize(config.image_size);
  applySettings(readSettings());
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

$('[name="images"]').onchange = (event) => {
  selectedImages = selectedImages.concat(Array.from(event.target.files || []));
  syncImageInput();
  renderImagePreviews();
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
    $("#genStatus").textContent = "已保存 " + result.saved.length + " 张";
    $("#genStatus").className = "status gen-status";
    await saveHistory(result);
    await showResultView("gallery");
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
    await saveHistory({ images: [] }, { status: "failed", error: error.message });
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

document.addEventListener("dragstart", (event) => {
  const item = event.target.closest(".image-preview-item");
  if (!item) {
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
  }
  draggedImageIndex = -1;
});

document.addEventListener("dragover", (event) => {
  if (event.target.closest(".image-preview-item")) {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }
});

document.addEventListener("drop", (event) => {
  const item = event.target.closest(".image-preview-item");
  if (!item || draggedImageIndex < 0) {
    return;
  }
  event.preventDefault();
  moveImage(draggedImageIndex, Number(item.dataset.imageIndex));
});

document.addEventListener("click", async (event) => {
  const imageButton = event.target.closest("[data-image-action]");
  if (imageButton) {
    const index = Number(imageButton.dataset.imageIndex);
    if (imageButton.dataset.imageAction === "remove") {
      selectedImages.splice(index, 1);
      syncImageInput();
      renderImagePreviews();
    }
    return;
  }

  const resultTab = event.target.closest("[data-result-view]");
  if (resultTab) {
    await showResultView(resultTab.dataset.resultView);
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
      } else if (action === "use-reference") {
        await addHistoryReferenceAsImage(historyButton.dataset.historyId, historyButton.dataset.referenceIndex);
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
$("#closePaymentModal").onclick = closePaymentModal;
$("#paymentModal").onclick = (event) => {
  if (event.target === $("#paymentModal")) {
    closePaymentModal();
  }
};

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !$("#paymentModal").hidden) {
    closePaymentModal();
  }
});

initConfig().catch((error) => {
  $("#genStatus").textContent = error.message;
  $("#genStatus").className = "status error";
});
migrateLegacyHistory().then(renderHistory).catch((error) => {
  $("#historyPanel").innerHTML = '<div class="history-empty">历史读取失败：' + esc(error.message) + '</div>';
});
loadBalance();
loadRechargeInfo();
loadAccountLink();
