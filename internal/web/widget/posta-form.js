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
      button: "Send message",
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
      button: "Envoyer",
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
    phone: { type: "tel", autocomplete: "tel", required: false },
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
    "--bg:#fff;--fg:#101A37;--mut:#5A6486;--bd:#DFE5F2;--in:#fff;--err:#B42318;--ok:#067647;" +
    "box-sizing:border-box;max-width:var(--posta-form-max-width,480px);font-family:var(--posta-form-font,inherit);" +
    "font-size:15px;line-height:1.5;color:var(--fg);background:var(--bg);border:1px solid var(--bd);" +
    "border-radius:calc(var(--r) + 4px);padding:20px}" +
    ".pf[data-theme=dark]{--bg:#101A37;--fg:#F2F4FA;--mut:#A9B1CC;--bd:#2A3A66;--in:#0A1124;--err:#FDA29B;--ok:#75E0A7}" +
    "@media (prefers-color-scheme:dark){.pf[data-theme=auto]{--bg:#101A37;--fg:#F2F4FA;--mut:#A9B1CC;--bd:#2A3A66;--in:#0A1124;--err:#FDA29B;--ok:#75E0A7}}" +
    ".pf *{box-sizing:border-box}" +
    ".pf h2{margin:0 0 14px;font-size:1.2em;line-height:1.3}" +
    ".pf .desc{margin:-10px 0 14px;color:var(--mut)}" +
    ".pf .f{margin:0 0 12px}" +
    ".pf label{display:block;margin:0 0 4px;font-weight:600;font-size:.9em}" +
    ".pf .opt{font-weight:400;color:var(--mut)}" +
    ".pf input,.pf textarea{display:block;width:100%;margin:0;padding:9px 11px;font:inherit;color:var(--fg);" +
    "background:var(--in);border:1px solid var(--bd);border-radius:var(--r);transition:border-color .15s,box-shadow .15s}" +
    ".pf textarea{min-height:120px;resize:vertical}" +
    ".pf input:focus,.pf textarea:focus{outline:none;border-color:var(--a);box-shadow:0 0 0 3px color-mix(in srgb,var(--a) 25%,transparent)}" +
    ".pf [aria-invalid=true]{border-color:var(--err)}" +
    ".pf .ferr{margin:4px 0 0;font-size:.85em;color:var(--err)}" +
    ".pf .hp{position:absolute;left:-9999px;width:1px;height:1px;overflow:hidden}" +
    ".pf button{display:inline-flex;align-items:center;justify-content:center;min-height:42px;padding:0 18px;font:inherit;font-weight:600;" +
    "color:#fff;background:var(--a);border:0;border-radius:var(--r);cursor:pointer;transition:filter .15s}" +
    ".pf button:hover{filter:brightness(1.08)}" +
    ".pf button:disabled{opacity:.6;cursor:default;filter:none}" +
    ".pf button:focus-visible{outline:2px solid var(--a);outline-offset:2px}" +
    ".pf .st{margin:10px 0 0;font-size:.9em}" +
    ".pf .st:empty{display:none}" +
    ".pf .st.err{color:var(--err)}" +
    ".pf .done{margin:4px 0;color:var(--ok);font-weight:600}" +
    ".pf .brand{display:flex;justify-content:flex-end;margin-top:16px;padding-top:10px;border-top:1px solid var(--bd)}" +
    ".pf .brand a{display:inline-flex;align-items:center;gap:5px;font-size:12px;color:var(--mut);text-decoration:none}" +
    ".pf .brand a:hover{color:var(--fg)}" +
    ".pf .brand strong{font-weight:700}" +
    "@media (prefers-reduced-motion:reduce){.pf *{transition:none!important}}";

  var counter = 0;

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

    var style = document.createElement("style");
    style.textContent = STYLE;
    root.appendChild(style);

    var theme = el.getAttribute("data-theme");
    var wrap = h("div", { class: "pf", "data-theme": theme === "dark" || theme === "auto" ? theme : "light", part: "form" });
    var accent = el.getAttribute("data-accent");
    if (accent && window.CSS && CSS.supports && CSS.supports("color", accent)) wrap.style.setProperty("--pf-accent", accent);
    root.appendChild(wrap);

    var body = h("div");
    wrap.appendChild(body);

    var brand = h("div", { class: "brand" });
    var link = h("a", { href: BRAND_URL, target: "_blank", rel: "noopener" });
    link.appendChild(document.createTextNode(t.poweredBy + " "));
    var mark = h("span", { "aria-hidden": "true", style: "display:inline-flex" });
    mark.innerHTML = MARK_SVG;
    link.appendChild(mark);
    link.appendChild(h("strong", null, "Posta"));
    brand.appendChild(link);
    wrap.appendChild(brand);

    var loading = h("p", { class: "st" }, t.loading);
    body.appendChild(loading);

    function showNotice(text) {
      body.textContent = "";
      body.appendChild(h("p", { class: "st err", role: "status" }, text));
    }

    getJSON(base + "/" + encodeURIComponent(key) + "/config").then(
      function (res) {
        var cfg = res.body && res.body.data;
        if (!cfg || !cfg.enabled) {
          var reason = (cfg && cfg.reason) || "unavailable";
          if (reason === "origin_not_allowed") {
            console.warn("[posta] " + location.origin + " is not in this form's allowed origins. Add it on the form's Setup tab in Posta.");
            showNotice(t.originDenied);
          } else {
            showNotice(t.unavailable);
          }
          emit(el, "error", { key: key, status: res.status, code: reason, message: reason });
          return;
        }
        render(cfg, cfg.fields && cfg.fields.length ? cfg.fields : fieldList(el, t));
      },
      function () {
        showNotice(t.network);
        emit(el, "error", { key: key, status: 0, code: "network", message: t.network });
      }
    );

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
      var inputs = {};
      var errors = {};

      fields.forEach(function (f) {
        var fid = id + "-" + f.key;
        var row = h("div", { class: "f" });
        var label = h("label", { for: fid }, f.label || f.key);
        if (!f.required) {
          label.appendChild(document.createTextNode(" "));
          label.appendChild(h("span", { class: "opt" }, "(" + t.optional + ")"));
        }
        row.appendChild(label);
        var input =
          f.type === "textarea"
            ? h("textarea", { id: fid, name: f.key, required: !!f.required, maxlength: f.max_len })
            : h("input", {
                id: fid,
                name: f.key,
                type: f.type || "text",
                required: !!f.required,
                autocomplete: f.autocomplete,
                maxlength: f.max_len,
                placeholder: f.placeholder,
              });
        row.appendChild(input);
        var err = h("p", { class: "ferr", id: fid + "-err", hidden: true });
        row.appendChild(err);
        inputs[f.key] = input;
        errors[f.key] = err;
        form.appendChild(row);
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
        Object.keys(errors).forEach(function (k) {
          errors[k].hidden = true;
          errors[k].textContent = "";
          inputs[k].removeAttribute("aria-invalid");
          inputs[k].removeAttribute("aria-describedby");
        });
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
        Object.keys(inputs).forEach(function (k) { payload[k] = inputs[k].value; });
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
                body.appendChild(h("p", { class: "done", role: "status" }, el.getAttribute("data-success") || data.message || t.success));
                emit(el, "success", { key: key, id: data.id || null });
                return;
              }
              var fieldErrs = (res.body && res.body.error && res.body.error.fields) || [];
              var first = null;
              fieldErrs.forEach(function (fe) {
                if (!inputs[fe.key]) return;
                errors[fe.key].textContent = fe.message || fe.code;
                errors[fe.key].hidden = false;
                inputs[fe.key].setAttribute("aria-invalid", "true");
                inputs[fe.key].setAttribute("aria-describedby", errors[fe.key].id);
                first = first || inputs[fe.key];
              });
              var message = data.message || (res.body && res.body.message) || t.error;
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
