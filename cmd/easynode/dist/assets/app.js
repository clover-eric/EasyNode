const $ = (s) => document.querySelector(s);
const app = $("#app");
let state = null;
let toastTimer = null;
let lang = localStorage.getItem("easynode_lang") || ((navigator.language || "").toLowerCase().startsWith("zh") ? "zh" : "en");

const copyText = {
  zh: {
    requestFailed: "请求失败",
    setupIntro: "把复杂协议交给系统。你只需要填域名，默认方案已经适合大多数用户。",
    setupHintTitle: "推荐：保持默认选择",
    setupHintText: "系统会同时生成“稳、快、兼容”的节点。客户端里复制订阅即可使用。",
    adminPassword: "管理员密码",
    passwordPlaceholder: "至少 8 位",
    panelPath: "面板路径",
    domain: "你的域名",
    ipDirect: "我暂时没有域名，先用 IP 直连",
    domainMode: "域名部署",
    ipMode: "IP 直连",
    domainModeText: "自动申请证书，启用更多协议",
    ipModeText: "无需域名，只部署可直接使用的节点",
    nodePlan: "节点方案",
    restoreRecommended: "恢复推荐",
    recommendation: "推荐度",
    bestFor: "适合",
    clients: "软件",
    setupButton: "一键部署推荐节点",
    deploying: "部署中...",
    setupNotice: "不知道怎么选就保留默认：日常首选 + 高速传输 + 兼容优先。高级方案可以以后再开。",
    loginIntro: "输入管理员密码进入面板。",
    securePanel: "安全面板",
    securePanelText: "会话隔离、密码哈希、配置本机保存",
    loginButton: "进入面板",
    nodes: "个节点",
    copySub: "复制订阅",
    subQR: "订阅二维码",
    settings: "设置",
    addProtocol: "添加协议",
    protocolLibrary: "协议库",
    availableNow: "可立即使用",
    comingSoon: "输入域名并签发证书后可用",
    added: "已添加",
    add: "添加",
    remove: "移除",
    removeSuccess: "协议已移除",
    addSuccess: "协议已添加",
    config: "sing-box 配置",
    logout: "退出",
    chainProxy: "链式代理",
    chainText: "落地服务器生成配对码，入口服务器填入后建立出口配置。",
    endpointPlaceholder: "本机面板地址或公网地址",
    generateCode: "生成配对码",
    addExit: "添加落地",
    addExitText: "输入另一台 EasyNode 生成的配对码。",
    pairingCode: "配对码",
    myEndpoint: "我的入口地址",
    pairDoneButton: "完成配对",
    noPeers: "暂无配对",
    acceptingPairing: "允许被配对",
    pairingClosed: "已关闭被配对",
    closePairing: "关闭配对",
    openPairing: "允许配对",
    unpair: "解除配对",
    chainActive: "链式出口已启用",
    chainRemoved: "已解除配对",
    expired: "过期",
    paired: "配对完成",
    running: "运行中",
    stopped: "已停止",
    mainstreamClients: "主流客户端",
    latency: "延迟",
    port: "端口",
    traffic: "流量",
    score: "推荐",
    ipPurity: "IP 纯净度",
    checking: "检测中...",
    purityHint: "基于公开 IP 信誉信号的基础判断，仅作参考。",
    ipType: "IP 类型",
    nativeCheck: "原生倾向",
    useCases: "适用场景",
    risks: "风险提示",
    copyLink: "复制链接",
    showQR: "二维码",
    scanToImport: "扫码导入",
    stop: "停止",
    start: "启动",
    copied: "已复制",
    copyFailed: "复制失败，请长按或手动复制",
    unavailable: "暂不可用",
    certRequired: "需要域名证书接入后才可使用",
    language: "English",
    settingsTitle: "系统设置",
    currentPassword: "当前密码",
    newPassword: "新密码（可留空）",
    saveSettings: "保存设置",
    onlineUpgrade: "在线升级",
    upgradeTip: "升级会先备份配置，过程中面板可能短暂断开。建议在业务低峰操作。",
    upgradeConfirmTitle: "确认在线升级",
    upgradeConfirmText: "系统会先备份配置，然后拉取最新版本并重启服务。升级期间面板可能短暂断开。",
    upgradeStarted: "升级已执行，请稍后刷新页面",
    upgrading: "正在升级",
    upgradeDone: "升级完成，请刷新页面",
    upgradeDoneAction: "完成，返回首页",
    upgradeFailed: "升级失败",
    cancel: "取消",
    saved: "已保存",
    currentPasswordRequired: "请输入当前密码"
  },
  en: {
    requestFailed: "Request failed",
    setupIntro: "Let EasyNode handle protocol choices. Enter your domain; the default plan fits most users.",
    setupHintTitle: "Recommended: keep defaults",
    setupHintText: "EasyNode creates stable, fast, and compatible nodes. Copy the subscription into your client to use them.",
    adminPassword: "Admin password",
    passwordPlaceholder: "At least 8 characters",
    panelPath: "Panel path",
    domain: "Your domain",
    ipDirect: "No domain yet, use IP direct mode",
    domainMode: "Domain setup",
    ipMode: "IP direct",
    domainModeText: "Issue certificates automatically and enable more protocols",
    ipModeText: "No domain required; deploy directly usable nodes only",
    nodePlan: "Node plans",
    restoreRecommended: "Restore recommended",
    recommendation: "Score",
    bestFor: "Best for",
    clients: "Clients",
    setupButton: "Deploy recommended nodes",
    deploying: "Deploying...",
    setupNotice: "Not sure what to choose? Keep the defaults: Daily, Speed, and Compatibility. Advanced plans can be enabled later.",
    loginIntro: "Enter admin password to open the panel.",
    securePanel: "Secure panel",
    securePanelText: "Session isolation, hashed password, local config storage",
    loginButton: "Open panel",
    nodes: "nodes",
    copySub: "Copy subscription",
    subQR: "Subscription QR",
    settings: "Settings",
    addProtocol: "Add protocol",
    protocolLibrary: "Protocol library",
    availableNow: "Available now",
    comingSoon: "Available after domain certificate is issued",
    added: "Added",
    add: "Add",
    remove: "Remove",
    removeSuccess: "Protocol removed",
    addSuccess: "Protocol added",
    config: "sing-box config",
    logout: "Log out",
    chainProxy: "Chain proxy",
    chainText: "Generate a pairing code on the exit server, then enter it on the entry server.",
    endpointPlaceholder: "Panel URL or public endpoint",
    generateCode: "Generate code",
    addExit: "Add exit",
    addExitText: "Enter a pairing code generated by another EasyNode server.",
    pairingCode: "Pairing code",
    myEndpoint: "My entry endpoint",
    pairDoneButton: "Pair",
    noPeers: "No peers yet",
    acceptingPairing: "Accepting pairing",
    pairingClosed: "Pairing disabled",
    closePairing: "Disable pairing",
    openPairing: "Enable pairing",
    unpair: "Unpair",
    chainActive: "Chain exit enabled",
    chainRemoved: "Unpaired",
    expired: "expires",
    paired: "Paired",
    running: "Running",
    stopped: "Stopped",
    mainstreamClients: "mainstream clients",
    latency: "Latency",
    port: "Port",
    traffic: "Traffic",
    score: "Score",
    ipPurity: "IP purity",
    checking: "Checking...",
    purityHint: "Basic public IP reputation heuristic. For reference only.",
    ipType: "IP type",
    nativeCheck: "Native tendency",
    useCases: "Use cases",
    risks: "Risk flags",
    copyLink: "Copy link",
    showQR: "QR code",
    scanToImport: "Scan to import",
    stop: "Stop",
    start: "Start",
    copied: "Copied",
    copyFailed: "Copy failed. Please copy manually.",
    unavailable: "Unavailable",
    certRequired: "Requires domain certificate support",
    language: "中文",
    settingsTitle: "System settings",
    currentPassword: "Current password",
    newPassword: "New password (optional)",
    saveSettings: "Save settings",
    onlineUpgrade: "Online upgrade",
    upgradeTip: "Upgrade creates a backup first. The panel may disconnect briefly. Run it during low traffic hours.",
    upgradeConfirmTitle: "Confirm online upgrade",
    upgradeConfirmText: "EasyNode will back up config, pull the latest version, and restart services. The panel may disconnect briefly.",
    upgradeStarted: "Upgrade executed. Refresh the page later.",
    upgrading: "Upgrading",
    upgradeDone: "Upgrade complete. Refresh the page.",
    upgradeDoneAction: "Done, return home",
    upgradeFailed: "Upgrade failed",
    cancel: "Cancel",
    saved: "Saved",
    currentPasswordRequired: "Current password required"
  }
};

