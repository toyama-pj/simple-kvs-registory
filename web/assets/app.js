const app = document.querySelector("#app");

const state = {
  view: "loading",
  busy: false,
  authMode: "login",
  authMethod: "password",
  otpSent: false,
  authEmail: "",
  authName: "",
  passkeyEnabled: false,
  me: null,
  namespaces: [],
  selectedNamespace: null,
  devices: [],
  measurements: [],
  namespaceMembers: [],
  passkeys: [],
  passwordConfigured: false,
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
    case "namespace-members": renderNamespaceMembers(); break;
    case "settings": renderSettings(); break;
    default:
      app.innerHTML = '<section class="center-state"><div class="spinner" aria-hidden="true"></div><p>読み込んでいます</p></section>';
  }
}

function renderLogin() {
  const isRegister = state.authMode === "register";
  const usePassword = !isRegister && state.authMethod === "password";
  const passkeyAvailable = state.passkeyEnabled && window.PublicKeyCredential && navigator.credentials;
  app.innerHTML = `<section class="auth-shell">
    <div class="auth-card">
      <div class="brand"><div class="brand-mark" aria-hidden="true">◉</div><h1>Simple Chirp</h1><p>LoRaWAN センサーダッシュボード</p></div>
      ${errorBanner()}
      ${!isRegister && passkeyAvailable ? `<button id="passkey-login" class="button full" ${state.busy ? "disabled" : ""}>パスキーでログイン</button><div class="divider">または</div>` : ""}
      ${!isRegister ? `<div class="segmented" aria-label="ログイン方法"><button id="use-password" type="button" class="${usePassword ? "active" : ""}" aria-pressed="${usePassword}">パスワード</button><button id="use-code" type="button" class="${usePassword ? "" : "active"}" aria-pressed="${!usePassword}">メールコード</button></div>` : ""}
      ${usePassword ? `<form id="password-login-form" class="form-grid">
        <div class="field"><label for="email">メールアドレス</label><input id="email" name="email" type="email" autocomplete="username" maxlength="254" value="${escapeHTML(state.authEmail)}" required></div>
        <div class="field"><label for="login-password">パスワード</label><input id="login-password" name="password" type="password" autocomplete="current-password" maxlength="128" required></div>
        <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "ログイン中…" : "ログイン"}</button>
      </form>` : `<form id="otp-form" class="form-grid">
        ${isRegister ? `<div class="field"><label for="name">お名前</label><input id="name" name="name" autocomplete="name" maxlength="100" value="${escapeHTML(state.authName)}" required></div>` : ""}
        <div class="field"><label for="email">メールアドレス</label><input id="email" name="email" type="email" autocomplete="email" maxlength="254" value="${escapeHTML(state.authEmail)}" required></div>
        ${state.otpSent ? '<div class="field"><label for="code">6桁の確認コード</label><input id="code" name="code" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" required><small>メールに届いたコードを入力してください。</small></div>' : ""}
        <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "処理中…" : state.otpSent ? (isRegister ? "アカウントを作成" : "ログイン") : "確認コードを送る"}</button>
      </form>`}
      <div class="auth-switch">${isRegister ? "アカウントをお持ちですか？" : "初めて利用しますか？"} <button id="auth-switch" class="text-button">${isRegister ? "ログイン" : "新規登録"}</button></div>
    </div>
    ${toast()}
  </section>`;

  bind("auth-switch", "click", () => {
    state.authMode = isRegister ? "login" : "register";
    state.authMethod = isRegister ? "password" : "code";
    state.otpSent = false;
    state.authEmail = "";
    state.authName = "";
    state.errors = [];
    render();
  });
  bind("use-password", "click", () => { state.authMethod = "password"; state.otpSent = false; state.errors = []; render(); });
  bind("use-code", "click", () => { state.authMethod = "code"; state.otpSent = false; state.errors = []; render(); });
  bind("password-login-form", "submit", submitPasswordLogin);
  bind("otp-form", "submit", submitOTPForm);
  bind("passkey-login", "click", loginWithPasskey);
}

