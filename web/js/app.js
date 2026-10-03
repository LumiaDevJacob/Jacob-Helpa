/* Jacob Helpa — front end.
   Talks to the local Go server over fetch; no external requests anywhere. */

(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const on = (el, ev, fn) => el && el.addEventListener(ev, fn);
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  let config = null;
  let vantaScene = null;

  /* ---------------------------------------------------------------- api */

  async function api(path, body) {
    const res = await fetch(path, {
      method: body === undefined ? "GET" : "POST",
      headers: body === undefined ? {} : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    let data = {};
    try {
      data = await res.json();
    } catch {
      /* some responses have no body */
    }
    if (!res.ok) throw new Error(data.error || `Something went wrong (${res.status})`);
    return data;
  }

  /* -------------------------------------------------------------- toast */

  const ICONS = { ok: "✓", bad: "!", info: "i" };

  function toast(message, kind = "ok") {
    const el = document.createElement("div");
    el.className = "glass-toast" + (kind === "bad" ? " glass-toast--error" : kind === "ok" ? " glass-toast--success" : "");
    el.innerHTML = `<span class="glass-toast__icon">${ICONS[kind] || ICONS.info}</span><span class="glass-toast__text"></span>`;
    el.querySelector(".glass-toast__text").textContent = message;
    $("toasts").appendChild(el);
    setTimeout(() => {
      el.classList.add("is-going");
      setTimeout(() => el.remove(), 320);
    }, 2600);
  }

  async function copy(text) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Clipboard API can be blocked; fall back to a hidden selection.
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      let ok = false;
      try {
        ok = document.execCommand("copy");
      } catch {
        ok = false;
      }
      ta.remove();
      return ok;
    }
  }

  /* ------------------------------------------------------------- vanta */

  const VANTA_EFFECTS = { net: "NET", halo: "HALO", waves: "WAVES", globe: "GLOBE" };

  function startVanta(name, accent) {
    stopVanta();
    if (name === "off" || reduceMotion) return;
    const key = VANTA_EFFECTS[name];
    if (!key || !window.VANTA || !window.VANTA[key]) return;

    const colour = parseInt(accent.slice(1), 16);
    const shared = {
      el: "#welcomeBg",
      mouseControls: true,
      touchControls: true,
      gyroControls: false,
      minHeight: 200,
      minWidth: 200,
      scale: 1,
      scaleMobile: 1,
    };
    const perEffect = {
      NET: { color: colour, backgroundColor: 0x070a12, points: 11, maxDistance: 23, spacing: 16, showDots: true },
      HALO: { baseColor: colour, backgroundColor: 0x070a12, amplitudeFactor: 1.3, size: 1.4 },
      WAVES: { color: colour, shininess: 42, waveHeight: 17, waveSpeed: 0.85, zoom: 0.92 },
      GLOBE: { color: colour, color2: 0xffffff, backgroundColor: 0x070a12, size: 1.1 },
    };
    try {
      vantaScene = window.VANTA[key]({ ...shared, ...perEffect[key] });
    } catch (err) {
      console.warn("background effect unavailable", err);
    }
  }

  function stopVanta() {
    if (vantaScene && typeof vantaScene.destroy === "function") {
      try {
        vantaScene.destroy();
      } catch {
        /* already gone */
      }
    }
    vantaScene = null;
  }

  /* ------------------------------------------------------- welcome text */

  function layOutWordmark(text) {
    const host = $("brandWord");
    host.textContent = "";
    [...text].forEach((ch, i) => {
      const span = document.createElement("span");
      span.className = "ltr" + (ch === " " ? " ltr--space" : "");
      span.textContent = ch === " " ? " " : ch;
      span.style.setProperty("--i", i);
      host.appendChild(span);
    });
  }

  function typeOut(el, text, speed = 42) {
    return new Promise((resolve) => {
      if (reduceMotion) {
        el.textContent = text;
        resolve();
        return;
      }
      let i = 0;
      const tick = () => {
        el.textContent = text.slice(0, ++i);
        if (i < text.length) {
          setTimeout(tick, speed);
        } else {
          resolve();
        }
      };
      setTimeout(tick, 60);
    });
  }

  /* --------------------------------------------------------------- boot */

  function runBootSequence() {
    return new Promise((resolve) => {
      const fill = $("bootFill");
      const label = $("bootLabel");
      const pct = $("bootPct");
      const log = $("bootLog");
      const stream = new EventSource("/api/boot");
      let seen = 0;
      let settled = false;
      let safety = 0;

      // The safety timer below can fire long after the welcome screen has been
      // taken out of the page, so finish() runs at most once and never assumes
      // its elements are still there.
      const finish = () => {
        if (settled) return;
        settled = true;
        clearTimeout(safety);
        stream.close();
        const boot = $("boot");
        if (boot) boot.classList.add("is-done");
        if (fill) fill.style.width = "100%";
        if (pct) pct.textContent = "100%";
        resolve();
      };

      stream.onmessage = (ev) => {
        if (settled) return;
        let step;
        try {
          step = JSON.parse(ev.data);
        } catch {
          return;
        }
        seen += 1;
        const total = step.total || 7;
        const percent = Math.min(100, Math.round((step.index || seen) / total * 100));
        fill.style.width = percent + "%";
        pct.textContent = percent + "%";
        label.textContent = step.label;

        const li = document.createElement("li");
        const tick = document.createElement("span");
        tick.className = "tick" + (step.ok ? "" : " tick--bad");
        tick.textContent = step.ok ? "✓" : "✕";
        const name = document.createElement("span");
        name.textContent = step.label;
        const detail = document.createElement("span");
        detail.className = "detail";
        detail.textContent = step.detail || "";
        li.append(tick, name, detail);
        log.appendChild(li);
        log.scrollTop = log.scrollHeight;

        if (step.done) finish();
      };

      // If the stream drops we still want the app, so never hang here.
      stream.onerror = () => finish();
      safety = setTimeout(finish, 9000);
    });
  }

  function enterApp() {
    const welcome = $("welcome");
    if (welcome.classList.contains("is-leaving")) return;
    welcome.classList.add("is-leaving");
    const app = $("app");
    app.setAttribute("aria-hidden", "false");
    requestAnimationFrame(() => app.classList.add("is-in"));
    setTimeout(() => {
      welcome.remove();
      stopVanta();
    }, 900);
  }

  /* ----------------------------------------------------------- routing */

  const VIEW_INFO = {
    home: ["Home", "Everything in one place"],
    passwords: ["Passwords", "Random, strong, generated on your machine"],
    usernames: ["Usernames", "Four styles to choose from"],
    tokens: ["Keys & UUIDs", "Random values for your projects"],
    hash: ["Hashes", "Digest any text"],
    text: ["Text tools", "Clean up and convert text"],
    vault: ["Secure vault", "Notes encrypted with AES-256-GCM"],
    system: ["System", "What this machine looks like"],
    settings: ["Settings", "Make it yours"],
  };

  function go(view) {
    document.querySelectorAll(".navitem").forEach((b) => {
      b.classList.toggle("is-active", b.dataset.view === view);
    });
    document.querySelectorAll(".view").forEach((section) => {
      section.classList.toggle("is-active", section.dataset.view === view);
    });
    const [title, sub] = VIEW_INFO[view] || ["", ""];
    $("viewTitle").textContent = title;
    $("viewSub").textContent = sub;
    $("views").scrollTop = 0;
    if (view === "system") loadSysinfo();
    if (view === "vault") refreshVaultState();
  }

  /* ------------------------------------------------------- output lists */

  // Renders a list of values, each row copying itself when clicked.
  function renderValues(listEl, values, labelFor) {
    listEl.textContent = "";
    values.forEach((value, i) => {
      const li = document.createElement("li");
      li.className = "glass-list__item glass-list__item--interactive outrow";
      const content = document.createElement("div");
      content.className = "glass-list__content";
      const title = document.createElement("div");
      title.className = "glass-list__title";
      title.textContent = value;
      content.appendChild(title);
      if (labelFor) {
        const sub = document.createElement("div");
        sub.className = "glass-list__subtitle";
        sub.textContent = labelFor(i, value);
        content.appendChild(sub);
      }
      const trailing = document.createElement("span");
      trailing.className = "glass-list__value";
      trailing.textContent = "copy";
      li.append(content, trailing);
      li.addEventListener("click", async () => {
        if (await copy(value)) {
          trailing.textContent = "copied";
          li.classList.add("is-copied");
          setTimeout(() => {
            trailing.textContent = "copy";
            li.classList.remove("is-copied");
          }, 1300);
        } else {
          toast("Could not reach the clipboard", "bad");
        }
      });
      listEl.appendChild(li);
    });
  }

  function wireCopyAll() {
    document.querySelectorAll("[data-copyall]").forEach((btn) => {
      on(btn, "click", async () => {
        const list = $(btn.dataset.copyall);
        const values = [...list.querySelectorAll(".glass-list__title")].map((n) => n.textContent);
        if (!values.length) return;
        toast((await copy(values.join("\n"))) ? `Copied ${values.length}` : "Could not reach the clipboard",
          values.length ? "ok" : "bad");
      });
    });
  }

  /* ------------------------------------------------------------ actions */

  async function guard(fn) {
    try {
      await fn();
    } catch (err) {
      toast(err.message, "bad");
    }
  }

  function bindRange(input, output) {
    const sync = () => (output.textContent = input.value);
    on(input, "input", sync);
    sync();
  }

  function setupPasswords() {
    bindRange($("pwLen"), $("pwLenOut"));
    bindRange($("pwCount"), $("pwCountOut"));

    const generate = () => guard(async () => {
      const data = await api("/api/password", {
        length: Number($("pwLen").value),
        count: Number($("pwCount").value),
        lower: $("pwLower").checked,
        upper: $("pwUpper").checked,
        digits: $("pwDigits").checked,
        symbols: $("pwSymbols").checked,
        noAmbiguous: $("pwNoAmb").checked,
      });
      renderValues($("pwOut"), data.passwords);
      $("pwCard").hidden = false;
      $("pwBits").textContent = `${data.bits} bits · ${data.strength}`;
    });

    on($("pwGo"), "click", generate);
  }

  function setupUsernames() {
    let style = "clean";
    $("unStyle").querySelectorAll(".glass-segmented__item").forEach((btn) => {
      on(btn, "click", () => {
        style = btn.dataset.style;
        $("unStyle").querySelectorAll(".glass-segmented__item").forEach((b) => {
          b.classList.toggle("is-on", b === btn);
          b.setAttribute("aria-pressed", String(b === btn));
        });
      });
    });

    on($("unGo"), "click", () => guard(async () => {
      const data = await api("/api/username", { base: $("unBase").value, style, count: 10 });
      renderValues($("unOut"), data.usernames);
      $("unCard").hidden = false;
    }));
  }

  function setupTokens() {
    bindRange($("tkBytes"), $("tkBytesOut"));
    bindRange($("tkCount"), $("tkCountOut"));
    let latest = null;
    let kind = "hex";

    const show = () => {
      if (!latest) return;
      renderValues($("tkOut"), latest[kind] || []);
    };

    $("tkTabs").querySelectorAll(".glass-segmented__item").forEach((btn) => {
      on(btn, "click", () => {
        kind = btn.dataset.kind;
        $("tkTabs").querySelectorAll(".glass-segmented__item").forEach((b) => {
          b.classList.toggle("is-on", b === btn);
          b.setAttribute("aria-pressed", String(b === btn));
        });
        show();
      });
    });

    on($("tkGo"), "click", () => guard(async () => {
      latest = await api("/api/tokens", {
        bytes: Number($("tkBytes").value),
        count: Number($("tkCount").value),
      });
      $("tkCard").hidden = false;
      show();
    }));
  }

  const HASH_NAMES = [["md5", "MD5"], ["sha1", "SHA-1"], ["sha256", "SHA-256"], ["sha512", "SHA-512"]];

  function setupHash() {
    on($("hsGo"), "click", () => guard(async () => {
      const data = await api("/api/hash", { text: $("hsIn").value });
      const list = $("hsOut");
      list.textContent = "";
      HASH_NAMES.forEach(([key, name]) => {
        const li = document.createElement("li");
        li.className = "glass-list__item glass-list__item--interactive outrow";
        const content = document.createElement("div");
        content.className = "glass-list__content";
        const sub = document.createElement("div");
        sub.className = "glass-list__subtitle";
        sub.textContent = name;
        const title = document.createElement("div");
        title.className = "glass-list__title";
        title.textContent = data[key];
        content.append(sub, title);
        const trailing = document.createElement("span");
        trailing.className = "glass-list__value";
        trailing.textContent = "copy";
        li.append(content, trailing);
        li.addEventListener("click", async () => {
          if (await copy(data[key])) {
            trailing.textContent = "copied";
            setTimeout(() => (trailing.textContent = "copy"), 1300);
          }
        });
        list.appendChild(li);
      });
      $("hsCard").hidden = false;
      $("hsBytes").textContent = `${data.bytes} bytes in`;
    }));
  }

  function setupText() {
    $("txOps").querySelectorAll("[data-op]").forEach((btn) => {
      on(btn, "click", () => guard(async () => {
        const data = await api("/api/text", { text: $("txIn").value, op: btn.dataset.op });
        $("txOut").textContent = data.result;
        $("txCard").hidden = false;
        const s = data.stats;
        $("txStats").textContent = `${s.characters} characters · ${s.words} words · ${s.lines} lines · ${s.bytes} bytes`;
      }));
    });

    on($("txCopy"), "click", async () => {
      const text = $("txOut").textContent;
      if (!text) return;
      toast((await copy(text)) ? "Copied" : "Could not reach the clipboard", text ? "ok" : "bad");
    });
  }

  /* ------------------------------------------------------------- vault */

  let editingNote = null;

  async function refreshVaultState() {
    const state = await api("/api/vault/status");
    $("vaultLock").hidden = state.unlocked;
    $("vaultOpen").hidden = !state.unlocked;
    $("vaultLockTitle").textContent = state.exists ? "Unlock the vault" : "Create your vault";
    $("vaultUnlock").textContent = state.exists ? "Unlock" : "Create";
    if (state.unlocked) await loadNotes();
  }

  async function loadNotes() {
    const data = await api("/api/vault/list");
    const list = $("noteList");
    list.textContent = "";

    if (!data.notes || !data.notes.length) {
      const li = document.createElement("li");
      li.className = "glass-list__item glass-list__item--center";
      li.textContent = "Nothing saved yet.";
      list.appendChild(li);
      return;
    }

    data.notes.forEach((note) => {
      const li = document.createElement("li");
      li.className = "glass-list__item";
      const content = document.createElement("div");
      content.className = "glass-list__content";
      const title = document.createElement("div");
      title.className = "glass-list__title";
      title.textContent = note.title;
      const when = document.createElement("div");
      when.className = "glass-list__subtitle";
      when.textContent = new Date(note.updated).toLocaleString();
      const body = document.createElement("p");
      body.className = "notebody";
      body.textContent = note.body;
      content.append(title, when, body);

      const buttons = document.createElement("div");
      buttons.className = "rowbtns";
      const editBtn = document.createElement("button");
      editBtn.className = "glass-btn glass-btn--sm glass-btn--tertiary";
      editBtn.textContent = "Edit";
      on(editBtn, "click", () => {
        editingNote = note.id;
        $("noteTitle").value = note.title;
        $("noteBody").value = note.body;
        $("noteClear").hidden = false;
        $("noteSave").textContent = "Update note";
        $("noteTitle").focus();
      });
      const copyBtn = document.createElement("button");
      copyBtn.className = "glass-btn glass-btn--sm glass-btn--tertiary";
      copyBtn.textContent = "Copy";
      on(copyBtn, "click", async () => {
        toast((await copy(note.body)) ? "Copied" : "Could not reach the clipboard", "ok");
      });
      const delBtn = document.createElement("button");
      delBtn.className = "glass-btn glass-btn--sm glass-btn--tertiary";
      delBtn.textContent = "Delete";
      on(delBtn, "click", () => guard(async () => {
        if (!window.confirm(`Delete "${note.title}"? This cannot be undone.`)) return;
        await api("/api/vault/delete", { id: note.id });
        toast("Note deleted");
        await loadNotes();
      }));
      buttons.append(editBtn, copyBtn, delBtn);

      li.append(content, buttons);
      list.appendChild(li);
    });
  }

  function clearNoteForm() {
    editingNote = null;
    $("noteTitle").value = "";
    $("noteBody").value = "";
    $("noteClear").hidden = true;
    $("noteSave").textContent = "Save note";
  }

  function setupVault() {
    const unlock = () => guard(async () => {
      const password = $("vaultPw").value;
      await api("/api/vault/unlock", { password });
      $("vaultPw").value = "";
      toast("Vault unlocked");
      await refreshVaultState();
    });

    on($("vaultUnlock"), "click", unlock);
    on($("vaultPw"), "keydown", (e) => {
      if (e.key === "Enter") unlock();
    });

    on($("vaultLockBtn"), "click", () => guard(async () => {
      await api("/api/vault/lock", {});
      clearNoteForm();
      toast("Vault locked");
      await refreshVaultState();
    }));

    on($("noteSave"), "click", () => guard(async () => {
      await api("/api/vault/save", {
        id: editingNote || "",
        title: $("noteTitle").value,
        body: $("noteBody").value,
      });
      toast(editingNote ? "Note updated" : "Note saved");
      clearNoteForm();
      await loadNotes();
    }));

    on($("noteClear"), "click", clearNoteForm);
  }

  /* ------------------------------------------------------------ system */

  const SYS_ROWS = [
    ["os", "Operating system"],
    ["arch", "Architecture"],
    ["cpus", "Logical cores"],
    ["hostname", "Computer name"],
    ["username", "Signed in as"],
    ["localTime", "Local time"],
    ["uptime", "App running for"],
    ["memoryMB", "Memory in use (MB)"],
    ["appVersion", "Jacob Helpa"],
    ["goVersion", "Built with"],
    ["dataDir", "App folder"],
  ];

  async function loadSysinfo() {
    await guard(async () => {
      const data = await api("/api/sysinfo");
      const list = $("sysOut");
      list.textContent = "";
      SYS_ROWS.forEach(([key, label]) => {
        list.appendChild(listRow(label, String(data[key] ?? "—")));
      });
      (data.addresses || []).forEach((addr, i) => {
        list.appendChild(listRow(i === 0 ? "Network address" : "", addr));
      });
    });
  }

  function listRow(label, value) {
    const li = document.createElement("li");
    li.className = "glass-list__item";
    const content = document.createElement("div");
    content.className = "glass-list__content";
    const title = document.createElement("div");
    title.className = "glass-list__title";
    title.textContent = label;
    content.appendChild(title);
    const val = document.createElement("span");
    val.className = "glass-list__value selectable";
    val.textContent = value;
    li.append(content, val);
    return li;
  }

  /* ---------------------------------------------------------- settings */

  function applyConfig(cfg) {
    config = cfg;
    document.documentElement.setAttribute("data-theme", cfg.theme);
    document.documentElement.style.setProperty("--accent", cfg.accent);

    $("setAccent").value = cfg.accent;
    $("setGreeting").value = cfg.greeting || "";
    $("setSkip").checked = !!cfg.skipIntro;
    markSegment("setTheme", "mode", cfg.theme);
    markSegment("setBg", "bg", cfg.background);

    $("heroHi").textContent = cfg.greeting ? `Welcome back, ${cfg.greeting}` : "Welcome back";
    renderLinks();
  }

  function markSegment(hostId, attr, value) {
    $(hostId).querySelectorAll(".glass-segmented__item").forEach((b) => {
      const match = b.dataset[attr] === value;
      b.classList.toggle("is-on", match);
      b.setAttribute("aria-pressed", String(match));
    });
  }

  async function saveConfig(changes) {
    const next = { ...config, ...changes };
    const saved = await api("/api/config", next);
    applyConfig(saved);
  }

  function renderLinks() {
    const chips = $("linkChips");
    chips.textContent = "";
    const list = $("linkList");
    list.textContent = "";

    if (!config.links || !config.links.length) {
      const empty = document.createElement("li");
      empty.className = "glass-list__item glass-list__item--center";
      empty.textContent = "No shortcuts yet.";
      list.appendChild(empty);
      chips.textContent = "Nothing here yet — add some in Settings.";
      return;
    }

    config.links.forEach((item, index) => {
      const chip = document.createElement("button");
      chip.className = "glass-badge glass-badge--interactive";
      chip.textContent = item.label;
      on(chip, "click", () => guard(async () => {
        await api("/api/open", { target: item.target });
        toast(`Opening ${item.label}`);
      }));
      chips.appendChild(chip);

      const li = document.createElement("li");
      li.className = "glass-list__item";
      const content = document.createElement("div");
      content.className = "glass-list__content";
      const title = document.createElement("div");
      title.className = "glass-list__title";
      title.textContent = item.label;
      const sub = document.createElement("div");
      sub.className = "glass-list__subtitle glass-list__subtitle--wrap";
      sub.textContent = item.target;
      content.append(title, sub);

      const buttons = document.createElement("div");
      buttons.className = "rowbtns";
      const openBtn = document.createElement("button");
      openBtn.className = "glass-btn glass-btn--sm glass-btn--tertiary";
      openBtn.textContent = "Open";
      on(openBtn, "click", () => guard(async () => {
        await api("/api/open", { target: item.target });
      }));
      const delBtn = document.createElement("button");
      delBtn.className = "glass-btn glass-btn--sm glass-btn--tertiary";
      delBtn.textContent = "Remove";
      on(delBtn, "click", () => guard(async () => {
        const links = config.links.filter((_, i) => i !== index);
        await saveConfig({ links });
        toast("Shortcut removed");
      }));
      buttons.append(openBtn, delBtn);

      li.append(content, buttons);
      list.appendChild(li);
    });
  }

  function setupSettings() {
    $("setTheme").querySelectorAll(".glass-segmented__item").forEach((btn) => {
      on(btn, "click", () => guard(() => saveConfig({ theme: btn.dataset.mode })));
    });
    $("setBg").querySelectorAll(".glass-segmented__item").forEach((btn) => {
      on(btn, "click", () => guard(() => saveConfig({ background: btn.dataset.bg })));
    });
    on($("setAccent"), "change", () => guard(() => saveConfig({ accent: $("setAccent").value })));
    on($("setGreeting"), "change", () => guard(() => saveConfig({ greeting: $("setGreeting").value.trim() })));
    on($("setSkip"), "change", () => guard(() => saveConfig({ skipIntro: $("setSkip").checked })));

    on($("linkAdd"), "click", () => guard(async () => {
      const label = window.prompt("What should this shortcut be called?");
      if (!label) return;
      const target = window.prompt("Web link, or a file or folder on this machine:");
      if (!target) return;
      const links = [...(config.links || []), { label: label.trim(), target: target.trim() }];
      await saveConfig({ links });
      toast("Shortcut added");
    }));
  }

  /* -------------------------------------------------------------- chrome */

  function startClock() {
    const tick = () => {
      $("clock").textContent = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    };
    tick();
    setInterval(tick, 20000);
  }

  function setupChrome() {
    document.querySelectorAll(".navitem").forEach((btn) => {
      on(btn, "click", () => go(btn.dataset.view));
    });
    document.querySelectorAll("[data-goto]").forEach((el) => {
      on(el, "click", () => go(el.dataset.goto));
    });

    on($("quitBtn"), "click", async () => {
      document.body.style.transition = "opacity .3s ease";
      document.body.style.opacity = "0";
      try {
        await api("/api/quit", {});
      } catch {
        /* the server is going away, so a failure here is expected */
      }
      setTimeout(() => window.close(), 220);
    });

    // Ctrl+1..9 jumps between pages.
    on(document, "keydown", (e) => {
      if (!e.ctrlKey || e.shiftKey || e.altKey) return;
      const index = Number(e.key);
      if (!index) return;
      const items = [...document.querySelectorAll(".navitem")];
      if (items[index - 1]) {
        e.preventDefault();
        go(items[index - 1].dataset.view);
      }
    });
  }

  /* ---------------------------------------------------------------- go */

  async function main() {
    layOutWordmark("Jacob Helpa");

    let meta = { config: null, version: "2.0.0" };
    try {
      meta = await api("/api/meta");
    } catch (err) {
      console.error(err);
    }
    applyConfig(meta.config || {
      theme: "dark", background: "net", accent: "#5b8cff", greeting: "", skipIntro: false, links: [],
    });
    $("sideVersion").textContent = "v" + (meta.version || "2.0.0");

    const aboutRows = [
      ["Version", meta.version || "2.0.0"],
      ["Interface", "GlassKit (MIT)"],
      ["Welcome animation", "Vanta.js + three.js (MIT)"],
      ["Runs on", "a local server, no internet needed"],
    ];
    aboutRows.forEach(([k, v]) => $("aboutList").appendChild(listRow(k, v)));

    setupChrome();
    setupPasswords();
    setupUsernames();
    setupTokens();
    setupHash();
    setupText();
    setupVault();
    setupSettings();
    wireCopyAll();
    startClock();

    startVanta(config.background, config.accent);

    if (config.skipIntro) {
      runBootSequence();
      enterApp();
      return;
    }

    const typing = typeOut($("brandTag"), "your little toolkit, all offline");
    const booting = runBootSequence();
    await Promise.all([typing, booting]);
    const caret = document.querySelector(".caret");
    if (caret) caret.classList.add("is-done");

    const enter = $("enterBtn");
    enter.classList.add("is-ready");
    on(enter, "click", enterApp);
    // Let the finished screen breathe for a moment, then move on by itself.
    const auto = setTimeout(enterApp, 2600);
    on(enter, "click", () => clearTimeout(auto));
    on(document, "keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") {
        clearTimeout(auto);
        enterApp();
      }
    }, { once: true });
  }

  main();
})();