const profiles = {
  zh: {
    "vless-reality": ["日常首选", "VLESS Reality", "5/5", "v2rayN, Nekoray, Shadowrocket, sing-box", "网页、视频、聊天、长期稳定使用", "不需要域名证书，伪装能力强，默认建议开启。", "部分旧客户端不支持 Reality。", true],
    "hysteria2": ["高速传输", "Hysteria2", "5/5", "Nekoray, Shadowrocket, sing-box, Hiddify", "移动网络、跨境视频、下载、UDP 通畅线路", "基于 QUIC，弱网和高延迟线路上通常更快。", "需要域名证书；默认勾选，系统会自动尝试配置。", true],
    "trojan-tls": ["兼容优先", "Trojan TLS", "4/5", "几乎所有主流代理客户端", "给家人朋友用、旧客户端、追求少折腾", "形态接近普通 HTTPS，客户端兼容性最好。", "需要域名和 TLS 证书；默认勾选，系统会自动尝试配置。", true],
    "vless-ws-tls": ["CDN 线路", "VLESS WS TLS", "3/5", "v2rayN, Clash Meta, Shadowrocket", "套 Cloudflare/CDN、隐藏真实服务器 IP", "走 WebSocket + TLS，适合反代和 CDN 中转。", "速度通常不是最强，配置链路更长。", false],
    "tuic": ["备用加速", "TUIC v5", "3/5", "Nekoray, sing-box, 部分移动客户端", "UDP 可用时作为第二条 QUIC 备用线路", "现代 QUIC 协议，适合做补充节点。", "客户端覆盖率低于 Trojan 和 VLESS。", false]
  },
  en: {
    "vless-reality": ["Daily default", "VLESS Reality", "5/5", "v2rayN, Nekoray, Shadowrocket, sing-box", "Browsing, video, chat, long-term stable use", "No domain certificate required. Strong camouflage and best default choice.", "Some older clients do not support Reality.", true],
    "hysteria2": ["Speed mode", "Hysteria2", "5/5", "Nekoray, Shadowrocket, sing-box, Hiddify", "Mobile networks, streaming, downloads, UDP-friendly routes", "QUIC-based. Often faster on weak or high-latency networks.", "Requires a domain certificate; selected by default and auto-configured when possible.", true],
    "trojan-tls": ["Compatibility", "Trojan TLS", "4/5", "Almost every mainstream proxy client", "Friends, family, older clients, minimal troubleshooting", "Looks close to normal HTTPS and has excellent client support.", "Requires a domain and TLS certificate; selected by default and auto-configured when possible.", true],
    "vless-ws-tls": ["CDN route", "VLESS WS TLS", "3/5", "v2rayN, Clash Meta, Shadowrocket", "Cloudflare/CDN, reverse proxy, hiding origin IP", "WebSocket over TLS, useful behind CDN or reverse proxy.", "Usually not the fastest and has a longer config chain.", false],
    "tuic": ["Backup boost", "TUIC v5", "3/5", "Nekoray, sing-box, selected mobile clients", "A second QUIC route when UDP works well", "Modern QUIC protocol, useful as a backup node.", "Client coverage is lower than Trojan and VLESS.", false]
  }
};

