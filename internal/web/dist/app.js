const $ = (s, r=document) => r.querySelector(s);
const $$ = (s, r=document) => [...r.querySelectorAll(s)];

function base() {
  let p = location.pathname;
  if (p.endsWith("index.html")) p = p.slice(0, -10);
  if (!p.endsWith("/")) p += "/";
  return p;
}

async function api(path, opts={}) {
  const url = path.startsWith("/") ? base() + path.replace(/^\//, "") : path;
  const headers = { ...(opts.headers||{}) };
  let body = opts.body;
  if (body && typeof body === "object" && !(body instanceof FormData)) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(body);
  }
  const res = await fetch(url, {
    credentials: "same-origin",
    headers,
    method: opts.method || "GET",
    body,
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = text; }
  if (!res.ok) {
    const msg = (data && data.error) || text || String(res.status);
    const err = new Error(msg);
    err.status = res.status;
    throw err;
  }
  return data;
}

function bytes(n) {
  n = Number(n||0);
  const u = ["B","KB","MB","GB","TB"];
  let i = 0;
  while (n >= 1024 && i < u.length-1) { n /= 1024; i++; }
  return n.toFixed(i?1:0) + " " + u[i];
}

function ymd(t) {
  if (!t) return "";
  return String(t).slice(0, 10);
}

function fmtExpire(t) {
  const d = ymd(t);
  return d || "—";
}

const MONTHS = ["January","February","March","April","May","June","July","August","September","October","November","December"];
const WEEKDAYS = ["Sat","Sun","Mon","Tue","Wed","Thu","Fri"];

function datePickerHTML(id, value) {
  const has = /^\d{4}-\d{2}-\d{2}$/.test(value || "");
  let y, m, d;
  if (has) {
    y = Number(value.slice(0, 4));
    m = Number(value.slice(5, 7)) - 1;
    d = Number(value.slice(8, 10));
  } else {
    const n = new Date();
    y = n.getUTCFullYear();
    m = n.getUTCMonth();
    d = n.getUTCDate();
  }
  return `<div class="datepicker" id="${id}" data-y="${y}" data-m="${m}" data-d="${d}">
    <label class="inline"><input type="checkbox" class="dp-none" ${has ? "" : "checked"}> No expiry</label>
    <div class="dp-body">
      <div class="dp-nav">
        <select class="dp-year"></select>
        <select class="dp-month"></select>
      </div>
      <div class="dp-week">${WEEKDAYS.map(w => `<span>${w}</span>`).join("")}</div>
      <div class="dp-grid"></div>
      <p class="muted dp-label"></p>
    </div>
  </div>`;
}

function datePickerValue(id) {
  const root = document.getElementById(id);
  if (!root || root.querySelector(".dp-none").checked) return "";
  const y = Number(root.dataset.y);
  const m = Number(root.dataset.m) + 1;
  const d = Number(root.dataset.d);
  if (!y || !d) return "";
  return y + "-" + String(m).padStart(2, "0") + "-" + String(d).padStart(2, "0");
}

function bindDatePicker(id) {
  const root = document.getElementById(id);
  if (!root) return;
  const none = root.querySelector(".dp-none");
  const body = root.querySelector(".dp-body");
  const year = root.querySelector(".dp-year");
  const month = root.querySelector(".dp-month");
  const grid = root.querySelector(".dp-grid");
  const label = root.querySelector(".dp-label");
  const nowY = new Date().getUTCFullYear();
  year.innerHTML = "";
  for (let y = nowY - 1; y <= nowY + 15; y++) {
    const opt = document.createElement("option");
    opt.value = String(y);
    opt.textContent = String(y);
    if (y === Number(root.dataset.y)) opt.selected = true;
    year.appendChild(opt);
  }
  month.innerHTML = MONTHS.map((name, i) => `<option value="${i}" ${i === Number(root.dataset.m) ? "selected" : ""}>${name}</option>`).join("");
  const paint = () => {
    const y = Number(year.value);
    const m = Number(month.value);
    root.dataset.y = String(y);
    root.dataset.m = String(m);
    let d = Number(root.dataset.d) || 1;
    const dim = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
    if (d > dim) d = dim;
    root.dataset.d = String(d);
    const first = new Date(Date.UTC(y, m, 1));
    const pad = (first.getUTCDay() + 1) % 7;
    let html = "";
    for (let i = 0; i < pad; i++) html += `<span></span>`;
    for (let day = 1; day <= dim; day++) {
      html += `<button type="button" class="dp-day${day === d ? " on" : ""}" data-day="${day}">${day}</button>`;
    }
    grid.innerHTML = html;
    label.textContent = none.checked ? "No expiry" : (MONTHS[m] + " " + d + ", " + y + " (end of UTC day)");
    body.classList.toggle("dim", none.checked);
    $$(".dp-day", grid).forEach(btn => {
      btn.onclick = () => {
        none.checked = false;
        root.dataset.d = btn.dataset.day;
        paint();
      };
    });
  };
  none.onchange = paint;
  year.onchange = paint;
  month.onchange = paint;
  paint();
}

function trafficParts(limit) {
  const n = Number(limit||0);
  if (!n) return { value: "", unit: "unlimited" };
  const TB = 1024**4, GB = 1024**3, MB = 1024**2;
  if (n % TB === 0 || n >= TB) return { value: +(n/TB).toFixed(2), unit: "TB" };
  if (n % GB === 0 || n >= GB) return { value: +(n/GB).toFixed(2), unit: "GB" };
  return { value: +(n/MB).toFixed(2), unit: "MB" };
}

function trafficLimitFromForm(prefix) {
  const unit = $("#"+prefix+"-unit").value;
  if (unit === "unlimited") return 0;
  const n = Number($("#"+prefix).value||0);
  if (!(n > 0)) return 0;
  const mul = { MB: 1024**2, GB: 1024**3, TB: 1024**4 };
  return Math.round(n * (mul[unit] || mul.GB));
}

function trafficLimitFields(id, limit) {
  const p = trafficParts(limit);
  return `<div class="row" style="margin:0">
    <input id="${id}" type="number" min="0" step="0.1" value="${p.unit==="unlimited"?"":p.value}" ${p.unit==="unlimited"?"disabled":""}>
    <select id="${id}-unit">
      <option value="GB" ${p.unit==="GB"?"selected":""}>GB</option>
      <option value="MB" ${p.unit==="MB"?"selected":""}>MB</option>
      <option value="TB" ${p.unit==="TB"?"selected":""}>TB</option>
      <option value="unlimited" ${p.unit==="unlimited"?"selected":""}>unlimited</option>
    </select>
  </div>`;
}

function bindTrafficUnit(id) {
  const sel = $("#"+id+"-unit");
  const inp = $("#"+id);
  if (!sel || !inp) return;
  sel.onchange = () => { inp.disabled = sel.value === "unlimited"; if (sel.value==="unlimited") inp.value = ""; };
}

function userStatus(u) {
  const used = Number(u.traffic_up||0)+Number(u.traffic_down||0);
  if (u.traffic_limit && used >= u.traffic_limit) return { t: "quota", cls: "bad" };
  if (u.expire_at && Date.parse(u.expire_at) < Date.now()) return { t: "expired", cls: "bad" };
  if (!u.enable) return { t: "off", cls: "bad" };
  return { t: "on", cls: "ok" };
}

function show(view) {
  $$(".view").forEach(v => v.hidden = true);
  $$("nav button").forEach(b => b.classList.toggle("active", b.dataset.view === view));
  $("#title").textContent = ({dash:"Dashboard", users:"Users", inbounds:"Inbounds", domains:"Domains", settings:"Settings"})[view];
  $("#view-"+view).hidden = false;
}

function toast(err) { alert(err.message || err); }

function openModal(title, html, onOk) {
  $("#modal-title").textContent = title;
  $("#modal-body").innerHTML = html;
  $("#modal").hidden = false;
  $("#modal-ok").onclick = async () => { await onOk(); $("#modal").hidden = true; };
  $("#modal-cancel").onclick = () => $("#modal").hidden = true;
}

async function refreshCore() {
  try {
    const d = await api("/api/dashboard");
    const pill = $("#core-pill");
    pill.textContent = d.core_running ? "core up" : "core down";
    pill.className = "pill " + (d.core_running ? "ok" : "bad");
    return d;
  } catch { return null; }
}

async function renderDash() {
  const d = await api("/api/dashboard");
  $("#view-dash").innerHTML = `
    <div class="grid">
      <div class="card"><div class="muted">Users</div><div class="stat">${d.users_active}/${d.users_total}</div></div>
      <div class="card"><div class="muted">Traffic up</div><div class="stat">${bytes(d.traffic_up)}</div></div>
      <div class="card"><div class="muted">Traffic down</div><div class="stat">${bytes(d.traffic_down)}</div></div>
      <div class="card"><div class="muted">Inbounds on</div><div class="stat">${d.inbounds_on}</div></div>
    </div>
    <div class="card" style="margin-top:16px">
      <p>Public host: <b>${d.public_host || "(set in Settings)"}</b></p>
      <p class="muted">HTTPS: if public host is a real domain pointing at this VPS, Let's Encrypt is requested automatically (port 80). Until then — and for <code>https://IP/</code> — a self-signed cert is used. Cloudflare SSL mode <b>Full</b> works with self-signed; <b>Full (strict)</b> needs Let's Encrypt.</p>
      <p class="muted">SQLite + certs live in <code>${d.data_dir}</code>. Mount that folder to keep users when the container is replaced.</p>
      <p class="muted">${d.core_error || ""}</p>
      <p>Admin path (bookmark this): <code>${d.admin_url}</code></p>
      <p class="muted">User subscription base: <code>${d.client_path}/&lt;token&gt;</code> — opening that in a browser shows configs and usage.</p>
      ${d.traffic_error ? `<p class="error">Traffic counter: ${d.traffic_error}</p>` : ""}
    </div>
    <div class="card" style="margin-top:16px">
      <p><b>Certificates</b></p>
      <p class="muted">Let's Encrypt HTTP-01 on port 80 for public host and domains. Renews automatically ~30 days before expiry. The issued cert is used for the panel and for direct TLS inbounds (Hysteria2 / TUIC / Trojan).</p>
      <table><thead><tr><th>Name</th><th>Status</th><th>Issuer</th><th>Expires</th></tr></thead>
      <tbody>${(d.certs||[]).map(c => `
        <tr>
          <td><code>${c.domain}</code></td>
          <td><span class="pill ${c.state==="issued"?"ok":(c.state==="failed"?"bad":"")}">${c.state}</span></td>
          <td>${c.issuer||"—"}</td>
          <td>${c.not_after? c.not_after+" ("+c.days_left+"d)":"—"}${c.error? `<div class="error">${c.error}</div>`:""}</td>
        </tr>`).join("") || `<tr><td colspan="4" class="muted">No hostnames yet</td></tr>`}
      </tbody></table>
      <div class="row"><button class="primary" id="issue-certs">Issue / renew now</button></div>
    </div>`;
  $("#issue-certs") && ($("#issue-certs").onclick = async () => {
    try {
      const r = await api("/api/certs/issue", { method:"POST", body:{} });
      if (r && r.error) toast(r.error);
      await renderDash();
    } catch (e) { toast(e); }
  });
  await refreshCore();
}

async function renderUsers() {
  const users = await api("/api/users");
  $("#view-users").innerHTML = `
    <div class="row"><button class="primary" id="add-user">Add user</button></div>
    <div class="card" style="margin-top:12px;overflow:auto">
      <table><thead><tr><th>User</th><th>Status</th><th>Expire</th><th>Traffic</th><th>Sub</th><th></th></tr></thead>
      <tbody>${users.map(u => {
        const st = userStatus(u);
        return `
        <tr>
          <td><b>${u.username}</b><div class="muted">${u.note||""}</div></td>
          <td><span class="pill ${st.cls}">${st.t}</span></td>
          <td>${fmtExpire(u.expire_at)}</td>
          <td>${bytes(u.traffic_up+u.traffic_down)}${u.traffic_limit? " / "+bytes(u.traffic_limit):" / unlimited"}</td>
          <td><code>${u.sub_token}</code></td>
          <td>
            <button data-links="${u.id}">Links</button>
            <button data-edit="${u.id}">Edit</button>
            <button data-del="${u.id}">Delete</button>
          </td>
        </tr>`;
      }).join("")}
      </tbody></table>
    </div>`;
  $("#add-user").onclick = () => {
    openModal("New user", `
      <label>Username <input id="f-user" required></label>
      <label>Note <input id="f-note"></label>
      <label>Traffic limit ${trafficLimitFields("f-lim", 0)}</label>
      <label>Expire</label>
      ${datePickerHTML("f-exp", "")}
      <p class="muted">Unlimited traffic = no cap. Expiry is end of that UTC day.</p>`, async () => {
        await api("/api/users", { method:"POST", body:{
          username: $("#f-user").value,
          note: $("#f-note").value,
          traffic_limit: trafficLimitFromForm("f-lim"),
          expire_at: datePickerValue("f-exp"),
        }});
        await renderUsers();
      });
    bindTrafficUnit("f-lim");
    bindDatePicker("f-exp");
  };
  $$("[data-del]").forEach(b => b.onclick = async () => {
    if (!confirm("Delete user?")) return;
    await api("/api/users/"+b.dataset.del, { method:"DELETE" });
    await renderUsers();
  });
  $$("[data-edit]").forEach(b => b.onclick = async () => {
    const u = users.find(x => String(x.id)===b.dataset.edit);
    const st = userStatus(u);
    openModal("Edit "+u.username, `
      <label>Username <input id="f-user" value="${u.username}"></label>
      <label>Note <input id="f-note" value="${u.note||""}"></label>
      <label>Enabled <select id="f-en"><option value="1" ${u.enable||st.t==="quota"?'selected':''}>on</option><option value="0" ${!u.enable && st.t!=="quota"?'selected':''}>off</option></select></label>
      <label>Traffic limit ${trafficLimitFields("f-lim", u.traffic_limit)}</label>
      <label>Expire</label>
      ${datePickerHTML("f-exp", ymd(u.expire_at))}
      ${u.telegram_secret ? `<p class="muted">Telegram secret (this user only; traffic counts toward their quota)</p><code>${u.telegram_secret}</code>
      <div class="row" style="margin-top:8px"><button type="button" id="f-tg-regen">Regenerate Telegram secret</button></div>` : ""}
      <p class="muted">Raising the traffic limit above usage turns a quota-blocked user back on.</p>`, async () => {
        await api("/api/users/"+u.id, { method:"PUT", body:{
          username: $("#f-user").value, note: $("#f-note").value,
          enable: $("#f-en").value==="1", traffic_limit: trafficLimitFromForm("f-lim"),
          expire_at: datePickerValue("f-exp"),
        }});
        await renderUsers();
      });
    bindTrafficUnit("f-lim");
    bindDatePicker("f-exp");
    $("#f-tg-regen") && ($("#f-tg-regen").onclick = async () => {
      if (!confirm("New Telegram secret for this user. Their old tg:// link stops working.")) return;
      try {
        await api("/api/users/"+u.id, { method:"PUT", body:{ telegram_regenerate: true }});
        await renderUsers();
        $("#modal").hidden = true;
      } catch (e) { toast(e); }
    });
  });
  $$("[data-links]").forEach(b => b.onclick = async () => {
    const data = await api("/api/users/"+b.dataset.links+"/links");
    openModal("Subscription", `
      <p>Import this URL in Hiddify / v2rayNG / Clash Meta / sing-box:</p>
      <div class="links">
        <code>${location.origin}${data.sub}</code>
        <p class="muted">Open the first URL in a browser for the user page (configs + usage). Apps import it as a subscription.</p>
        <code>${location.origin}${data.clash}</code>
        <code>${location.origin}${data.sing_box}</code>
      </div>
      <p class="muted">Share links</p>
      <div class="links">${data.links.map(l => `<div><span class="pill ${l.mode}">${l.mode}</span> ${l.tag}<code>${l.uri}</code></div>`).join("")}</div>
    `, async () => {});
  });
}

async function renderInbounds() {
  const rows = await api("/api/inbounds");
  $("#view-inbounds").innerHTML = `
    <p class="muted">Direct: REALITY / Hysteria2 / TUIC / WireGuard / raw TLS, plus WS / gRPC / HTTP/2 / HTTPUpgrade / xHTTP on the same 443 path mux as CDN (own domain). CDN: those HTTP transports on a Cloudflare / Arvan / Gcore hostname. Toggle each inbound independently.</p>
    <div class="card" style="overflow:auto"><table>
      <thead><tr><th>On</th><th>Tag</th><th>Mode</th><th>Listen</th><th>Path</th><th>Remark</th></tr></thead>
      <tbody>${rows.map(r => `
        <tr>
          <td><input class="toggle" type="checkbox" data-id="${r.id}" ${r.enable?"checked":""}></td>
          <td><code>${r.tag}</code><div class="muted">${r.protocol}/${r.transport}/${r.security}</div></td>
          <td><span class="pill ${r.mode}">${r.mode}</span></td>
          <td>${r.listen_port || r.internal_port}</td>
          <td><code>${r.path||""}</code></td>
          <td>${r.remark}</td>
        </tr>`).join("")}
      </tbody></table></div>`;
  $$("input.toggle", $("#view-inbounds")).forEach(el => el.onchange = async () => {
    await api("/api/inbounds/"+el.dataset.id, { method:"PUT", body:{ enable: el.checked } });
  });
}

async function renderDomains() {
  const rows = await api("/api/domains");
  $("#view-domains").innerHTML = `
    <p class="muted"><b>direct</b> domains are used in Reality/Hysteria/TUIC links (A record to this machine). <b>cdn</b> domains are used in WS/gRPC links (orange-cloud / Arvan / Gcore in front of origin <b>443</b> or 80).</p>
    <div class="row"><button class="primary" id="add-dom">Add domain</button></div>
    <div class="card" style="margin-top:12px;overflow:auto"><table>
      <thead><tr><th>Domain</th><th>Mode</th><th>CDN</th><th></th></tr></thead>
      <tbody>${rows.map(d => `
        <tr>
          <td>${d.domain}</td>
          <td><span class="pill ${d.mode}">${d.mode}</span></td>
          <td>${d.provider}</td>
          <td><button data-del="${d.id}">Delete</button></td>
        </tr>`).join("") || `<tr><td colspan="4" class="muted">No domains yet — Settings → public host is used instead.</td></tr>`}
      </tbody></table></div>`;
  $("#add-dom").onclick = () => openModal("Add domain", `
    <label>Domain <input id="f-d" placeholder="cdn.example.com"></label>
    <label>Mode <select id="f-m"><option value="direct">direct</option><option value="cdn">cdn</option><option value="camouflage">camouflage</option></select></label>
    <label>Provider <select id="f-p"><option value="none">none</option><option value="cloudflare">cloudflare</option><option value="arvan">arvan</option><option value="gcore">gcore</option></select></label>`, async () => {
      await api("/api/domains", { method:"POST", body:{ domain:$("#f-d").value, mode:$("#f-m").value, provider:$("#f-p").value }});
      await renderDomains();
    });
  $$("[data-del]", $("#view-domains")).forEach(b => b.onclick = async () => {
    await api("/api/domains/"+b.dataset.del, { method:"DELETE" });
    await renderDomains();
  });
}

async function renderSettings() {
  const [s, me] = await Promise.all([api("/api/settings"), api("/api/me")]);
  $("#view-settings").innerHTML = `
    <div class="card">
      <label>Public host / IP used in share links <input id="s-host" value="${s.public_host||""}"></label>
      <label>Let's Encrypt email (empty is allowed, but recommended) <input id="s-acme" type="email" placeholder="you@example.com" value="${s.acme_email||""}"></label>
      <label>REALITY handshake dest (looks like this site) <input id="s-real" value="${s.reality_server_name||""}"></label>
      <label>Hysteria2 obfs password (empty = off) <input id="s-hy" value="${s.hy2_obfs||""}"></label>
      <label>Admin secret path <input id="s-admin" value="${s.admin_path||""}" autocomplete="off"></label>
      <p class="muted">Changing the admin path logs you into the new URL. Bookmark it. Client path stays <code>${s.client_path||""}</code>.</p>
      <p class="muted">Panel HTTPS is on the same TCP 443 as REALITY and Telegram (SNI mux). Your domain / an IP / empty SNI → panel. The Telegram fake domain → MTProto. Other SNI (e.g. the handshake dest) → REALITY. Let's Encrypt uses HTTP-01 on port 80 for configured hostnames; IPs stay self-signed.</p>
      <p class="muted">REALITY public key: <code>${s.reality_public_key||""}</code></p>
      <p class="muted">WireGuard server public key: <code>${s.wg_public_key||""}</code></p>
      <div class="row"><button class="primary" id="save-s">Save & reload core</button></div>
    </div>
    <div class="card" style="margin-top:16px">
      <p><b>Telegram proxy</b></p>
      <p class="muted">Same FakeTLS MTProto on 443 as Hiddify, but <b>each user has their own secret</b>. Bytes on that secret are added to the user's traffic (same quota as VPN). Disabled, expired, or over-quota users are dropped from the proxy. Do <b>not</b> use your panel domain or the REALITY handshake dest as the fake website.</p>
      <label class="inline"><input type="checkbox" id="s-tg" ${s.telegram_enabled?"checked":""}> Enable Telegram MTProto</label>
      <label>Fake TLS domain <input id="s-tg-dom" value="${s.telegram_fake_domain||"www.cloudflare.com"}" placeholder="www.cloudflare.com"></label>
      <p class="muted">Share links are on each user (Links / the subscription page), not here. Changing the fake domain or regenerating issues new secrets for every user.</p>
      <div class="row"><button type="button" id="s-tg-regen">Regenerate all user secrets</button></div>
    </div>
    <div class="card" style="margin-top:16px">
      <p><b>Admin account</b></p>
      <p class="muted">Env <code>SOOOSKI_ADMIN_USER</code> / <code>SOOOSKI_ADMIN_PASSWORD</code> apply only on first boot. After that, change them here.</p>
      <label>Username <input id="a-user" value="${me.username||""}" autocomplete="username"></label>
      <label>Current password <input id="a-cur" type="password" autocomplete="current-password"></label>
      <label>New password (leave blank to keep) <input id="a-new" type="password" autocomplete="new-password"></label>
      <label>Confirm new password <input id="a-new2" type="password" autocomplete="new-password"></label>
      <div class="row"><button class="primary" id="save-admin">Save username / password</button></div>
    </div>`;
  const saveSettings = async (extra={}) => {
    const prev = s.admin_path||"";
    const data = await api("/api/settings", { method:"PUT", body:{
      public_host: $("#s-host").value,
      acme_email: $("#s-acme").value,
      reality_server_name: $("#s-real").value,
      hy2_obfs: $("#s-hy").value,
      admin_path: $("#s-admin").value,
      telegram_enabled: $("#s-tg").checked ? "1" : "0",
      telegram_fake_domain: $("#s-tg-dom").value,
      ...extra,
    }});
    const next = (data && data.admin_path) || prev;
    if (next && next !== prev) {
      alert("Admin path changed. Bookmark the new URL.");
      location.href = location.origin + "/" + String(next).replace(/^\/+|\/+$/g,"") + "/";
      return;
    }
    await renderSettings();
  };
  $("#save-s").onclick = async () => {
    try { await saveSettings(); } catch (e) { toast(e); }
  };
  $("#s-tg-regen") && ($("#s-tg-regen").onclick = async () => {
    if (!confirm("This issues a new Telegram secret for every user. Existing tg:// links stop working.")) return;
    try { await saveSettings({ telegram_regenerate: "1" }); } catch (e) { toast(e); }
  });
  $("#save-admin").onclick = async () => {
    const pw = $("#a-new").value;
    const pw2 = $("#a-new2").value;
    if (pw && pw !== pw2) { toast("Passwords do not match"); return; }
    try {
      await api("/api/admin", { method:"PUT", body:{
        current_password: $("#a-cur").value,
        username: $("#a-user").value,
        password: pw,
        password_confirm: pw2,
      }});
      $("#a-cur").value = "";
      $("#a-new").value = "";
      $("#a-new2").value = "";
      alert("Admin account updated");
      await renderSettings();
    } catch (e) { toast(e); }
  };
}

const renderers = { dash: renderDash, users: renderUsers, inbounds: renderInbounds, domains: renderDomains, settings: renderSettings };

$$("nav button").forEach(b => b.onclick = async () => {
  show(b.dataset.view);
  try { await renderers[b.dataset.view](); } catch (e) { toast(e); }
});
$("#logout").onclick = async () => { await api("/api/logout", { method:"POST" }); location.reload(); };

$("#login-form").onsubmit = async (e) => {
  e.preventDefault();
  $("#login-err").hidden = true;
  const fd = new FormData(e.target);
  try {
    await api("/api/login", { method:"POST", body:{ username: fd.get("username"), password: fd.get("password") }});
    $("#login").hidden = true;
    $("#app").hidden = false;
    show("dash");
    await renderDash();
  } catch (err) {
    $("#login-err").hidden = false;
    $("#login-err").textContent = err.message;
  }
};

(async () => {
  try {
    await api("/api/me");
    $("#login").hidden = true;
    $("#app").hidden = false;
    show("dash");
    await renderDash();
  } catch {}
})();
