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

function renderBalance(usage) {
  $("#balanceTop").textContent = usage.total_available_text || "-";
  $("#usedTop").textContent = usage.total_used_text || "-";
  $("#accountName").textContent = usage.name || "-";

  const subscriptions = (usage.subscriptions || []).map((item) => (
    '<div class="subscription">' +
      '<div><span>套餐状态</span><strong>' + esc(item.status || "-") + '</strong></div>' +
      '<div><span>可用</span><strong>' + esc(item.available || "-") + '</strong></div>' +
      '<div><span>周期</span><strong>' + esc(item.start || "-") + ' - ' + esc(item.end || "-") + '</strong></div>' +
    '</div>'
  )).join("");

  $("#balanceBox").innerHTML =
    '<div class="balance-total"><span>当前可用</span><strong>' + esc(usage.total_available_text || "-") +
    '</strong></div><div class="balance-meta"><span>账号</span><b>' + esc(usage.name || "-") +
    '</b></div><div class="balance-meta"><span>已消耗</span><b>' + esc(usage.total_used_text || "-") +
    '</b></div>' + subscriptions;
}

async function loadBalance() {
  try {
    const usage = await api("/api/balance");
    renderBalance(usage);
  } catch (error) {
    $("#balanceTop").textContent = "读取失败";
    $("#balanceBox").innerHTML = '<span class="status error">' + esc(error.message) + '</span>';
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
      '<div class="plan-price"><strong>￥' + esc(formatAmount(plan.price_amount)) + '</strong><span>' +
      esc(plan.duration_value) + esc(plan.duration_unit) + '</span></div>' +
      '<button class="btn small" type="button" data-plan-id="' + Number(plan.id) + '">购买</button>' +
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
  $("#payment").innerHTML = '<span class="status">正在创建支付订单...</span>';
  const payment = await api("/api/recharge/pay", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  $("#payment").innerHTML = '<div class="paybox"><img src="' + esc(payment.qr) +
    '" alt="支付二维码"><div><span>订单号</span><strong>' + esc(payment.trade_no || "-") +
    '</strong><a href="' + esc(payment.url) + '" target="_blank">打开支付链接</a></div></div>';
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
    await pay({ amount: Number($("#amount").value || 0) });
  } catch (error) {
    $("#payment").innerHTML = '<span class="status error">' + esc(error.message) + '</span>';
  }
};

$("#refreshBilling").onclick = () => {
  loadBalance();
  loadRechargeInfo();
};

document.addEventListener("click", async (event) => {
  const amountButton = event.target.closest("[data-amount]");
  const planButton = event.target.closest("[data-plan-id]");
  if (!amountButton && !planButton) {
    return;
  }
  try {
    if (amountButton) {
      $("#amount").value = amountButton.dataset.amount;
      await pay({ amount: Number(amountButton.dataset.amount) });
    } else {
      await pay({ plan_id: Number(planButton.dataset.planId) });
    }
  } catch (error) {
    $("#payment").innerHTML = '<span class="status error">' + esc(error.message) + '</span>';
  }
});

initConfig().catch((error) => {
  $("#genStatus").textContent = error.message;
  $("#genStatus").className = "status error";
});
loadBalance();
loadRechargeInfo();
