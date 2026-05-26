const $ = (selector) => document.querySelector(selector);

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

document.querySelectorAll(".tabs button").forEach((button) => {
  button.onclick = () => {
    document.querySelectorAll(".tabs button,.view").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    $("#" + button.dataset.tab).classList.add("active");
  };
});

api("/api/config").then((config) => {
  for (const [key, value] of Object.entries(config)) {
    const input = document.querySelector('[name="' + key + '"]');
    if (input) {
      input.value = value;
    }
  }
});

$("#genForm").onsubmit = async (event) => {
  event.preventDefault();
  $("#genStatus").textContent = "生成中...";
  $("#genStatus").className = "status";
  try {
    const result = await api("/api/generate", {
      method: "POST",
      body: new FormData(event.target),
    });
    $("#gallery").innerHTML = result.images.map((url) => (
      '<a href="' + esc(url) + '" target="_blank"><img src="' + esc(url) + '"></a>'
    )).join("") || '<div class="status">没有返回图片</div>';
    $("#genStatus").textContent = "已保存 " + result.saved.length + " 张";
  } catch (error) {
    $("#genStatus").textContent = error.message;
    $("#genStatus").className = "status error";
  }
};

$("#balanceBtn").onclick = async () => {
  const status = $("#balanceStatus");
  status.textContent = "查询中...";
  status.className = "status";
  try {
    const usage = await api("/api/balance");
    $("#balanceBox").innerHTML = '<table class="table"><tr><th>账号</th><td>' + esc(usage.name) +
      '</td></tr><tr><th>可用额度</th><td>' + esc(usage.total_available_text) +
      '</td></tr><tr><th>已消耗</th><td>' + esc(usage.total_used_text) + "</td></tr></table>" +
      (usage.subscriptions || []).map((item) => (
        '<table class="table"><tr><th>订阅状态</th><td>' + esc(item.status) +
        '</td></tr><tr><th>可用</th><td>' + esc(item.available) +
        '</td></tr><tr><th>已用</th><td>' + esc(item.used) +
        '</td></tr><tr><th>周期</th><td>' + esc(item.start) + " - " + esc(item.end) +
        "</td></tr></table>"
      )).join("");
    status.textContent = "";
  } catch (error) {
    status.textContent = error.message;
    status.className = "status error";
  }
};

async function pay(body) {
  const payment = await api("/api/recharge/pay", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  $("#payment").innerHTML = '<div class="paybox"><img src="' + esc(payment.qr) +
    '"><div><p>订单号：' + esc(payment.trade_no || "-") +
    '</p><p><a href="' + esc(payment.url) + '" target="_blank">打开支付链接</a></p></div></div>';
}

$("#amountPay").onclick = () => pay({ amount: Number($("#amount").value || 0) });

$("#loadPlans").onclick = async () => {
  const status = $("#rechargeStatus");
  status.textContent = "加载中...";
  status.className = "status";
  try {
    const info = await api("/api/recharge/info");
    $("#plans").innerHTML = (info.plans || []).filter((plan) => plan.enabled).map((plan) => (
      '<div class="panel plan"><b>' + esc(plan.title) + '</b><div class="status">' +
      esc(plan.subtitle || "") + " " + esc(plan.price_amount) + " 元 / " +
      esc(plan.duration_value) + esc(plan.duration_unit) +
      '</div><button class="btn" onclick="pay({plan_id:' + Number(plan.id) + '})">购买</button></div>'
    )).join("") || '<div class="status">暂无套餐</div>';
    status.textContent = "";
  } catch (error) {
    status.textContent = error.message;
    status.className = "status error";
  }
};
