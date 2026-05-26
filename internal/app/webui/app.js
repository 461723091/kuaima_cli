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

let selectedPayment = null;
let selectedImages = [];
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

function renderImagePreviews() {
  const list = $("#imagePreviewList");
  if (selectedImages.length === 0) {
    list.className = "image-preview-list empty";
    list.textContent = "上传参考图后可预览、删除和调整顺序";
    return;
  }

  list.className = "image-preview-list";
  list.innerHTML = selectedImages.map((file, index) => (
    '<div class="image-preview-item">' +
      '<img src="' + esc(imagePreviewURL(file)) + '" alt="' + esc(file.name) + '">' +
      '<div class="image-preview-meta"><strong>' + esc(file.name) + '</strong><span>' +
      esc(formatFileSize(file.size)) + '</span></div>' +
      '<div class="image-preview-actions">' +
        '<button type="button" title="上移" data-image-action="up" data-image-index="' + index + '">↑</button>' +
        '<button type="button" title="下移" data-image-action="down" data-image-index="' + index + '">↓</button>' +
        '<button type="button" title="删除" data-image-action="remove" data-image-index="' + index + '">×</button>' +
      '</div>' +
    '</div>'
  )).join("");
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

function moveImage(from, to) {
  if (to < 0 || to >= selectedImages.length) {
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
  $("#genStatus").textContent = "生成中...";
  $("#genStatus").className = "status";
  try {
    const result = await api("/api/generate", {
      method: "POST",
      body: new FormData(event.target),
    });
    $("#gallery").classList.remove("empty");
    $("#gallery").innerHTML = result.images.map((url) => (
      '<a href="' + esc(url) + '" target="_blank"><img src="' + esc(url) + '" alt="生成结果"></a>'
    )).join("") || '<div class="status">没有返回图片</div>';
    $("#genStatus").textContent = "已保存 " + result.saved.length + " 张";
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

document.addEventListener("click", async (event) => {
  const imageButton = event.target.closest("[data-image-action]");
  if (imageButton) {
    const index = Number(imageButton.dataset.imageIndex);
    const action = imageButton.dataset.imageAction;
    if (action === "remove") {
      selectedImages.splice(index, 1);
      syncImageInput();
      renderImagePreviews();
    } else if (action === "up") {
      moveImage(index, index - 1);
    } else if (action === "down") {
      moveImage(index, index + 1);
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
loadBalance();
loadRechargeInfo();
loadAccountLink();