async function submitPasswordLogin(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const email = String(form.get("email") || "").trim();
  const password = String(form.get("password") || "");
  state.authEmail = email;
  setBusy(true);
  try {
    await api("/auth/password/login", { method: "POST", json: { email, password } });
    await loadAuthenticatedApp();
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
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
    actions: '<button id="settings" class="button secondary small">設定</button><button id="logout" class="button small">ログアウト</button>',
  })}
  ${errorBanner()}
  ${state.namespaces.length ? `<div class="namespace-list">${state.namespaces.map((namespace, index) => `<button class="list-row namespace-row" data-index="${index}"><strong>${escapeHTML(namespace.name || "Namespace")}</strong><span class="list-meta">${namespace.latestAt ? `最終更新: ${escapeHTML(formatAgo(namespace.latestAt))}` : "データなし"}</span><span class="chevron">›</span></button>`).join("")}</div>` : `<section class="empty-state"><div><h2>Namespace がありません</h2><p>最初のNamespaceとPersonal Organizationを作成して、Deviceを登録しましょう。</p><button id="create-namespace" class="button" ${state.busy ? "disabled" : ""}>${state.busy ? "作成中…" : "Namespaceを作成"}</button></div></section>`}
  ${toast()}`;
  document.querySelectorAll(".namespace-row").forEach(row => row.addEventListener("click", () => openNamespace(Number(row.dataset.index))));
  bind("settings", "click", openSettings);
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
  const canManage = namespace?.grant_type === "admin";
  app.innerHTML = `${header(`${namespace?.name || "Namespace"}⌄`, {
    back: true,
    actions: '<button id="logout" class="button small">ログアウト</button>',
  })}
  ${errorBanner()}
  <div class="toolbar"><span>${state.devices.length} Device / ${metrics.length} Metrics</span><div class="toolbar-actions">${canManage ? '<button id="members" class="button secondary small">ユーザー</button><button id="add-device" class="button secondary small">＋ Device</button>' : ""}</div></div>
  ${metrics.length ? `<div class="metrics">${metrics.map(metricCard).join("")}</div>` : `<section class="empty-state"><div><h2>計測データがありません</h2><p>DeviceからCayenne LPP uplinkを送信すると、ここに表示されます。</p>${canManage ? '<button id="empty-add-device" class="button">Deviceを追加</button>' : ""}</div></section>`}
  ${toast()}`;
  bind("back", "click", () => { state.view = "dashboard"; render(); });
  bind("logout", "click", logout);
  bind("members", "click", openNamespaceMembers);
  bind("add-device", "click", () => { state.view = "add-device"; render(); });
  bind("empty-add-device", "click", () => { state.view = "add-device"; render(); });
}

const grantLabels = {
  r: "閲覧のみ",
  w: "送信のみ",
  rw: "閲覧・送信",
  admin: "管理者",
};

async function openNamespaceMembers() {
  state.view = "loading";
  render();
  try {
    await refreshNamespaceMembers();
    state.errors = [];
    state.view = "namespace-members";
  } catch (error) {
    state.view = "namespace";
    addError(error);
  }
  render();
}

async function refreshNamespaceMembers() {
  const response = await api(`/cfg/${state.selectedNamespace.namespace_id}/members`);
  state.namespaceMembers = response.data || [];
}

function renderNamespaceMembers() {
  const currentUserID = state.me?.ID || state.me?.id;
  app.innerHTML = `${header("Namespace ユーザー", { back: true })}${errorBanner()}
  <div class="info-box">登録済みユーザーのメールアドレスを指定して招待します。同じユーザーを再送信すると権限を更新できます。</div>
  <section class="settings-section">
    <h2>招待・権限更新</h2>
    <form id="invite-form" class="form-grid">
      <div class="field"><label for="invite-email">メールアドレス</label><input id="invite-email" name="email" type="email" autocomplete="email" maxlength="254" required></div>
      <div class="field"><label for="grant-type">権限</label><select id="grant-type" name="grant_type" required><option value="r">閲覧のみ</option><option value="rw" selected>閲覧・送信</option><option value="w">送信のみ</option><option value="admin">管理者</option></select></div>
      <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "更新中…" : "招待する"}</button>
    </form>
  </section>
  <section class="settings-section">
    <h2>現在のユーザー</h2>
    <div class="member-list">${state.namespaceMembers.length ? state.namespaceMembers.map((member, index) => `<div class="member-row"><div class="member-main"><strong>${escapeHTML(member.name)}</strong><small>${escapeHTML(member.email)}</small></div><span class="grant-badge ${member.grant_type === "admin" ? "admin" : ""}">${escapeHTML(grantLabels[member.grant_type] || member.grant_type)}</span>${member.user_id === currentUserID ? '<span class="self-label">自分</span>' : `<button class="button danger small remove-member" data-index="${index}">削除</button>`}</div>`).join("") : '<div class="empty-state"><p>ユーザーがいません。</p></div>'}</div>
  </section>${toast()}`;
  bind("back", "click", () => { state.view = "namespace"; render(); });
  bind("invite-form", "submit", inviteNamespaceMember);
  document.querySelectorAll(".remove-member").forEach(button => button.addEventListener("click", () => removeNamespaceMember(Number(button.dataset.index))));
}

async function inviteNamespaceMember(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const email = String(form.get("email") || "").trim();
  const grantType = String(form.get("grant_type") || "rw");
  setBusy(true);
  try {
    await api(`/cfg/${state.selectedNamespace.namespace_id}/invite`, { method: "POST", json: { email, grant_type: grantType } });
    await refreshNamespaceMembers();
    state.errors = [];
    showToast("ユーザーの権限を更新しました");
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
}

async function removeNamespaceMember(index) {
  const member = state.namespaceMembers[index];
  if (!member || !window.confirm(`${member.email} をNamespaceから削除しますか？`)) return;
  setBusy(true);
  try {
    await api(`/cfg/${state.selectedNamespace.namespace_id}/disinvite`, { method: "POST", json: { email: member.email } });
    await refreshNamespaceMembers();
    state.errors = [];
    showToast("ユーザーを削除しました");
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
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

async function openSettings() {
  state.view = "loading";
  render();
  try {
    const [password, passkeys] = await Promise.all([api("/auth/password"), api("/auth/passkeys")]);
    state.passwordConfigured = Boolean(password.configured);
    state.passkeys = passkeys.data || [];
    state.errors = [];
    state.view = "settings";
  } catch (error) {
    state.view = "dashboard";
    addError(error);
  }
  render();
}

function renderSettings() {
  const supported = state.passkeyEnabled && window.PublicKeyCredential && navigator.credentials;
  const email = state.me?.Email || state.me?.email || "";
  app.innerHTML = `${header("アカウント設定", { back: true })}${errorBanner()}
  <section class="settings-section">
    <h2>パスワード</h2>
    <p class="section-description">${escapeHTML(email)}${state.passwordConfigured ? " のパスワードを変更します。" : " にログイン用パスワードを設定します。"}</p>
    <form id="password-form" class="form-grid">
      ${state.passwordConfigured ? '<div class="field"><label for="current-password">現在のパスワード</label><input id="current-password" name="current_password" type="password" autocomplete="current-password" maxlength="128" required></div>' : ""}
      <div class="field"><label for="new-password">新しいパスワード</label><input id="new-password" name="new_password" type="password" autocomplete="new-password" minlength="12" maxlength="128" required><small>12〜128文字。メールコードとパスキーは引き続き利用できます。</small></div>
      <div class="field"><label for="confirm-password">新しいパスワード（確認）</label><input id="confirm-password" name="confirm_password" type="password" autocomplete="new-password" minlength="12" maxlength="128" required></div>
      <button class="button full" type="submit" ${state.busy ? "disabled" : ""}>${state.busy ? "保存中…" : state.passwordConfigured ? "パスワードを変更" : "パスワードを設定"}</button>
    </form>
  </section>
  <section class="settings-section">
    <h2>パスキー</h2>
    <div class="info-box"><span class="status-dot"></span>端末の生体認証・PINを使います。秘密鍵がサーバーへ送信されることはありません。</div>
    <div class="passkey-list">${state.passkeys.length ? state.passkeys.map(item => `<div class="passkey-row"><div><strong>${escapeHTML(item.name)}</strong><small>登録: ${escapeHTML(formatDate(item.created_at))}${item.last_used_at ? ` / 最終利用: ${escapeHTML(formatAgo(item.last_used_at))}` : ""}</small></div><button class="button danger small delete-passkey" data-id="${escapeHTML(item.id)}">削除</button></div>`).join("") : '<div class="empty-state"><p>登録済みパスキーはありません。</p></div>'}</div>
    ${supported ? `<form id="passkey-form" class="form-grid"><div class="field"><label for="passkey-name">新しいパスキーの名前</label><input id="passkey-name" name="name" maxlength="100" value="この端末" required></div><button class="button full" ${state.busy ? "disabled" : ""}>${state.busy ? "登録中…" : "パスキーを追加"}</button></form>` : '<p class="unavailable">このブラウザまたはサーバー設定ではパスキーを追加できません。</p>'}
  </section>
  ${toast()}`;
  bind("back", "click", () => { state.view = "dashboard"; render(); });
  bind("password-form", "submit", updatePassword);
  bind("passkey-form", "submit", registerPasskey);
  document.querySelectorAll(".delete-passkey").forEach(button => button.addEventListener("click", () => deletePasskey(button.dataset.id)));
}

async function updatePassword(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const currentPassword = String(form.get("current_password") || "");
  const newPassword = String(form.get("new_password") || "");
  const confirmation = String(form.get("confirm_password") || "");
  if (newPassword !== confirmation) {
    addError(new Error("新しいパスワードが一致しません"));
    return;
  }
  setBusy(true);
  try {
    await api("/auth/password", { method: "PUT", json: { current_password: currentPassword, new_password: newPassword } });
    state.passwordConfigured = true;
    state.errors = [];
    showToast("パスワードを保存しました。他のログインセッションは失効しました。");
  } catch (error) {
    addError(error);
  } finally {
    state.busy = false;
    render();
  }
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
    state.view = "settings";
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
  state.namespaceMembers = [];
  state.passkeys = [];
  state.passwordConfigured = false;
  state.authMethod = "password";
  state.otpSent = false;
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
