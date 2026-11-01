/*!
 * Posta form widget — https://goposta.dev
 * SPDX-License-Identifier: AGPL-3.0-or-later
 *
 * <div data-posta-form="PUBLIC_KEY"></div>
 * <script src="https://posta.example.com/widget/v1/posta-form.js" async></script>
 */
(function () {
  "use strict";

  if (window.PostaForms) return;

  var SCRIPT = document.currentScript;
  var BRAND_URL = "https://goposta.dev/?ref=form-widget";

  var STRINGS = {
    en: {
      name: "Name",
      email: "Email",
      phone: "Phone",
      subject: "Subject",
      message: "Message",
      optional: "optional",
      required: "This field is required.",
      choose: "Choose…",
      button: "Send message",
      another: "Submit another response",
      sending: "Sending…",
      success: "Thanks — your message has been received.",
      loading: "Loading…",
      unavailable: "This form is not accepting messages right now.",
      originDenied: "This form is not available on this website.",
      network: "Could not send. Check your connection and try again.",
      error: "Your message could not be sent. Please try again.",
      poweredBy: "Powered by",
    },
    fr: {
      name: "Nom",
      email: "E-mail",
      phone: "Téléphone",
      subject: "Objet",
      message: "Message",
      optional: "facultatif",
      required: "Ce champ est obligatoire.",
      choose: "Choisir…",
      button: "Envoyer",
      another: "Envoyer une autre réponse",
      sending: "Envoi…",
      success: "Merci, votre message a bien été reçu.",
      loading: "Chargement…",
      unavailable: "Ce formulaire n'accepte pas de messages pour le moment.",
      originDenied: "Ce formulaire n'est pas disponible sur ce site.",
      network: "Envoi impossible. Vérifiez votre connexion et réessayez.",
      error: "Votre message n'a pas pu être envoyé. Veuillez réessayer.",
      poweredBy: "Propulsé par",
    },
  };

  // The fields a form without a schema can show, keyed by the names the
  // server recognises (services/messages/parse.go).
  var BUILTIN_FIELDS = {
    name: { type: "text", autocomplete: "name", required: true },
    email: { type: "email", autocomplete: "email", required: true },
    phone: { type: "phone", autocomplete: "tel", required: false },
    subject: { type: "text", required: false },
    message: { type: "textarea", required: true },
  };
  var DEFAULT_FIELDS = ["name", "email", "message"];

  var MARK_SVG =
    '<svg viewBox="78 96 356 320" width="14" height="13" aria-hidden="true" focusable="false">' +
    '<path fill="currentColor" fill-rule="evenodd" d="M168,96 H344 A20,20 0 0 1 364,116 V272 A20,20 0 0 1 344,292 H168 A20,20 0 0 1 148,272 V116 A20,20 0 0 1 168,96 Z M187,149 H325 A11,11 0 0 1 336,160 V160 A11,11 0 0 1 325,171 H187 A11,11 0 0 1 176,160 V160 A11,11 0 0 1 187,149 Z M187,209 H285 A11,11 0 0 1 296,220 V220 A11,11 0 0 1 285,231 H187 A11,11 0 0 1 176,220 V220 A11,11 0 0 1 187,209 Z M78,250 L256,377 L434,250 L434,380 A36,36 0 0 1 398,416 L114,416 A36,36 0 0 1 78,380 Z"/>' +
    "</svg>";

  var STYLE =
    ":host{display:block}" +
    ".pf{--a:var(--posta-form-accent,var(--pf-accent,#9333EA));--r:var(--posta-form-radius,8px);" +
    // The theme sets --t-*; a site overrides any of them with --posta-form-*.
    "--t-bg:#fff;--t-fg:#101A37;--t-mut:#5A6486;--t-bd:#DFE5F2;--t-in:#fff;--t-err:#B42318;--t-ok:#067647;" +
    "--bg:var(--posta-form-bg,var(--t-bg));--fg:var(--posta-form-text,var(--t-fg));--mut:var(--posta-form-muted,var(--t-mut));" +
    "--bd:var(--posta-form-border,var(--t-bd));--in:var(--posta-form-input-bg,var(--t-in));--err:var(--posta-form-error,var(--t-err));" +
    "--ok:var(--posta-form-success,var(--t-ok));--at:var(--posta-form-accent-text,#fff);" +
    "box-sizing:border-box;max-width:var(--posta-form-max-width,480px);font-family:var(--posta-form-font,inherit);" +
    "font-size:15px;line-height:1.5;color:var(--fg);background:var(--bg);border:1px solid var(--bd);" +
    "border-radius:calc(var(--r) + 4px);padding:20px}" +
    ".pf[data-theme=dark]{--t-bg:#101A37;--t-fg:#F2F4FA;--t-mut:#A9B1CC;--t-bd:#2A3A66;--t-in:#0A1124;--t-err:#FDA29B;--t-ok:#75E0A7}" +
    "@media (prefers-color-scheme:dark){.pf[data-theme=auto]{--t-bg:#101A37;--t-fg:#F2F4FA;--t-mut:#A9B1CC;--t-bd:#2A3A66;--t-in:#0A1124;--t-err:#FDA29B;--t-ok:#75E0A7}}" +
    ".pf *{box-sizing:border-box}" +
    ".pf h2{margin:0 0 14px;font-size:1.2em;line-height:1.3}" +
    ".pf .desc{margin:-10px 0 14px;color:var(--mut)}" +
    ".pf .f{margin:0 0 12px}" +
    ".pf label{display:block;margin:0 0 4px;font-weight:600;font-size:.9em}" +
    ".pf .opt{font-weight:400;color:var(--mut)}" +
    ".pf fieldset{border:0;padding:0;min-width:0}" +
    ".pf legend{display:block;padding:0;margin:0 0 6px;font-weight:600;font-size:.9em}" +
    ".pf .help{margin:-2px 0 6px;font-size:.85em;color:var(--mut)}" +
    ".pf select{display:block;width:100%;margin:0;padding:9px 11px;font:inherit;color:var(--fg);background:var(--in);border:1px solid var(--bd);border-radius:var(--r)}" +
    ".pf .opts{display:grid;gap:6px}" +
    ".pf .opt-item,.pf .tick{display:flex;align-items:center;gap:8px;font-weight:400;font-size:1em;margin:0;cursor:pointer}" +
    ".pf .opt-item input,.pf .tick input{width:auto;margin:0;accent-color:var(--a)}" +
    ".pf .scale{display:flex;flex-wrap:wrap;align-items:center;gap:6px}" +
    ".pf .scale .end{font-size:.85em;color:var(--mut)}" +
    ".pf .scale .opt-item{position:relative;justify-content:center;min-width:38px;height:38px;border:1px solid var(--bd);border-radius:var(--r);background:var(--in)}" +
    ".pf .scale .opt-item input{position:absolute;opacity:0;width:100%;height:100%;cursor:pointer}" +
    ".pf .scale .opt-item:has(input:checked){background:var(--a);border-color:var(--a);color:var(--at)}" +
    ".pf .scale .opt-item:has(input:focus-visible){outline:2px solid var(--a);outline-offset:2px}" +
    ".pf .rating .opt-item{border:0;background:none;min-width:30px;font-size:1.5em;color:var(--bd)}" +
    ".pf .rating .opt-item.on,.pf .rating .opt-item:has(input:checked){background:none;color:#F5A524}" +
    ".pf .sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}" +
    ".pf input,.pf textarea{display:block;width:100%;margin:0;padding:9px 11px;font:inherit;color:var(--fg);" +
    "background:var(--in);border:1px solid var(--bd);border-radius:var(--r);transition:border-color .15s,box-shadow .15s}" +
    ".pf textarea{min-height:120px;resize:vertical}" +
    ".pf input:focus,.pf textarea:focus{outline:none;border-color:var(--a);box-shadow:0 0 0 3px color-mix(in srgb,var(--a) 25%,transparent)}" +
    ".pf [aria-invalid=true]{border-color:var(--err)}" +
    ".pf .opts[aria-invalid=true],.pf .scale[aria-invalid=true]{outline:1px solid var(--err);outline-offset:4px;border-radius:2px}" +
    ".pf .ferr{margin:4px 0 0;font-size:.85em;color:var(--err)}" +
    ".pf .hp{position:absolute;left:-9999px;width:1px;height:1px;overflow:hidden}" +
    ".pf button{display:inline-flex;align-items:center;justify-content:center;min-height:42px;padding:0 18px;font:inherit;font-weight:600;" +
    "color:var(--at);background:var(--a);border:0;border-radius:var(--r);cursor:pointer;transition:filter .15s}" +
    ".pf button:hover{filter:brightness(1.08)}" +
    ".pf button:disabled{opacity:.6;cursor:default;filter:none}" +
    ".pf button:focus-visible{outline:2px solid var(--a);outline-offset:2px}" +
    ".pf .st{margin:10px 0 0;font-size:.9em}" +
    ".pf .st:empty{display:none}" +
    ".pf .st.err{color:var(--err)}" +
    ".pf .done{margin:4px 0;color:var(--ok);font-weight:600}" +
    ".pf button.again{margin-top:10px;background:none;color:var(--a);padding:0;min-height:0;text-decoration:underline}" +
    ".pf .brand{display:flex;justify-content:flex-end;margin-top:16px;padding-top:10px;border-top:1px solid var(--bd)}" +
    ".pf .brand a{display:inline-flex;align-items:center;gap:5px;font-size:12px;color:var(--mut);text-decoration:none}" +
    ".pf .brand a:hover{color:var(--fg)}" +
    ".pf .brand strong{font-weight:700}" +
    ".pf .brand .mk{display:inline-flex}" +
    "@media (prefers-reduced-motion:reduce){.pf *{transition:none!important}}";

  var counter = 0;
  var sheet = null;

  function adoptStyle(root) {
    try {
      if (root.adoptedStyleSheets !== undefined && typeof CSSStyleSheet === "function") {
        if (!sheet) {
          sheet = new CSSStyleSheet();
          sheet.replaceSync(STYLE);
        }
        root.adoptedStyleSheets = root.adoptedStyleSheets.concat(sheet);
        return;
      }
    } catch (e) {
      // fall through to a <style> element
    }
    var style = document.createElement("style");
    style.textContent = STYLE;
    root.appendChild(style);
  }

  function apiBase(el) {
    var host = el.getAttribute("data-posta-host");
    if (host) return host.replace(/\/+$/, "") + "/api/v1/f";
    if (SCRIPT && SCRIPT.src) {
      var u = new URL(SCRIPT.src, document.baseURI);
      var prefix = u.pathname.replace(/\/widget\/v1\/posta-form\.js$/, "");
      return u.origin + prefix.replace(/\/+$/, "") + "/api/v1/f";
    }
    return "";
  }

  function pickLang(el) {
    var lang = (el.getAttribute("data-lang") || navigator.language || "en").toLowerCase().slice(0, 2);
    return STRINGS[lang] ? lang : "en";
  }

  // fieldList builds the field list from data-fields. Form data (schemas)
  // will hand the same shape to render() from /config instead.
  function fieldList(el, t) {
    var wanted = (el.getAttribute("data-fields") || DEFAULT_FIELDS.join(","))
      .split(",")
      .map(function (s) { return s.trim().toLowerCase(); })
      .filter(function (k, i, all) { return BUILTIN_FIELDS[k] && all.indexOf(k) === i; });
    if (wanted.indexOf("email") < 0) wanted.unshift("email");
    if (wanted.indexOf("message") < 0) wanted.push("message");
    return wanted.map(function (key) {
      var def = BUILTIN_FIELDS[key];
      return {
        key: key,
        label: el.getAttribute("data-label-" + key) || t[key],
        type: def.type,
        required: def.required,
        autocomplete: def.autocomplete,
      };
    });
  }

  function h(tag, attrs, text) {
    var node = document.createElement(tag);
    if (attrs) {
      for (var k in attrs) {
        if (attrs[k] !== undefined && attrs[k] !== null && attrs[k] !== false) node.setAttribute(k, attrs[k] === true ? "" : attrs[k]);
      }
    }
    if (text) node.textContent = text;
    return node;
  }

  function safeList(raw) {
    try {
      var v = JSON.parse(raw);
      return Array.isArray(v) ? v.map(String) : [];
    } catch (e) {
      return [];
    }
  }

  function emit(el, name, detail) {
    el.dispatchEvent(new CustomEvent("posta:" + name, { bubbles: true, composed: true, detail: detail }));
  }

  function getJSON(url) {
    return fetch(url, { credentials: "omit", headers: { Accept: "application/json" } }).then(function (res) {
      return res.json().then(function (body) { return { status: res.status, body: body }; }, function () {
        return { status: res.status, body: null };
      });
    });
  }

  function mount(el) {
    if (!el || el.__postaForm) return;
    var key = (el.getAttribute("data-posta-form") || "").trim();
    if (!key) {
      console.warn("[posta] data-posta-form needs the form's public key");
      return;
    }
    el.__postaForm = true;

    var base = apiBase(el);
    var t = STRINGS[pickLang(el)];
    var id = "pf" + ++counter;
    var root = el.attachShadow ? el.attachShadow({ mode: "open" }) : el;

    adoptStyle(root);

    var wrap = h("div", { class: "pf", part: "form" });
    // The form's saved appearance is the default; the embed's own attributes
    // win, so one form can look different on different pages.
    function applyLook(appearance) {
      var a = appearance || {};
      var theme = el.getAttribute("data-theme") || a.theme;
      wrap.setAttribute("data-theme", theme === "dark" || theme === "auto" ? theme : "light");
      var accent = el.getAttribute("data-accent") || a.accent;
      if (accent && window.CSS && CSS.supports && CSS.supports("color", accent)) wrap.style.setProperty("--pf-accent", accent);
    }
    applyLook(null);
    root.appendChild(wrap);

    var body = h("div");
    wrap.appendChild(body);

    var brand = h("div", { class: "brand" });
    var link = h("a", { href: BRAND_URL, target: "_blank", rel: "noopener" });
    link.appendChild(document.createTextNode(t.poweredBy + " "));
    var mark = h("span", { "aria-hidden": "true", class: "mk" });
    mark.innerHTML = MARK_SVG;
    link.appendChild(mark);
    link.appendChild(h("strong", null, "Posta"));
    brand.appendChild(link);
    wrap.appendChild(brand);

    function showNotice(text, isError) {
      body.textContent = "";
      body.appendChild(h("p", { class: isError === false ? "st" : "st err", role: "status" }, text));
    }

    function load() {
      body.textContent = "";
      body.appendChild(h("p", { class: "st" }, t.loading));
      getJSON(base + "/" + encodeURIComponent(key) + "/config").then(
        function (res) {
          var cfg = res.body && res.body.data;
          if (!cfg || !cfg.enabled) {
            var reason = (cfg && cfg.reason) || "unavailable";
            if (reason === "origin_not_allowed") {
              console.warn("[posta] " + location.origin + " is not in this form's allowed origins. Add it on the form's Setup tab in Posta.");
              showNotice(t.originDenied);
            } else if (reason === "closed") {
              showNotice(cfg.closed_message || t.unavailable, false);
            } else {
              showNotice(t.unavailable);
            }
            emit(el, "error", { key: key, status: res.status, code: reason, message: reason });
            return;
          }
          applyLook(cfg.appearance);
          render(cfg, cfg.fields && cfg.fields.length ? cfg.fields : fieldList(el, t));
        },
        function () {
          showNotice(t.network);
          emit(el, "error", { key: key, status: 0, code: "network", message: t.network });
        }
      );
    }
    load();

    // buildField renders one field and returns how to read it, focus it and
    // show an error on it.
    function buildField(f, fid) {
      var errId = fid + "-err";
      var helpId = fid + "-help";
      var type = f.type || "text";
      var preset = el.getAttribute("data-value-" + f.key) || "";
      // A multi-value preset is a JSON array (from a hosted page link) or a
      // comma-separated list (written by hand).
      var presets = preset.charAt(0) === "[" ? safeList(preset) : preset ? preset.split(",").map(function (v) { return v.trim(); }) : [];

      if (type === "hidden") {
        var hidden = h("input", { type: "hidden", name: f.key, value: preset });
        return { row: hidden, get: function () { return hidden.value; }, focus: function () {}, mark: function () {}, clear: function () {} };
      }

      var grouped = type === "scale" || type === "rating" || type === "multiselect" && f.display !== "list" ||
        type === "select" && (f.display === "radio" || !f.display && (f.options || []).length <= 6);
      var row = h(grouped ? "fieldset" : "div", { class: "f" });
      var caption = h(grouped ? "legend" : "label", grouped ? null : { for: fid }, type === "checkbox" ? "" : f.label || f.key);
      if (type !== "checkbox") {
        if (!f.required) {
          caption.appendChild(document.createTextNode(" "));
          caption.appendChild(h("span", { class: "opt" }, "(" + t.optional + ")"));
        }
        row.appendChild(caption);
      }
      var help = f.help ? h("p", { class: "help", id: helpId }, f.help) : null;
      if (help) row.appendChild(help);
      var describedBy = help ? helpId : null;

      var focusEl, get, marked = [];
      var requiredGroup = false;

      if (type === "textarea") {
        focusEl = h("textarea", { id: fid, name: f.key, required: !!f.required, maxlength: f.max_len, minlength: f.min_len, placeholder: f.placeholder, "aria-describedby": describedBy });
        focusEl.value = preset;
        row.appendChild(focusEl);
        get = function () { return focusEl.value; };
        marked = [focusEl];
      } else if (type === "checkbox") {
        focusEl = h("input", { id: fid, type: "checkbox", name: f.key, required: !!f.required, "aria-describedby": describedBy });
        var tick = h("label", { class: "tick", for: fid });
        focusEl.checked = /^(true|1|on|yes)$/i.test(preset);
        tick.appendChild(focusEl);
        tick.appendChild(h("span", null, f.label || f.key));
        if (help) row.insertBefore(tick, help);
        else row.appendChild(tick);
        get = function () { return focusEl.checked; };
        marked = [focusEl];
      } else if (type === "select" && !grouped) {
        focusEl = h("select", { id: fid, name: f.key, required: !!f.required, "aria-describedby": describedBy });
        focusEl.appendChild(h("option", { value: "" }, t.choose));
        (f.options || []).forEach(function (o) { focusEl.appendChild(h("option", { value: o.value }, o.label || o.value)); });
        focusEl.value = preset;
        if (focusEl.value !== preset) focusEl.value = "";
        row.appendChild(focusEl);
        get = function () { return focusEl.value; };
        marked = [focusEl];
      } else if (type === "multiselect" && !grouped) {
        focusEl = h("select", { id: fid, name: f.key, multiple: true, size: Math.min((f.options || []).length, 6), "aria-describedby": describedBy });
        (f.options || []).forEach(function (o) {
          focusEl.appendChild(h("option", { value: o.value, selected: presets.indexOf(o.value) >= 0 }, o.label || o.value));
        });
        row.appendChild(focusEl);
        get = function () {
          return Array.prototype.filter.call(focusEl.options, function (o) { return o.selected; }).map(function (o) { return o.value; });
        };
        requiredGroup = !!f.required;
        marked = [focusEl];
      } else if (grouped) {
        var choices = [];
        if (type === "scale" || type === "rating") {
          var lo = type === "rating" ? 1 : f.min != null ? f.min : 1;
          var hi = f.max != null ? f.max : 5;
          for (var n = lo; n <= hi; n++) choices.push({ value: String(n), label: type === "rating" ? "★" : String(n), title: String(n) });
        } else {
          choices = (f.options || []).map(function (o) { return { value: o.value, label: o.label || o.value }; });
        }
        var multi = type === "multiselect";
        var list = h("div", { class: type === "scale" || type === "rating" ? "scale " + type : "opts", role: multi ? "group" : "radiogroup", "aria-describedby": describedBy });
        if (type === "scale" && f.min_label) list.appendChild(h("span", { class: "end" }, f.min_label));
        var boxes = choices.map(function (ch, i) {
          var cid = fid + "-" + i;
          var box = h("input", { id: cid, type: multi ? "checkbox" : "radio", name: f.key, value: ch.value, required: !multi && !!f.required });
          box.checked = multi ? presets.indexOf(ch.value) >= 0 : ch.value === preset;
          var item = h("label", { class: "opt-item", for: cid, title: ch.title || null });
          item.appendChild(box);
          item.appendChild(h("span", type === "rating" ? { "aria-hidden": "true" } : null, ch.label));
          if (type === "rating") item.appendChild(h("span", { class: "sr" }, ch.title + " / " + hi));
          list.appendChild(item);
          return box;
        });
        if (type === "scale" && f.max_label) list.appendChild(h("span", { class: "end" }, f.max_label));
        if (type === "rating") {
          var paint = function () {
            var picked = 0;
            boxes.forEach(function (b, i) { if (b.checked) picked = i + 1; });
            boxes.forEach(function (b, i) { b.parentNode.classList.toggle("on", i < picked); });
          };
          list.addEventListener("change", paint);
          paint();
        }
        row.appendChild(list);
        focusEl = boxes[0];
        get = multi
          ? function () { return boxes.filter(function (b) { return b.checked; }).map(function (b) { return b.value; }); }
          : function () { var b = boxes.filter(function (b) { return b.checked; })[0]; return b ? b.value : ""; };
        requiredGroup = multi && !!f.required;
        marked = [list];
      } else {
        var inputType = { phone: "tel", email: "email", number: "number", url: "url", date: "date", time: "time" }[type] || "text";
        focusEl = h("input", {
          id: fid,
          name: f.key,
          type: inputType,
          required: !!f.required,
          autocomplete: f.autocomplete || { email: "email", tel: "tel" }[inputType],
          maxlength: f.max_len,
          minlength: f.min_len,
          min: inputType === "number" ? f.min : null,
          max: inputType === "number" ? f.max : null,
          step: inputType === "number" ? "any" : null,
          placeholder: f.placeholder,
          "aria-describedby": describedBy,
        });
        focusEl.value = preset;
        row.appendChild(focusEl);
        get = function () { return focusEl.value; };
        marked = [focusEl];
      }

      var err = h("p", { class: "ferr", id: errId, hidden: true });
      row.appendChild(err);
      var control = {
        row: row,
        get: get,
        requiredGroup: requiredGroup,
        focus: function () { focusEl.focus(); },
        mark: function (text) {
          err.textContent = text;
          err.hidden = false;
          marked.forEach(function (m) {
            m.setAttribute("aria-invalid", "true");
            m.setAttribute("aria-describedby", describedBy ? describedBy + " " + errId : errId);
          });
        },
        clear: function () {
          err.hidden = true;
          err.textContent = "";
          marked.forEach(function (m) {
            m.removeAttribute("aria-invalid");
            if (describedBy) m.setAttribute("aria-describedby", describedBy);
            else m.removeAttribute("aria-describedby");
          });
        },
      };
      // An error describes the value that was sent; once the visitor
      // changes the field it no longer applies.
      row.addEventListener("input", function () { if (!err.hidden) control.clear(); });
      row.addEventListener("change", function () { if (!err.hidden) control.clear(); });
      return control;
    }

    function render(cfg, fields) {
      body.textContent = "";
      var nonce = cfg.nonce ? cfg.nonce.nonce : "";
      // The server rejects a nonce younger than min_fill_seconds as a bot.
      // A fast human (or autofill) would lose their message silently, so
      // hold the submit until the nonce is old enough.
      var nonceAt = Date.now();
      var minFillMs = (cfg.min_fill_seconds || 0) * 1000;

      var title = el.getAttribute("data-title");
      var description = el.getAttribute("data-description");
      if (title) body.appendChild(h("h2", null, title));
      if (description) body.appendChild(h("p", { class: "desc" }, description));

      var form = h("form", { "aria-label": title || null });

      var controls = {};

      fields.forEach(function (f) {
        var c = buildField(f, id + "-" + f.key);
        controls[f.key] = c;
        form.appendChild(c.row);
      });

      var hpWrap = h("div", { class: "hp", "aria-hidden": "true" });
      var honeypot = h("input", { type: "text", name: cfg.honeypot_field || "_gotcha", tabindex: "-1", autocomplete: "off" });
      hpWrap.appendChild(honeypot);
      form.appendChild(hpWrap);

      var buttonText = el.getAttribute("data-button") || t.button;
      var button = h("button", { type: "submit" }, buttonText);
      form.appendChild(button);
      var status = h("p", { class: "st", role: "status", "aria-live": "polite" });
      form.appendChild(status);
      body.appendChild(form);

      function clearErrors() {
        status.textContent = "";
        status.className = "st";
        Object.keys(controls).forEach(function (k) { controls[k].clear(); });
      }

      function fail(text) {
        status.textContent = text;
        status.className = "st err";
      }

      function refreshNonce() {
        if (!cfg.require_nonce) return Promise.resolve();
        return getJSON(base + "/" + encodeURIComponent(key) + "/nonce").then(
          function (res) {
            nonce = res.body && res.body.data ? res.body.data.nonce : "";
            nonceAt = Date.now();
          },
          function () { nonce = ""; }
        );
      }

      form.addEventListener("submit", function (e) {
        e.preventDefault();
        clearErrors();
        var payload = {};
        var missing = null;
        Object.keys(controls).forEach(function (k) {
          var c = controls[k];
          payload[k] = c.get();
          if (c.requiredGroup && !payload[k].length) {
            c.mark(t.required);
            missing = missing || c;
          }
        });
        // Native validation covers single inputs; a required group of
        // checkboxes has no native check.
        if (missing) {
          missing.focus();
          return;
        }
        payload[honeypot.name] = honeypot.value;
        if (nonce) payload._nonce = nonce;

        button.disabled = true;
        button.textContent = t.sending;
        emit(el, "submit", { key: key });

        var wait = nonce ? Math.max(0, minFillMs + 250 - (Date.now() - nonceAt)) : 0;
        new Promise(function (resolve) { setTimeout(resolve, wait); })
          .then(function () {
            return fetch(base + "/" + encodeURIComponent(key), {
              method: "POST",
              credentials: "omit",
              // text/plain keeps this a CORS simple request: no preflight.
              headers: { "Content-Type": "text/plain;charset=UTF-8", Accept: "application/json" },
              body: JSON.stringify(payload),
            });
          })
          .then(function (res) {
            return res.json().then(
              function (b) { return { status: res.status, body: b }; },
              function () { return { status: res.status, body: null }; }
            );
          })
          .then(
            function (res) {
              var data = (res.body && res.body.data) || {};
              if (res.status >= 200 && res.status < 300) {
                body.textContent = "";
                var done = el.getAttribute("data-success") || cfg.confirmation_message || data.message || t.success;
                body.appendChild(h("p", { class: "done", role: "status" }, done));
                if (cfg.allow_another_response) {
                  var again = h("button", { type: "button", class: "again" }, t.another);
                  again.addEventListener("click", load);
                  body.appendChild(again);
                }
                emit(el, "success", { key: key, id: data.id || null });
                return;
              }
              var fieldErrs = (res.body && res.body.error && res.body.error.fields) || [];
              var first = null;
              fieldErrs.forEach(function (fe) {
                var c = controls[fe.key];
                if (!c) return;
                c.mark(fe.message || fe.code);
                first = first || c;
              });
              var message = data.message || (res.body && res.body.error && res.body.error.message) || (res.body && res.body.message) || t.error;
              if (!first) fail(message);
              else first.focus();
              var code = data.status || (res.body && res.body.error && res.body.error.code) || "error";
              emit(el, "error", { key: key, status: res.status, code: code, message: message });
              return refreshNonce();
            },
            function () {
              fail(t.network);
              emit(el, "error", { key: key, status: 0, code: "network", message: t.network });
              return refreshNonce();
            }
          )
          .then(function () {
            button.disabled = false;
            button.textContent = buttonText;
          });
      });

      emit(el, "ready", { key: key });
    }
  }

  function scan(scope) {
    var nodes = (scope || document).querySelectorAll("[data-posta-form]");
    for (var i = 0; i < nodes.length; i++) mount(nodes[i]);
  }

  window.PostaForms = { mount: mount, scan: scan, version: "1" };

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", function () { scan(); });
  else scan();
})();
