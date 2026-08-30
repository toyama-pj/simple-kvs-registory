const app = document.querySelector("#app");

const state = {
  view: "loading",
  busy: false,
  authMode: "login",
  otpSent: false,
  authEmail: "",
  authName: "",
  passkeyEnabled: false,
  me: null,
  namespaces: [],
  selectedNamespace: null,
  devices: [],
  measurements: [],
  passkeys: [],
  errors: [],
  toast: "",
};

class APIError extends Error {
  constructor(status, detail) {
    super(detail || `Request failed (${status})`);
    this.status = status;
  }
}

async function api(path, { method = "GET", json, body, headers = {} } = {}) {
  const requestHeaders = { Accept: "application/json", ...headers };
  let requestBody = body;
  if (json !== undefined) {
    requestHeaders["Content-Type"] = "application/json";
    requestBody = JSON.stringify(json);
  }
  const response = await fetch(`/api/v1${path}`, {
    method,
    headers: requestHeaders,
    body: requestBody,
    credentials: "same-origin",
  });
  if (response.status === 204) return null;
  const contentType = response.headers.get("content-type") || "";
  const payload = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const detail = typeof payload === "object" ? payload.detail || payload.title : payload;
    throw new APIError(response.status, detail);
  }
  return payload;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function setBusy(value) {
  state.busy = value;
  render();
}

function addError(error) {
  const message = error instanceof Error ? error.message : String(error);
  state.errors = [...state.errors.slice(-3), message];
  render();
}

function showToast(message) {
  state.toast = message;
  render();
  window.setTimeout(() => {
    if (state.toast === message) {
      state.toast = "";
      render();
    }
  }, 2800);
}

function bind(id, event, handler) {
  document.querySelector(`#${id}`)?.addEventListener(event, handler);
}

function errorBanner() {
  if (!state.errors.length) return "";
  return `<div class="alert" role="alert">${state.errors.length} 件のエラーがあります<div class="alert-detail">${escapeHTML(state.errors.at(-1))}</div></div>`;
}

function toast() {
  return state.toast ? `<div class="toast" role="status">${escapeHTML(state.toast)}</div>` : "";
}

function header(title, { back = false, actions = "" } = {}) {
  return `<header class="app-header">
    <div>${back ? '<button id="back" class="icon-button back-button" aria-label="戻る">‹</button>' : ""}</div>
    <h1>${escapeHTML(title)}</h1>
    <div class="header-actions">${actions}</div>
  </header>`;
}

function render() {
  switch (state.view) {
    case "login": renderLogin(); break;
    case "dashboard": renderDashboard(); break;
    case "namespace": renderNamespace(); break;
    case "add-device": renderAddDevice(); break;
    case "passkeys": renderPasskeys(); break;
    default:
      app.innerHTML = '<section class="center-state"><div class="spinner" aria-hidden="true"></div><p>読み込んでいます</p></section>';
  }
}

function renderLogin() {
  const isRegister = state.authMode === "register";
  const passkeyAvailable = state.passkeyEnabled && window.PublicKeyCredential && navigator.credentials;
  app.innerHTML = `<section class="auth-shell">
    <div class="auth-card">
      <div class="brand"><div class="brand-mark" aria-hidden="true">◉</div><h1>Simple Chirp</h1><p>LoRaWAN センサーダッシュボード</p></div>
      ${errorBanner()}
      ${!isRegister && passkeyAvailable ? `<button id="passkey-login" class="button full" ${state.busy ? "disabled" : ""}>パスキーでログイン</button><div class="divider">またはメールコード</div>` : ""}
      <form id="otp-form" class="form-grid">
        ${isRegister ? `<div class="field"><label for="name">お名前</label><input id="name" name="name" autocomplete="name" maxlength="100" value="${escapeHTML(state.authName)}" required></div>` : ""}
        <div class="field"><label for="email">メールアドレス</label><input id="email" name="email" type="email" autocomplete="email" maxlength="254" value="${escapeHTML(state.authEmail)}" required></div>
        ${state.otpSent ? '<div class="field"><label for="code">6桁の確認コード</label><input id="code" name="code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" required><small>メールに届いたコードを入力してください。</small></div>' : ""}
        <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "処理中…" : state.otpSent ? (isRegister ? "アカウントを作成" : "ログイン") : "確認コードを送る"}</button>
      </form>
      <div class="auth-switch">${isRegister ? "アカウントをお持ちですか？" : "初めて利用しますか？"} <button id="auth-switch" class="text-button">${isRegister ? "ログイン" : "新規登録"}</button></div>
    </div>
    ${toast()}
  </section>`;

  bind("auth-switch", "click", () => {
    state.authMode = isRegister ? "login" : "register";
    state.otpSent = false;
    state.authEmail = "";
    state.authName = "";
    state.errors = [];
    render();
  });
  bind("otp-form", "submit", submitOTPForm);
  bind("passkey-login", "click", loginWithPasskey);
}