function t(key) { return copyText[lang][key]; }
function profile(id) {
  const p = profiles[lang][id];
  return { title: p[0], protocol: p[1], score: p[2], clients: p[3], bestFor: p[4], summary: p[5], tradeoff: p[6], enabled: p[7] };
}
function allProfiles() { return Object.keys(profiles.en).map(id => [id, profile(id)]); }

async function api(path, opts = {}) {
  const res = await fetch(path, { headers: { "Content-Type": "application/json" }, ...opts });
  if (!res.ok) {
    let msg = t("requestFailed");
    try { msg = (await res.json()).error || msg; } catch {}
    throw new Error(msg);
  }
  return res.json();
}

function switchLang() {
  lang = lang === "zh" ? "en" : "zh";
  localStorage.setItem("easynode_lang", lang);
  boot();
}

function langButton() {
  return `<button class="btn ghost langBtn" id="langBtn">${t("language")}</button>`;
}

function toast(text) {
  clearTimeout(toastTimer);
  let el = $(".toast");
  if (!el) {
    el = document.createElement("div");
    el.className = "toast";
    document.body.appendChild(el);
  }
  el.textContent = text;
  toastTimer = setTimeout(() => el.remove(), 2200);
}

function protocolName(p) { return profile(p)?.protocol || p; }
function stars(score) {
  const n = Number(String(score).split("/")[0]) || 0;
  return `<span class="stars" aria-label="${score}">${"★".repeat(n)}${"☆".repeat(5 - n)}</span>`;
}

async function boot() {
  const setup = await api("/api/v1/setup/status");
  if (!setup.setup_done) return renderSetup(setup);
  try {
    state = await api("/api/v1/state");
    renderDashboard();
  } catch {
    renderLogin();
  }
}

function renderSetup(setup) {
  app.innerHTML = `<main class="shell"><section class="panel wide">
    <div class="top miniTop"><div class="brand"><div class="mark"><span></span></div><div><h1>EasyNode</h1><p>${t("setupIntro")}</p></div></div><div class="actions">${langButton()}</div></div>
    <div class="setupHint"><b>${t("setupHintTitle")}</b><span>${t("setupHintText")}</span></div>
    <div class="steps">
      <div>
        <div class="field"><label>${t("adminPassword")}</label><input id="password" type="password" minlength="8" placeholder="${t("passwordPlaceholder")}"></div>
        <div class="field"><label>${t("panelPath")}</label><input id="panelPath" value="${setup.panel_path}"></div>
      </div>
      <div>
        <div class="field"><label>${t("domain")}</label><input id="domain" placeholder="example.com"></div>
        <div class="modeSwitch">
          <button class="modeBtn active" type="button" id="domainMode"><b>${t("domainMode")}</b><span>${t("domainModeText")}</span></button>
          <button class="modeBtn" type="button" id="ipMode"><b>${t("ipMode")}</b><span>${t("ipModeText")}</span></button>
        </div>
        <input id="ipDirect" type="checkbox" hidden>
      </div>
    </div>
    <div class="sectionTitle"><span>${t("nodePlan")}</span><button class="btn ghost" id="selectRecommended">${t("restoreRecommended")}</button></div>
    <div class="protocolGrid">${allProfiles().map(([id, p]) => protocolOption(id, p)).join("")}</div>
    <button class="btn primary big" id="setupBtn">${t("setupButton")}</button>
    <div class="notice">${t("setupNotice")}</div>
    <div id="err" class="error"></div>
  </section></main>`;
  $("#langBtn").onclick = switchLang;
  $("#selectRecommended").onclick = () => {
    applyProtocolAvailability();
  };
  $("#domainMode").onclick = () => setIPMode(false);
  $("#ipMode").onclick = () => setIPMode(true);
  applyProtocolAvailability();
  $("#setupBtn").onclick = async () => {
    $("#setupBtn").disabled = true;
    $("#setupBtn").textContent = t("deploying");
    try {
      const protocols = [...document.querySelectorAll("input[name=proto]:checked")].map(x => x.value);
      state = await api("/api/v1/setup", { method: "POST", body: JSON.stringify({
        password: $("#password").value,
        panel_path: $("#panelPath").value,
        domain: $("#domain").value.trim(),
        ip_direct: $("#ipDirect").checked,
        protocols
      })});
      if (state.panel_url) {
        location.href = state.panel_url;
        return;
      }
      renderDashboard();
    } catch (e) {
      $("#err").textContent = e.message;
      $("#setupBtn").disabled = false;
      $("#setupBtn").textContent = t("setupButton");
    }
  };
}

