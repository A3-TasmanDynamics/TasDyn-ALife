// Section editor for /admin/rules: sections, numbered rules and dot points,
// sent as JSON (the server writes the rules document and numbers them).
(function () {
  var form = document.querySelector("[data-rules-form]");
  var root = document.querySelector("[data-rules-editor]");
  if (!form || !root) return;
  var modeInput = form.querySelector("[data-rules-mode]");
  var jsonInput = form.querySelector("[data-rules-json]");
  var textPane = form.querySelector("[data-rules-text]");
  var textarea = textPane.querySelector("textarea");
  var tabs = form.querySelectorAll("[data-rules-tab]");

  var model = [];
  try { model = JSON.parse(root.getAttribute("data-rules") || "[]") || []; } catch (e) { model = []; }
  var textDirty = false;

  var SUGGESTED = ["General rules", "Gameplay rules", "Roleplay rules", "Safe zones", "TeamSpeak rules", "Discord rules", "Police rules", "EMS rules", "Gang rules"];

  function h(tag, attrs, kids) {
    var el = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === "on") { for (var ev in attrs.on) el.addEventListener(ev, attrs.on[ev]); }
      else if (k === "text") el.textContent = attrs[k];
      else if (attrs[k] === true) el.setAttribute(k, "");
      else if (attrs[k] !== false && attrs[k] != null) el.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) { if (c) el.appendChild(typeof c === "string" ? document.createTextNode(c) : c); });
    return el;
  }

  function grow(t) { t.style.height = "auto"; t.style.height = t.scrollHeight + 2 + "px"; }

  var ICONS = { "↑": "M12 19V5M5 12l7-7 7 7", "↓": "M12 5v14M5 12l7 7 7-7", "×": "M6 6l12 12M18 6L6 18" };
  function iconBtn(label, glyph, fn, cls) {
    var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("width", "14"); svg.setAttribute("height", "14"); svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("fill", "none"); svg.setAttribute("stroke", "currentColor"); svg.setAttribute("stroke-width", "2.2");
    svg.setAttribute("stroke-linecap", "round"); svg.setAttribute("stroke-linejoin", "round"); svg.setAttribute("aria-hidden", "true");
    var path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("d", ICONS[glyph]); svg.appendChild(path);
    return h("button", { type: "button", class: "re-icon" + (cls ? " " + cls : ""), "aria-label": label, title: label, on: { click: fn } }, [svg]);
  }

  function move(list, i, d) {
    var j = i + d;
    if (j < 0 || j >= list.length) return;
    var x = list[i]; list[i] = list[j]; list[j] = x;
    render();
  }

  function focusLast(sel) {
    var els = root.querySelectorAll(sel);
    if (els.length) els[els.length - 1].focus();
  }

  function render() {
    root.textContent = "";
    if (!model.length) {
      root.appendChild(h("p", { class: "re-empty", text: "No sections yet. Add your first one below." }));
    }
    model.forEach(function (sec, si) {
      var n = si + 1;
      var rulesBox = h("div", { class: "re-rules" });
      sec.rules = sec.rules || [];
      sec.rules.forEach(function (rule, ri) {
        rule.points = rule.points || [];
        var pts = h("ul", { class: "re-points" });
        rule.points.forEach(function (pt, pi) {
          pt.sub = pt.sub || [];
          var subs = h("ul", { class: "re-points re-sub" });
          pt.sub.forEach(function (sp, qi) {
            subs.appendChild(h("li", { class: "re-point" }, [
              h("input", { class: "re-input", value: sp, "aria-label": "Rule " + n + "." + (ri + 1) + " sub-point", placeholder: "Sub-point", on: { input: function (e) { pt.sub[qi] = e.target.value; } } }),
              iconBtn("Delete sub-point", "×", function () { pt.sub.splice(qi, 1); render(); }, "re-del")
            ]));
          });
          pts.appendChild(h("li", { class: "re-point-wrap" }, [
            h("div", { class: "re-point" }, [
              h("input", { class: "re-input", value: pt.text, "aria-label": "Rule " + n + "." + (ri + 1) + " dot point", placeholder: "Dot point", on: { input: function (e) { pt.text = e.target.value; } } }),
              h("button", { type: "button", class: "re-mini", on: { click: function () { pt.sub.push(""); render(); focusLast(".re-sub .re-input"); } } }, ["+ Sub-point"]),
              iconBtn("Delete dot point", "×", function () { rule.points.splice(pi, 1); render(); }, "re-del")
            ]),
            pt.sub.length ? subs : null
          ]));
        });
        var ta = h("textarea", { class: "re-input re-rule-text", rows: "1", "aria-label": "Rule " + n + "." + (ri + 1), placeholder: "Write the rule", on: { input: function (e) { rule.text = e.target.value; grow(e.target); } } });
        ta.value = rule.text || "";
        rulesBox.appendChild(h("div", { class: "re-rule" }, [
          h("span", { class: "re-num", text: n + "." + (ri + 1) }),
          h("div", { class: "re-rule-body" }, [
            ta,
            rule.points.length ? pts : null,
            h("button", { type: "button", class: "re-mini", on: { click: function () { rule.points.push({ text: "", sub: [] }); render(); focusLast(".re-point-wrap > .re-point .re-input"); } } }, ["+ Dot point"])
          ]),
          h("div", { class: "re-tools" }, [
            iconBtn("Move rule up", "↑", function () { move(sec.rules, ri, -1); }),
            iconBtn("Move rule down", "↓", function () { move(sec.rules, ri, 1); }),
            iconBtn("Delete rule", "×", function () {
              if (!rule.text || confirm("Delete rule " + n + "." + (ri + 1) + "?")) { sec.rules.splice(ri, 1); render(); }
            }, "re-del")
          ])
        ]));
      });

      var intro = h("textarea", { class: "re-input re-intro", rows: "1", "aria-label": "Section " + n + " intro", placeholder: "Optional intro under the section title", on: { input: function (e) { sec.intro = e.target.value; grow(e.target); } } });
      intro.value = sec.intro || "";
      root.appendChild(h("section", { class: "re-section" }, [
        h("div", { class: "re-section-head" }, [
          h("span", { class: "re-section-n", text: n + "." }),
          h("input", { class: "re-input re-title", value: sec.title || "", "aria-label": "Section " + n + " title", placeholder: "Section title, e.g. General rules", on: { input: function (e) { sec.title = e.target.value; } } }),
          h("div", { class: "re-tools" }, [
            iconBtn("Move section up", "↑", function () { move(model, si, -1); }),
            iconBtn("Move section down", "↓", function () { move(model, si, 1); }),
            iconBtn("Delete section", "×", function () {
              if (!sec.rules.length || confirm("Delete “" + (sec.title || "this section") + "” and its " + sec.rules.length + " rule(s)?")) { model.splice(si, 1); render(); }
            }, "re-del")
          ])
        ]),
        intro,
        rulesBox,
        h("button", { type: "button", class: "btn re-add-rule", on: { click: function () { sec.rules.push({ text: "", points: [] }); render(); focusLast(".re-section:nth-of-type(" + n + ") .re-rule-text"); } } }, ["+ Add rule"])
      ]));
    });

    var have = {};
    model.forEach(function (s) { have[(s.title || "").toLowerCase()] = true; });
    var chips = SUGGESTED.filter(function (t) { return !have[t.toLowerCase()]; }).map(function (t) {
      return h("button", { type: "button", class: "re-chip", on: { click: function () { addSection(t); } } }, ["+ " + t]);
    });
    root.appendChild(h("div", { class: "re-add" }, [
      h("button", { type: "button", class: "btn btn-primary", on: { click: function () { addSection(""); } } }, ["+ Add section"]),
      chips.length ? h("span", { class: "hint", text: "or quickly add:" }) : null
    ].concat(chips)));

    root.querySelectorAll("textarea").forEach(grow);
  }

  function addSection(title) {
    model.push({ title: title, intro: "", rules: [{ text: "", points: [] }] });
    render();
    focusLast(title ? ".re-rule-text" : ".re-title");
  }

  // The text form, matching the server's rules.Format.
  function one(s) { return String(s || "").split(/\s+/).filter(Boolean).join(" "); }
  function toText() {
    return model.map(function (s, i) {
      var out = "# " + one(s.title) + "\n";
      if (one(s.intro)) out += "> " + one(s.intro) + "\n";
      (s.rules || []).forEach(function (r, j) {
        out += (i + 1) + "." + (j + 1) + " " + one(r.text) + "\n";
        (r.points || []).forEach(function (p) {
          if (!one(p.text)) return;
          out += "- " + one(p.text) + "\n";
          (p.sub || []).forEach(function (sp) { if (one(sp)) out += "  - " + one(sp) + "\n"; });
        });
      });
      return out;
    }).join("\n");
  }

  function setMode(mode) {
    if (mode === modeInput.value) return;
    if (mode === "text") {
      textarea.value = toText();
      textDirty = false;
    } else if (textDirty && !confirm("Switch back to sections? Your text edits will be lost (publish them first to keep them).")) {
      return;
    }
    modeInput.value = mode;
    root.hidden = mode !== "sections";
    textPane.hidden = mode !== "text";
    tabs.forEach(function (t) {
      var on = t.getAttribute("data-rules-tab") === mode;
      t.classList.toggle("active", on);
      t.setAttribute("aria-selected", on ? "true" : "false");
    });
    if (mode === "sections") render();
  }

  tabs.forEach(function (t) { t.addEventListener("click", function () { setMode(t.getAttribute("data-rules-tab")); }); });
  textarea.addEventListener("input", function () { textDirty = true; });
  form.addEventListener("submit", function () {
    if (modeInput.value === "sections") jsonInput.value = JSON.stringify(model);
  });

  render();
})();