async function submitOTPForm(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const email = String(form.get("email") || "").trim();
  const name = String(form.get("name") || "").trim();
  const code = String(form.get("code") || "").trim();
  state.authEmail = email;
  state.authName = name;
  setBusy(true);
  try {
    if (!state.otpSent) {
      const endpoint = state.authMode === "register" ? "/auth/register" : "/auth/login";
      await api(endpoint, { method: "POST", json: state.authMode === "register" ? { name, email } : { email } });
      state.otpSent = true;
      state.toast = "確認コードを送信しました";
      state.errors = [];
    } else {
      const endpoint = state.authMode === "register" ? "/auth/register/callback" : "/auth/login/callback";
      await api(endpoint, { method: "POST", json: { email, code } });
      state.otpSent = false;
      await loadAuthenticatedApp();
      return;
    }
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
}

async function loginWithPasskey() {
  setBusy(true);
  try {
    const begin = await api("/auth/passkeys/login/begin", { method: "POST" });
    const publicKey = parseRequestOptions(begin.options.publicKey);
    const credential = await navigator.credentials.get({ publicKey });
    await api("/auth/passkeys/login/finish", {
      method: "POST",
      body: JSON.stringify(credentialToJSON(credential)),
      headers: { "Content-Type": "application/json", "X-Passkey-Ceremony-ID": begin.ceremony_id },
    });
    await loadAuthenticatedApp();
  } catch (error) {
    if (error?.name !== "NotAllowedError") addError(error);
  } finally {
    state.busy = false;
    render();
  }
}

async function loadAuthenticatedApp() {
  const [me, namespaces] = await Promise.all([api("/cfg/me"), api("/cfg/me/namespace?limit=50")]);
  state.me = me;
  state.namespaces = Array.isArray(namespaces) ? namespaces : [];
  state.view = "dashboard";
  state.errors = [];
  render();
  await loadNamespaceSummaries();
}

async function loadNamespaceSummaries() {
  await Promise.all(state.namespaces.slice(0, 20).map(async namespace => {
    try {
      const response = await api(`/namespaces/${namespace.namespace_id}/measurements?limit=1`);
      namespace.latestAt = response.data?.[0]?.received_at || null;
    } catch (_) {
      namespace.latestAt = null;
    }
  }));
  if (state.view === "dashboard") render();
}

function renderDashboard() {
  const name = state.me?.Name || state.me?.name || "あなた";
  app.innerHTML = `${header(`${name} の Device / Namespace`, {
    actions: '<button id="passkeys" class="button secondary small">パスキー</button><button id="logout" class="button small">ログアウト</button>',
  })}
  ${errorBanner()}
  ${state.namespaces.length ? `<div class="namespace-list">${state.namespaces.map((namespace, index) => `<button class="list-row namespace-row" data-index="${index}"><strong>${escapeHTML(namespace.name || "Namespace")}</strong><span class="list-meta">${namespace.latestAt ? `最終更新: ${escapeHTML(formatAgo(namespace.latestAt))}` : "データなし"}</span><span class="chevron">›</span></button>`).join("")}</div>` : `<section class="empty-state"><div><h2>Namespace がありません</h2><p>最初のNamespaceとPersonal Organizationを作成して、Deviceを登録しましょう。</p><button id="create-namespace" class="button" ${state.busy ? "disabled" : ""}>${state.busy ? "作成中…" : "Namespaceを作成"}</button></div></section>`}
  ${toast()}`;
  document.querySelectorAll(".namespace-row").forEach(row => row.addEventListener("click", () => openNamespace(Number(row.dataset.index))));
  bind("passkeys", "click", openPasskeys);
  bind("logout", "click", logout);
  bind("create-namespace", "click", createDefaultNamespace);
}

async function createDefaultNamespace() {
  setBusy(true);
  try {
    await api("/cfg/me/namespace/create", { method: "POST" });
    state.namespaces = await api("/cfg/me/namespace?limit=50");
    state.errors = [];
    showToast("Namespaceを作成しました");
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
}

async function openNamespace(index) {
  state.selectedNamespace = state.namespaces[index];
  state.view = "loading";
  render();
  try {
    const id = state.selectedNamespace.namespace_id;
    const [devices, measurements] = await Promise.all([
      api(`/namespaces/${id}/devices`),
      api(`/namespaces/${id}/measurements?limit=500`),
    ]);
    state.devices = devices.data || [];
    state.measurements = measurements.data || [];
    state.view = "namespace";
  } catch (error) {
    state.view = "dashboard";
    addError(error);
  }
  render();
}

function renderNamespace() {
  const namespace = state.selectedNamespace;
  const metrics = groupMetrics(state.measurements);
  app.innerHTML = `${header(`${namespace?.name || "Namespace"}⌄`, {
    back: true,
    actions: '<button id="logout" class="button small">ログアウト</button>',
  })}
  ${errorBanner()}
  <div class="toolbar"><span>${state.devices.length} Device / ${metrics.length} Metrics</span><button id="add-device" class="button secondary small">＋ Device</button></div>
  ${metrics.length ? `<div class="metrics">${metrics.map(metricCard).join("")}</div>` : `<section class="empty-state"><div><h2>計測データがありません</h2><p>Deviceを追加し、Cayenne LPP uplinkを送信してください。</p><button id="empty-add-device" class="button">Deviceを追加</button></div></section>`}
  ${toast()}`;
  bind("back", "click", () => { state.view = "dashboard"; render(); });
  bind("logout", "click", logout);
  bind("add-device", "click", () => { state.view = "add-device"; render(); });
  bind("empty-add-device", "click", () => { state.view = "add-device"; render(); });
}

function groupMetrics(measurements) {
  const groups = new Map();
  for (const measurement of measurements) {
    const key = `${measurement.device_id}:${measurement.channel}:${measurement.name}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(measurement);
  }
  return [...groups.values()].map(values => ({ latest: values[0], values })).sort((a, b) => new Date(b.latest.received_at) - new Date(a.latest.received_at));
}

const metricLabels = {
  temperature: ["気温", "°C"], relative_humidity: ["湿度", "%"], barometric_pressure: ["気圧", "hPa"],
  illuminance: ["照度", "lx"], presence: ["人感", ""], digital_input: ["デジタル入力", ""],
  digital_output: ["デジタル出力", ""], analog_input: ["アナログ入力", ""], analog_output: ["アナログ出力", ""],
  accelerometer: ["加速度", "g"], gyrometer: ["角速度", "°/s"], gps: ["位置", ""],
};

function metricCard(metric) {
  const measurement = metric.latest;
  const device = state.devices.find(item => item.id === measurement.device_id);
  const [label, unit] = metricLabels[measurement.name] || [measurement.name, ""];
  const value = formatValue(measurement.value);
  return `<article class="metric-card">
    <div><div class="metric-title">${escapeHTML(label)}${unit ? ` (${escapeHTML(unit)})` : ""}</div><div class="metric-value">${escapeHTML(value)}${unit ? `<span class="metric-unit">${escapeHTML(unit)}</span>` : ""}</div><div class="metric-meta"><span class="device-eui">${escapeHTML(device?.dev_eui || device?.name || "Device")}</span> - ${escapeHTML(formatAgo(measurement.received_at))}</div></div>
    ${sparkline(metric.values)}<span class="chevron">›</span>
  </article>`;
}

function formatValue(value) {
  if (typeof value === "number") return value.toFixed(1);
  if (typeof value === "boolean") return value ? "ON" : "OFF";
  if (value && typeof value === "object") return Object.values(value).map(item => typeof item === "number" ? item.toFixed(2) : item).join(" / ");
  return String(value ?? "--");
}

function sparkline(values) {
  const numbers = values.map(item => typeof item.value === "number" ? item.value : null).filter(value => value !== null).slice(0, 30).reverse();
  if (numbers.length < 2) return '<svg class="sparkline" viewBox="0 0 180 70" aria-hidden="true"><polyline points="5,50 175,50"></polyline><circle cx="175" cy="50" r="6"></circle></svg>';
  const min = Math.min(...numbers);
  const max = Math.max(...numbers);
  const range = max - min || 1;
  const points = numbers.map((value, index) => `${5 + (170 * index / (numbers.length - 1))},${62 - ((value - min) / range) * 52}`).join(" ");
  const last = points.split(" ").at(-1).split(",");
  return `<svg class="sparkline" viewBox="0 0 180 70" aria-hidden="true"><polyline points="${points}"></polyline><circle cx="${last[0]}" cy="${last[1]}" r="6"></circle></svg>`;
}

function renderAddDevice() {
  app.innerHTML = `${header("Device を追加", { back: true })}${errorBanner()}
  <section class="form-card">
    <form id="device-form" class="form-grid">
      <div class="field"><label for="device-name">表示名</label><input id="device-name" name="name" maxlength="100" placeholder="Greenhouse Sensor" required></div>
      <div class="field"><label for="dev-eui">DevEUI</label><input id="dev-eui" name="dev_eui" autocomplete="off" maxlength="16" pattern="[0-9a-fA-F]{16}" placeholder="0102030405060708" required></div>
      <div class="field"><label for="dev-addr">DevAddr</label><input id="dev-addr" name="dev_addr" autocomplete="off" maxlength="8" pattern="[0-9a-fA-F]{8}" placeholder="26011BDA" required></div>
      <div class="field"><label for="app-key">AppSKey</label><input id="app-key" name="app_s_key" type="password" autocomplete="new-password" maxlength="32" pattern="[0-9a-fA-F]{32}" required></div>
      <div class="field"><label for="nwk-key">NwkSKey</label><input id="nwk-key" name="nwk_s_key" type="password" autocomplete="new-password" maxlength="32" pattern="[0-9a-fA-F]{32}" required><small>キーは暗号化して保存され、画面やログには再表示されません。</small></div>
      <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "追加中…" : "Deviceを追加"}</button>
    </form>
  </section>${toast()}`;
  bind("back", "click", () => { state.view = "namespace"; render(); });
  bind("device-form", "submit", addDevice);
}

async function addDevice(event) {
  event.preventDefault();
  const values = Object.fromEntries(new FormData(event.currentTarget));
  setBusy(true);
  try {
    await api(`/namespaces/${state.selectedNamespace.namespace_id}/devices`, { method: "POST", json: values });
    showToast("Deviceを追加しました");
    await openNamespace(state.namespaces.indexOf(state.selectedNamespace));
  } catch (error) {
    state.view = "add-device";
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
}

async function openPasskeys() {
  state.view = "loading";
  render();
  try {
    const response = await api("/auth/passkeys");
    state.passkeys = response.data || [];
    state.view = "passkeys";
  } catch (error) {
    state.view = "dashboard";
    addError(error);
  }
  render();
}

function renderPasskeys() {
  const supported = state.passkeyEnabled && window.PublicKeyCredential && navigator.credentials;
  app.innerHTML = `${header("パスキー", { back: true })}${errorBanner()}
  <div class="info-box"><span class="status-dot"></span>パスキーは端末の生体認証・PINでログインします。秘密鍵がこのサーバーへ送信されることはありません。</div>
  <h2 class="section-title">登録済み</h2>
  <div class="passkey-list">${state.passkeys.length ? state.passkeys.map(item => `<div class="passkey-row"><div><strong>${escapeHTML(item.name)}</strong><small>登録: ${escapeHTML(formatDate(item.created_at))}${item.last_used_at ? ` / 最終利用: ${escapeHTML(formatAgo(item.last_used_at))}` : ""}</small></div><button class="button danger small delete-passkey" data-id="${escapeHTML(item.id)}">削除</button></div>`).join("") : '<div class="empty-state"><p>登録済みパスキーはありません。</p></div>'}</div>
  ${supported ? `<form id="passkey-form" class="form-grid"><div class="field"><label for="passkey-name">新しいパスキーの名前</label><input id="passkey-name" name="name" maxlength="100" value="この端末" required></div><button class="button full" ${state.busy ? "disabled" : ""}>${state.busy ? "登録中…" : "パスキーを登録"}</button></form>` : '<div class="alert">このブラウザではパスキーを利用できません</div>'}
  ${toast()}`;
  bind("back", "click", () => { state.view = "dashboard"; render(); });
  bind("passkey-form", "submit", registerPasskey);
  document.querySelectorAll(".delete-passkey").forEach(button => button.addEventListener("click", () => deletePasskey(button.dataset.id)));
}

async function registerPasskey(event) {
  event.preventDefault();
  const name = String(new FormData(event.currentTarget).get("name") || "この端末");
  setBusy(true);
  try {
    const begin = await api("/auth/passkeys/register/begin", { method: "POST", json: { name } });
    const publicKey = parseCreationOptions(begin.options.publicKey);
    const credential = await navigator.credentials.create({ publicKey });
    await api("/auth/passkeys/register/finish", {
      method: "POST",
      body: JSON.stringify(credentialToJSON(credential)),
      headers: { "Content-Type": "application/json", "X-Passkey-Ceremony-ID": begin.ceremony_id },
    });
    const response = await api("/auth/passkeys");
    state.passkeys = response.data || [];
    showToast("パスキーを登録しました");
  } catch (error) {
    if (error?.name !== "NotAllowedError") addError(error);
  } finally {
    state.busy = false;
    state.view = "passkeys";
    render();
  }
}

async function deletePasskey(id) {
  if (!window.confirm("このパスキーを削除しますか？")) return;
  try {
    await api(`/auth/passkeys/${id}`, { method: "DELETE" });
    state.passkeys = state.passkeys.filter(item => item.id !== id);
    showToast("パスキーを削除しました");
  } catch (error) {
    addError(error);
  }
}

async function logout() {
  try { await api("/auth/logout", { method: "POST" }); } catch (_) { /* clear local view regardless */ }
  state.me = null;
  state.namespaces = [];
  state.selectedNamespace = null;
  state.errors = [];
  state.view = "login";
  render();
}

function parseCreationOptions(options) {
  if (window.PublicKeyCredential?.parseCreationOptionsFromJSON) return PublicKeyCredential.parseCreationOptionsFromJSON(options);
  const parsed = structuredClone(options);
  parsed.challenge = fromBase64URL(parsed.challenge);
  parsed.user.id = fromBase64URL(parsed.user.id);
  parsed.excludeCredentials = (parsed.excludeCredentials || []).map(item => ({ ...item, id: fromBase64URL(item.id) }));
  return parsed;
}

function parseRequestOptions(options) {
  if (window.PublicKeyCredential?.parseRequestOptionsFromJSON) return PublicKeyCredential.parseRequestOptionsFromJSON(options);
  const parsed = structuredClone(options);
  parsed.challenge = fromBase64URL(parsed.challenge);
  parsed.allowCredentials = (parsed.allowCredentials || []).map(item => ({ ...item, id: fromBase64URL(item.id) }));
  return parsed;
}

function credentialToJSON(credential) {
  const response = {};
  for (const key of ["clientDataJSON", "attestationObject", "authenticatorData", "signature", "userHandle"]) {
    if (credential.response[key] instanceof ArrayBuffer) response[key] = toBase64URL(credential.response[key]);
    else if (credential.response[key] === null) response[key] = null;
  }
  if (credential.response.getTransports) response.transports = credential.response.getTransports();
  return {
    id: credential.id,
    rawId: toBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    response,
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}

function fromBase64URL(value) {
  const padding = "=".repeat((4 - value.length % 4) % 4);
  const binary = atob(value.replaceAll("-", "+").replaceAll("_", "/") + padding);
  return Uint8Array.from(binary, character => character.charCodeAt(0)).buffer;
}

function toBase64URL(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

function formatAgo(value) {
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 10) return "数秒前";
  if (seconds < 60) return `${seconds} 秒前`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分前`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 時間前`;
  return `${Math.floor(seconds / 86400)} 日前`;
}

function formatDate(value) {
  return new Intl.DateTimeFormat("ja-JP", { year: "numeric", month: "short", day: "numeric" }).format(new Date(value));
}

async function boot() {
  try {
    const config = await api("/auth/passkeys/config");
    state.passkeyEnabled = Boolean(config.enabled);
  } catch (_) {
    state.passkeyEnabled = false;
  }
  try {
    await loadAuthenticatedApp();
  } catch (error) {
    if (!(error instanceof APIError && error.status === 401)) state.errors = [error.message];
    state.view = "login";
    render();
  }
}

boot();