function setIPMode(enabled) {
  $("#ipDirect").checked = enabled;
  $("#domainMode").classList.toggle("active", !enabled);
  $("#ipMode").classList.toggle("active", enabled);
  $("#domain").disabled = enabled;
  applyProtocolAvailability();
}

function applyProtocolAvailability() {
  const ipMode = $("#ipDirect")?.checked;
  document.querySelectorAll("input[name=proto]").forEach(x => {
    const directOnly = x.value === "vless-reality";
    x.disabled = ipMode && !directOnly;
    x.checked = ipMode ? directOnly : profile(x.value).enabled;
    const card = x.closest(".protocolOption");
    if (card) card.classList.toggle("disabled", x.disabled);
  });
}

function protocolOption(id, p) {
  return `<label class="protocolOption">
    <input type="checkbox" name="proto" value="${id}" ${p.enabled ? "checked" : ""}>
    <span class="optionBody">
      <span class="optionTop"><strong>${p.title}</strong><em>${t("recommendation")} ${stars(p.score)}</em></span>
      <span class="optionProtocol">${p.protocol}</span>
      <span class="optionText">${p.summary}</span>
      <span class="optionMeta"><b>${t("bestFor")}</b>${p.bestFor}</span>
      <span class="optionMeta"><b>${t("clients")}</b>${p.clients}</span>
      <span class="optionTrade">${p.tradeoff}</span>
    </span>
  </label>`;
}

function renderLogin() {
  app.innerHTML = `<main class="shell loginShell"><section class="loginPanel">
    <div class="loginHero">
      <div class="brand"><div class="mark large"><span></span></div><div><h1>EasyNode</h1><p>${t("securePanelText")}</p></div></div>
      <div class="loginPulse"><i></i><i></i><i></i></div>
    </div>
    <div class="loginForm">
      <div class="dialogHead"><div><h2>${t("securePanel")}</h2><p>${t("loginIntro")}</p></div>${langButton()}</div>
      <div class="field"><label>${t("adminPassword")}</label><input id="password" type="password" autofocus></div>
      <button class="btn primary big" id="loginBtn">${t("loginButton")}</button><div id="err" class="error"></div>
    </div>
  </section></main>`;
  $("#langBtn").onclick = switchLang;
  $("#loginBtn").onclick = async () => {
    try {
      await api("/api/v1/login", { method: "POST", body: JSON.stringify({ password: $("#password").value }) });
      state = await api("/api/v1/state");
      renderDashboard();
    } catch (e) {
      $("#err").textContent = e.message;
    }
  };
}

function chainPeersHTML() {
  const peers = state.chain_peers || [];
  if (!peers.length) return `<div class="notice">${t("noPeers")}</div>`;
  return peers.map(p => {
    const endpoint = p.endpoint || "-";
    const host = shortHost(endpoint);
    const name = shortPeerName(p.name, endpoint);
    return `<div class="chainPeer"><div><b>${esc(name)}</b><span>${esc(host)}</span></div><span class="chainPeerActions"><em>${p.status === "paired" ? t("paired") : esc(p.status || "-")}</em><button class="btn ghost" data-unpair="${esc(p.id)}">${t("unpair")}</button></span></div>`;
  }).join("");
}

function shortPeerName(name, endpoint) {
  if (!name || name.length > 42 || name.startsWith("Exit ENPAIR-")) return "Exit " + shortHost(endpoint);
  return name;
}

function shortHost(raw) {
  try {
    const u = new URL(raw);
    return u.hostname || raw;
  } catch {
    return raw.length > 42 ? raw.slice(0, 18) + "..." + raw.slice(-10) : raw;
  }
}

