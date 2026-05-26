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
const HISTORY_LIMIT = 50;

let selectedPayment = null;
let selectedImages = [];
let draggedImageIndex = -1;
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

function readHistory() {
  try {
    const value = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
    return Array.isArray(value) ? value : [];
  } catch {
    return [];
  }
}

function writeHistory(items) {
  localStorage.setItem(HISTORY_KEY, JSON.stringify(items.slice(0, HISTORY_LIMIT)));
}

function saveHistory(result) {
  const form = $("#genForm");
  const item = {
    id: String(Date.now()),
    created_at: new Date().toISOString(),
    prompt: form.elements.prompt.value,
    image_model: form.elements.image_model.value,
    image_count: form.elements.image_count.value,
    image_quality: form.elements.image_quality.value,
    ratio: $("#ratioSelect").value,
    resolution: $("#resolutionSelect").value,
    image_size: $('[name="image_size"]').value,
    reference_count: selectedImages.length,
    images: result.images || [],
  };
  writeHistory([item].concat(readHistory()));
  renderHistory();
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

function renderHistory() {
  const panel = $("#historyPanel");
  const history = readHistory();
  if (history.length === 0) {
    panel.innerHTML = '<div class="history-empty">暂无历史记录</div>';
    return;
  }
  panel.innerHTML = history.map((item) => (
    '<article class="history-item" data-history-id="' + esc(item.id) + '">' +
      '<div class="history-head"><div><strong>' + esc(formatHistoryTime(item.created_at)) +
      '</strong><p>' + esc(item.prompt || "") + '</p></div>' +
      '<button class="btn small" type="button" data-history-action="restore" data-history-id="' + esc(item.id) +
      '">恢复参数</button></div>' +
      '<div class="history-meta">' + esc(item.image_model || "-") + ' · ' +
      esc(item.image_size || "-") + ' · ' + esc(qualityLabel(item.image_quality)) +
      ' · 参考图 ' + esc(item.reference_count || 0) + ' 张</div>' +
      '<div class="history-images">' + (item.images || []).map((url) => (
        '<button type="button" data-history-action="use-image" data-image-url="' + esc(url) +
        '" title="作为参考图"><img src="' + esc(url) + '" alt="历史图片"><span>作为参考图</span></button>'
      )).join("") + '</div>' +
    '</article>'
  )).join("");
}

function restoreHistory(id) {
  const item = readHistory().find((entry) => entry.id === id);
  if (!item) {
    return;
  }
  const form = $("#genForm");
  form.elements.prompt.value = item.prompt || "";
  form.elements.image_model.value = item.image_model || form.elements.image_model.value;
  form.elements.image_count.value = item.image_count || form.elements.image_count.value;
  form.elements.image_quality.value = item.image_quality || form.elements.image_quality.value;
  if (item.ratio) {
    $("#ratioSelect").value = item.ratio;
  }
  if (item.resolution) {
    $("#resolutionSelect").value = item.resolution;
  }
  updateSize();
}

async function addImageURLAsReference(url) {
  $("#genStatus").textContent = "正在添加历史图片...";
  $("#genStatus").className = "status";
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
}

function showResultView(view) {
  const isHistory = view === "history";
  $("#gallery").hidden = isHistory;
  $("#historyPanel").hidden = !isHistory;
  document.querySelectorAll("[data-result-view]").forEach((button) => {
    button.classList.toggle("active", button.dataset.resultView === view);
  });
  if (isHistory) {
    renderHistory();
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
  $("#genStatus").textContent = "生成中...";
  $("#genStatus").className = "status";
  try {
    const result = await api("/api/generate", {
      method: "POST",
      body: new FormData(event.target),
    });
    $("#gallery").classList.remove("empty");
    $("#gallery").innerHTML = result.images.map((url) => (
      '<div class="gallery-item"><a href="' + esc(url) +
      '" target="_blank"><img src="' + esc(url) +
      '" alt="生成结果"></a><button type="button" data-history-action="use-image" data-image-url="' +
      esc(url) + '">作为参考图</button></div>'
    )).join("") || '<div class="status">没有返回图片</div>';
    $("#genStatus").textContent = "已保存 " + result.saved.length + " 张";
    saveHistory(result);
    showResultView("gallery");
    loadBalance();
  } catch (error) {
    $("#genStatus").textContent = error.message;
    $("#genStatus").className = "status error";
  }
};

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
    showResultView(resultTab.dataset.resultView);
    return;
  }

  const historyButton = event.target.closest("[data-history-action]");
  if (historyButton) {
    const action = historyButton.dataset.historyAction;
    try {
      if (action === "restore") {
        restoreHistory(historyButton.dataset.historyId);
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
renderHistory();
loadBalance();
loadRechargeInfo();
loadAccountLink();