function esc(s) {
  return String(s || "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function renderDashboard() {
  const sub = `${location.origin}/api/v1/subscribe/${state.subscribe_key}`;
  const runningCount = state.nodes.filter(n => n.status === "running").length;
  app.innerHTML = `<main class="shell">
    <header class="top dashboardTop">
      <div class="brandBlock">
        <div class="brand"><div class="mark"><span></span></div><div><h1>EasyNode</h1><p>${state.domain || "IP direct"}</p></div></div>
        <div class="topMeta"><span>${runningCount}/${state.nodes.length} ${t("running")}</span><span>${t("panelPath")} ${state.panel_path}</span></div>
      </div>
      <div class="actionPanel">
        <div class="actionGroup primaryGroup"><button class="btn primary" id="copySub">${t("copySub")}</button><button class="btn" id="subQR">${t("subQR")}</button></div>
        <div class="actionGroup"><button class="btn" id="purityBtn">${t("ipPurity")}</button><button class="btn" id="protocolsBtn">${t("addProtocol")}</button><button class="btn" id="settingsBtn">${t("settings")}</button></div>
        <div class="actionGroup subtleGroup">${langButton()}<button class="btn" id="downloadCfg">${t("config")}</button><button class="btn ghost" id="logout">${t("logout")}</button></div>
      </div>
    </header>
    <section class="grid">${state.nodes.map(nodeCard).join("")}</section>
    <section class="split chainGrid" style="margin-top:16px">
      <div class="card chainCard"><div class="cardHead"><div class="proto">${t("chainProxy")}</div><span class="badge chainAccept">${state.chain_pairing_disabled ? t("pairingClosed") : t("acceptingPairing")}</span></div><p>${t("chainText")}</p><div class="row chainRow" style="margin-top:14px"><input id="genEndpoint" placeholder="${t("endpointPlaceholder")}" value="${location.origin}"><button class="btn primary" id="genCode" ${state.chain_pairing_disabled ? "disabled" : ""}>${t("generateCode")}</button><button class="btn" id="togglePairing">${state.chain_pairing_disabled ? t("openPairing") : t("closePairing")}</button></div><div id="codeBox" class="notice chainNotice">1. 在落地服务器生成配对令牌。2. 到入口服务器粘贴令牌并完成配对。</div></div>
      <div class="card chainCard"><div class="proto">${t("addExit")}</div><p>${t("addExitText")}</p><div class="field"><label>${t("pairingCode")}</label><textarea class="copyBox chainTokenInput" id="pairCode" placeholder="ENPAIR-..."></textarea></div><div class="field"><label>${t("myEndpoint")}</label><input id="pairEndpoint" placeholder="${location.origin}" value="${location.origin}"></div><button class="btn primary" id="pairBtn">${t("pairDoneButton")}</button><div class="chainPeers">${chainPeersHTML()}</div></div>
    </section>
  </main>`;
  $("#langBtn").onclick = switchLang;
  $("#purityBtn").onclick = showPurity;
  $("#protocolsBtn").onclick = renderProtocolLibrary;
  $("#settingsBtn").onclick = renderSettings;
  $("#copySub").onclick = () => copy(sub, $("#copySub"));
  $("#subQR").onclick = () => showQR(sub, t("subQR"), "/api/v1/qrcode/subscribe");
  $("#downloadCfg").onclick = () => location.href = "/api/v1/sing-box/config";
  $("#logout").onclick = async () => { await api("/api/v1/logout", { method: "POST" }); renderLogin(); };
  $("#togglePairing").onclick = async () => {
    state = await api("/api/v1/chain/accepting", { method: "POST", body: JSON.stringify({ accepting: !!state.chain_pairing_disabled }) });
    renderDashboard();
  };
  $("#genCode").onclick = async () => {
    const code = await api("/api/v1/chain/generate-code", { method: "POST", body: JSON.stringify({ endpoint: $("#genEndpoint").value || location.origin }) });
    const token = code.bundle || code.code;
    $("#codeBox").innerHTML = `<b>${t("pairingCode")}</b><textarea class="copyBox" id="pairTokenBox" readonly>${token}</textarea><div class="row"><button class="btn primary" id="copyPairToken">${t("copyLink")}</button><span class="chainExpire">${new Date(code.expires_at).toLocaleString()} ${t("expired")}</span></div>`;
    $("#copyPairToken").onclick = () => copy(token, $("#copyPairToken"));
  };
  $("#pairBtn").onclick = async () => {
    try {
      await api("/api/v1/chain/pair", { method: "POST", body: JSON.stringify({ code: $("#pairCode").value.trim(), my_endpoint: $("#pairEndpoint").value || location.origin, my_public_key: crypto.randomUUID(), display_name: "" }) });
      state = await api("/api/v1/state");
      toast(t("chainActive"));
      renderDashboard();
    } catch (e) {
      toast(e.message);
    }
  };
  document.querySelectorAll("[data-unpair]").forEach(b => b.onclick = async () => {
    try {
      state = await api("/api/v1/chain/remove", { method: "POST", body: JSON.stringify({ id: b.dataset.unpair }) });
      toast(t("chainRemoved"));
      renderDashboard();
    } catch (e) {
      toast(e.message);
    }
  });
  document.querySelectorAll("[data-node-copy]").forEach(b => b.onclick = () => {
    const n = state.nodes.find(x => x.id === b.dataset.nodeCopy);
    copy(n?.subscribe_link || "", b);
  });
  document.querySelectorAll("[data-node-qr]").forEach(b => b.onclick = () => {
    const n = state.nodes.find(x => x.id === b.dataset.nodeQr);
    showQR(n?.subscribe_link || "", n ? protocolName(n.protocol) : "", n ? `/api/v1/qrcode/node/${encodeURIComponent(n.id)}` : "");
  });
  document.querySelectorAll("[data-toggle]").forEach(b => b.onclick = async () => {
    try {
      state.nodes = await api(`/api/v1/nodes/${b.dataset.toggle}/toggle`, { method: "POST" });
      state = await api("/api/v1/state");
      renderDashboard();
    } catch (e) {
      toast(e.message || t("unavailable"));
    }
  });
}

function renderProtocolLibrary() {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `<div class="dialog">
    <div class="dialogHead"><div><h2>${t("protocolLibrary")}</h2><p>${t("setupHintText")}</p></div><button class="btn ghost" id="closeProtocols">${t("cancel")}</button></div>
    <div class="protocolGrid" style="margin-top:14px">${allProfiles().map(([id, p]) => {
      const exists = state.nodes.some(n => n.protocol === id);
      const ready = id === "vless-reality" || (!state.ip_direct && state.cert_ready);
      return `<div class="protocolOption ${exists ? "exists" : ""}">
        <span></span><span class="optionBody">
          <span class="optionTop"><strong>${p.title}</strong><em class="starBadge">${stars(p.score)}</em></span>
          <span class="optionProtocol">${p.protocol}</span>
          <span class="optionText">${p.summary}</span>
          <span class="optionMeta"><b>${t("bestFor")}</b>${p.bestFor}</span>
          <span class="optionTrade">${ready ? t("availableNow") : t("comingSoon")}</span>
          ${exists ? `<button class="btn" data-remove-protocol="${id}">${t("remove")}</button>` : `<button class="btn" ${!ready ? "disabled" : ""} data-add-protocol="${id}">${ready ? t("add") : t("comingSoon")}</button>`}
        </span>
      </div>`;
    }).join("")}</div>
  </div>`;
  document.body.appendChild(modal);
  $("#closeProtocols").onclick = () => modal.remove();
  modal.querySelectorAll("[data-add-protocol]").forEach(btn => btn.onclick = async () => {
    btn.disabled = true;
    try {
      state = await api("/api/v1/nodes/add", { method: "POST", body: JSON.stringify({ protocol: btn.dataset.addProtocol }) });
      toast(t("addSuccess"));
      modal.remove();
      renderDashboard();
    } catch (e) {
      toast(e.message);
      btn.disabled = false;
    }
  });
  modal.querySelectorAll("[data-remove-protocol]").forEach(btn => btn.onclick = async () => {
    btn.disabled = true;
    try {
      state = await api("/api/v1/nodes/remove", { method: "POST", body: JSON.stringify({ protocol: btn.dataset.removeProtocol }) });
      toast(t("removeSuccess"));
      modal.remove();
      renderDashboard();
    } catch (e) {
      toast(e.message);
      btn.disabled = false;
    }
  });
}

function renderSettings() {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `<div class="dialog">
    <div class="dialogHead"><div><h2>${t("settingsTitle")}</h2><p>${state.domain || "IP direct"}</p></div><button class="btn ghost" id="closeSettings">${t("cancel")}</button></div>
    <div class="steps">
      <div>
        <div class="field"><label>${t("currentPassword")}</label><input id="setCurrentPassword" type="password"></div>
        <div class="field"><label>${t("newPassword")}</label><input id="setNewPassword" type="password" placeholder="${t("passwordPlaceholder")}"></div>
      </div>
      <div>
        <div class="field"><label>${t("panelPath")}</label><input id="setPanelPath" value="${state.panel_path}"></div>
        <div class="field"><label>${t("domain")}</label><input id="setDomain" value="${state.domain || ""}" placeholder="example.com"></div>
        <label class="check compact"><input id="setIPDirect" type="checkbox" ${state.ip_direct ? "checked" : ""}>${t("ipDirect")}</label>
      </div>
    </div>
    <button class="btn primary big" id="saveSettings">${t("saveSettings")}</button>
    <div class="notice">${t("upgradeTip")}</div>
    <button class="btn big" id="upgradeBtn">${t("onlineUpgrade")}</button>
    <div id="settingsErr" class="error"></div>
  </div>`;
  document.body.appendChild(modal);
  $("#closeSettings").onclick = () => modal.remove();
  $("#saveSettings").onclick = async () => {
    if (!$("#setCurrentPassword").value) {
      $("#settingsErr").textContent = t("currentPasswordRequired");
      return;
    }
    $("#saveSettings").disabled = true;
    try {
      state = await api("/api/v1/settings", { method: "POST", body: JSON.stringify({
        current_password: $("#setCurrentPassword").value,
        new_password: $("#setNewPassword").value,
        panel_path: $("#setPanelPath").value,
        domain: $("#setDomain").value.trim(),
        ip_direct: $("#setIPDirect").checked
      })});
      toast(t("saved"));
      modal.remove();
      renderDashboard();
    } catch (e) {
      $("#settingsErr").textContent = e.message;
      $("#saveSettings").disabled = false;
    }
  };
  $("#upgradeBtn").onclick = async () => {
    showUpgradeConfirm();
  };
}

function showUpgradeConfirm() {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `<div class="dialog confirmDialog">
    <div class="dialogHead"><div><h2>${t("upgradeConfirmTitle")}</h2><p>${t("upgradeConfirmText")}</p></div><button class="btn ghost" id="cancelUpgrade">${t("cancel")}</button></div>
    <div class="notice">${t("upgradeTip")}</div>
    <div class="row" style="margin-top:14px"><button class="btn primary" id="confirmUpgrade">${t("onlineUpgrade")}</button><button class="btn" id="cancelUpgrade2">${t("cancel")}</button></div>
  </div>`;
  document.body.appendChild(modal);
  const close = () => modal.remove();
  $("#cancelUpgrade").onclick = close;
  $("#cancelUpgrade2").onclick = close;
  $("#confirmUpgrade").onclick = async () => {
    $("#confirmUpgrade").disabled = true;
    modal.remove();
    showUpgradeModal();
    try {
      await api("/api/v1/system/upgrade", { method: "POST", body: "{}" });
      pollUpgrade();
    } catch (e) {
      toast(e.message || t("upgradeStarted"));
    }
  };
}

function showUpgradeModal() {
  const old = document.querySelector(".upgradeModal");
  if (old) old.remove();
  const modal = document.createElement("div");
  modal.className = "modal upgradeModal";
  modal.innerHTML = `<div class="dialog">
    <div class="dialogHead"><div><h2>${t("upgrading")}</h2><p>${t("upgradeTip")}</p></div><button class="btn ghost" id="closeUpgrade">${t("cancel")}</button></div>
    <div class="progressWrap"><div class="progressBar" id="upgradeBar" style="width:5%"></div></div>
    <div class="notice" id="upgradeStep">Preparing...</div>
    <pre class="upgradeLog" id="upgradeLog"></pre>
    <button class="btn primary big upgradeDoneBtn" id="upgradeDoneBtn" style="display:none">${t("upgradeDoneAction")}</button>
  </div>`;
  document.body.appendChild(modal);
  $("#closeUpgrade").onclick = () => modal.remove();
}

async function pollUpgrade() {
  try {
    const st = await api("/api/v1/system/upgrade/status");
    const bar = $("#upgradeBar");
    const step = $("#upgradeStep");
    const log = $("#upgradeLog");
    if (bar) bar.style.width = `${Math.max(5, st.progress || 5)}%`;
    if (step) step.textContent = st.error ? `${t("upgradeFailed")}: ${st.error}` : (st.progress >= 100 ? t("upgradeDone") : st.step);
    if (log && st.output) log.textContent = stripAnsi(st.output).slice(-4000);
    const doneBtn = $("#upgradeDoneBtn");
    if (doneBtn && st.progress >= 100 && !st.running && !st.error) {
      doneBtn.style.display = "block";
      doneBtn.onclick = () => { location.href = "/"; };
    }
    if (st.running) {
      setTimeout(pollUpgrade, 1500);
    }
  } catch {
    setTimeout(pollUpgrade, 2000);
  }
}

function stripAnsi(text) {
  return String(text || "").replace(/\x1b\[[0-9;]*m/g, "");
}

const purityI18n = {
  zh: {
    clean: "较干净",
    medium: "中等",
    risky: "风险较高",
    unknown: "未知",
    "datacenter": "机房 / VPS",
    "mobile": "移动网络",
    "residential/isp-like": "住宅 / 运营商倾向",
    "likely native": "较可能原生",
    "uncertain": "不确定",
    "likely proxy/broadcast": "疑似代理 / 广播",
    "data center / hosting ASN": "机房或托管 ASN",
    "proxy/VPN risk flag": "存在代理 / VPN 风险标记",
    "provider name looks like hosting": "服务商名称疑似机房",
    "reputation API unreachable": "纯净度接口暂时不可达",
    "reputation API returned no result": "纯净度接口没有返回结果",
    "AI / ChatGPT": "AI / ChatGPT",
    "Streaming": "流媒体",
    "Account registration": "账号注册",
    "Daily browsing": "日常浏览",
    good: "适合",
    usable: "可用",
    limited: "可能受限",
    caution: "谨慎使用",
    "Sensitive to proxy and abuse reputation": "对代理和滥用信誉较敏感",
    "Datacenter IPs may hit region or proxy checks": "机房 IP 可能触发地区或代理检测",
    "New accounts prefer clean ISP-like IPs": "新账号更适合干净的运营商倾向 IP",
    "Most sites tolerate normal VPS traffic": "大多数网站可接受普通 VPS 流量",
    "No obvious risk flag": "未发现明显风险标记"
  },
  en: {}
};

const countryZh = {
  "United States": "美国", "Japan": "日本", "Singapore": "新加坡", "Hong Kong": "中国香港", "Taiwan": "中国台湾",
  "South Korea": "韩国", "United Kingdom": "英国", "Germany": "德国", "France": "法国", "Canada": "加拿大",
  "Australia": "澳大利亚", "Netherlands": "荷兰", "Russia": "俄罗斯", "China": "中国", "India": "印度",
  "Thailand": "泰国", "Vietnam": "越南", "Malaysia": "马来西亚", "Indonesia": "印度尼西亚", "Philippines": "菲律宾"
};

function pt(value) {
  if (lang !== "zh") return value;
  return purityI18n.zh[value] || value;
}

function countryName(name) {
  return lang === "zh" ? (countryZh[name] || name || "") : (name || "");
}

function flagEmoji(code) {
  if (!code || code.length !== 2) return "";
  return code.toUpperCase().replace(/./g, c => String.fromCodePoint(127397 + c.charCodeAt(0)));
}

function nodeCard(n) {
  const p = profile(n.protocol);
  const canCopy = n.status === "running" && n.subscribe_link;
  const copyButton = canCopy
    ? `<button class="btn" data-node-copy="${n.id}">${t("copyLink")}</button>`
    : `<button class="btn" disabled title="${t("certRequired")}">${t("unavailable")}</button>`;
  const qrButton = canCopy ? `<button class="btn" data-node-qr="${n.id}">${t("showQR")}</button>` : "";
  return `<article class="card ${n.status}">
    <div class="cardHead"><div class="status"><i class="dot"></i>${n.status === "running" ? t("running") : t("stopped")}</div><span class="badge starBadge">${stars(p.score)}</span></div>
    <div class="proto">${p.title}</div><p class="desc">${p.summary}</p>
    <div class="miniInfo">${p.protocol} · ${p.clients || t("mainstreamClients")}</div>
    <div class="stats"><div class="stat"><b>${n.latency_ms ?? "-"}ms</b><span>${t("latency")}</span></div><div class="stat"><b>${n.port}</b><span>${t("port")}</span></div><div class="stat"><b>${formatBytes(n.traffic_used || 0)}</b><span>${t("traffic")}</span></div></div>
    <div class="row">${copyButton}${qrButton}<button class="btn" data-toggle="${n.id}">${n.status === "running" ? t("stop") : t("start")}</button></div>
  </article>`;
}

function formatBytes(bytes) {
  if (!bytes) return "0B";
  const units = ["B", "K", "M", "G", "T"];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)}${units[i]}`;
}

async function showPurity() {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `<div class="dialog"><div class="dialogHead"><div><h2>${t("ipPurity")}</h2><p>${t("purityHint")}</p></div><button class="btn ghost" id="closePurity">${t("cancel")}</button></div><div class="notice" id="purityBody">${t("checking")}</div></div>`;
  document.body.appendChild(modal);
  $("#closePurity").onclick = () => modal.remove();
  try {
    const r = await api("/api/v1/ip/purity");
    $("#purityBody").innerHTML = `<div class="purityGrid">
      <div class="purityMain"><div class="purityScore">${r.score}<span>/100</span></div><p>${pt(r.level || "")} · ${r.ip || ""}</p><p><span class="flag">${flagEmoji(r.country_code)}</span>${countryName(r.country)} ${r.asn || ""}</p><p>${r.isp || ""}</p></div>
      <div class="purityMeta"><b>${t("ipType")}</b><span>${pt(r.ip_type || "-")}</span></div>
      <div class="purityMeta"><b>${t("nativeCheck")}</b><span>${pt(r.native || "-")}</span></div>
      <div class="purityMeta wide"><b>${t("risks")}</b><span>${(r.risks || []).map(pt).join("<br>") || pt("No obvious risk flag")}</span></div>
      <div class="purityMeta wide"><b>${t("useCases")}</b>${(r.use_cases || []).map(x => `<span>${pt(x.name)}: ${pt(x.status)} - ${pt(x.reason)}</span>`).join("")}</div>
    </div>`;
  } catch (e) {
    $("#purityBody").textContent = e.message;
  }
}

function showQR(text, title, imageURL) {
  if (!text) {
    toast(t("copyFailed"));
    return;
  }
  const modal = document.createElement("div");
  modal.className = "modal";
  const src = imageURL || "";
  modal.innerHTML = `<div class="dialog qrDialog">
    <div class="dialogHead"><div><h2>${t("scanToImport")}</h2><p>${title}</p></div><button class="btn ghost" id="closeQR">${t("cancel")}</button></div>
    <div class="qrBox"><div class="qrImageWrap"><div class="qrLoading">${t("checking")}</div><img id="qrImg" src="${src}" alt="QR code"></div><textarea class="copyBox" readonly>${text}</textarea></div>
  </div>`;
  document.body.appendChild(modal);
  $("#closeQR").onclick = () => modal.remove();
  $("#qrImg").onload = () => {
    const loading = modal.querySelector(".qrLoading");
    if (loading) loading.remove();
  };
}

async function copy(text, button) {
  if (!text) {
    toast(t("copyFailed"));
    return;
  }
  const ok = await writeClipboard(text);
  if (ok) {
    toast(t("copied"));
    pulseButton(button, t("copied"));
  } else {
    showCopyFallback(text);
    toast(t("copyFailed"));
  }
}

async function writeClipboard(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {}
  }
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  ta.style.top = "0";
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    ta.remove();
  }
}

function pulseButton(button, text) {
  if (!button) return;
  const old = button.textContent;
  button.textContent = text;
  button.classList.add("success");
  setTimeout(() => {
    button.textContent = old;
    button.classList.remove("success");
  }, 1100);
}

function showCopyFallback(text) {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `<div class="dialog"><div class="dialogHead"><div><h2>${t("copyLink")}</h2><p>${t("copyFailed")}</p></div><button class="btn ghost" id="closeCopy">${t("cancel")}</button></div><textarea class="copyBox" readonly>${text}</textarea></div>`;
  document.body.appendChild(modal);
  $("#closeCopy").onclick = () => modal.remove();
  const box = modal.querySelector(".copyBox");
  box.focus();
  box.select();
}

boot().catch(e => {
  app.innerHTML = `<main class="shell"><section class="panel"><h1>EasyNode</h1><p>${e.message}</p></section></main>`;
});
