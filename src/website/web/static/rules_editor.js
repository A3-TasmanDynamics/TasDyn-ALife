// /admin/rules: the rulebook as a document in the shared editor (editor.js,
// mode "rules"). Sections are Section headings (h2), subsections are
// Subsection headings (h3), rules are numbered list items and dot points
// are the list levels below them. On publish the page is read back into
// sections, subsections, rules and points and sent as JSON; the server
// writes the rules document, numbers it and checks it.
(function () {
  var form = document.querySelector("[data-rules-form]");
  var root = form && form.querySelector("[data-doc-editor]");
  if (!form || !root || !window.DocEditor) return;
  var modeInput = form.querySelector("[data-rules-mode]");
  var jsonInput = form.querySelector("[data-rules-json]");
  var textPane = form.querySelector("[data-rules-text]");
  var textarea = textPane.querySelector("textarea");
  var tabs = form.querySelectorAll("[data-rules-tab]");
  var page = root.querySelector("[data-doc-page]");
  var dirty = false, textDirty = false;

  var doc = { intro: "", sections: [] };
  try { doc = JSON.parse(root.getAttribute("data-rules") || "{}") || doc; } catch (e) { /* keep empty */ }
  if (Array.isArray(doc)) doc = { intro: "", sections: doc };
  doc.sections = doc.sections || [];

  function esc(s) {
    var d = document.createElement("div");
    d.textContent = s == null ? "" : String(s);
    return d.innerHTML;
  }

  // Model -> page.
  function rulesHTML(rules) {
    if (!rules || !rules.length) return "";
    return "<ol>" + rules.map(function (r) {
      var pts = (r.points || []).filter(function (p) { return p.text; });
      return "<li>" + esc(r.text) + (pts.length ? "<ul>" + pts.map(function (p) {
        var sub = (p.sub || []).filter(Boolean);
        return "<li>" + esc(p.text) + (sub.length ? "<ul>" + sub.map(function (s) { return "<li>" + esc(s) + "</li>"; }).join("") + "</ul>" : "") + "</li>";
      }).join("") + "</ul>" : "") + "</li>";
    }).join("") + "</ol>";
  }
  function toHTML(d) {
    var h = String(d.intro || "").split("\n").filter(function (p) { return p.trim(); }).map(function (p) { return "<p>" + esc(p) + "</p>"; }).join("");
    (d.sections || []).forEach(function (s) {
      h += "<h2>" + esc(s.title) + "</h2>";
      if (s.intro) h += "<p>" + esc(s.intro) + "</p>";
      h += rulesHTML(s.rules);
      (s.subs || []).forEach(function (sub) {
        h += "<h3>" + esc(sub.title) + "</h3>";
        if (sub.intro) h += "<p>" + esc(sub.intro) + "</p>";
        h += rulesHTML(sub.rules);
      });
    });
    if (!h) h = "<p>Welcome to the server. Please read these rules before you play.</p><h2>General rules</h2><ol><li>Treat other players and staff with respect.</li></ol>";
    return h;
  }

  // Page -> model.
  function own(el) {
    var c = el.cloneNode(true);
    c.querySelectorAll("ul,ol").forEach(function (l) { l.remove(); });
    return c.textContent.replace(/\s+/g, " ").trim();
  }
  function isList(el) { return el && (el.tagName === "UL" || el.tagName === "OL"); }
  // entries reads a list as items with the lists nested under each. Browsers
  // nest an indented list either inside the item or (Chrome) as the next
  // sibling of the item; both mean "under this item".
  function entries(list) {
    var out = [];
    Array.prototype.forEach.call(list.children, function (c) {
      if (c.tagName === "LI") {
        out.push({ li: c, lists: Array.prototype.filter.call(c.children, isList) });
      } else if (isList(c)) {
        if (out.length) out[out.length - 1].lists.push(c);
        else entries(c).forEach(function (e) { out.push(e); });
      }
    });
    return out;
  }
  function items(list) { return entries(list).map(function (e) { return e.li; }); }
  function deeper(lists) {
    var out = [];
    lists.forEach(function (l) {
      entries(l).forEach(function (e) {
        var t = own(e.li);
        if (t) out.push(t);
        out = out.concat(deeper(e.lists));
      });
    });
    return out;
  }
  function ruleFrom(e) {
    var r = { text: own(e.li), points: [] };
    e.lists.forEach(function (l) {
      entries(l).forEach(function (pe) { r.points.push({ text: own(pe.li), sub: deeper(pe.lists) }); });
    });
    return r;
  }
  function fromPage() {
    var d = { intro: [], sections: [] };
    var sec = null, target = null; // target: the heading whose rules we're filling
    var intro = [];
    Array.prototype.forEach.call(page.children, function (el) {
      var t = el.tagName;
      if (t === "H1" || t === "H2") {
        sec = { title: own(el), intro: "", rules: [], subs: [] };
        d.sections.push(sec);
        target = sec;
        return;
      }
      if (t === "H3" || t === "H4") {
        if (!sec) { sec = { title: "", intro: "", rules: [], subs: [] }; d.sections.push(sec); }
        target = { title: own(el), intro: "", rules: [] };
        sec.subs.push(target);
        return;
      }
      if (t === "OL" || t === "UL") {
        if (!target) { items(el).forEach(function (li) { var x = own(li); if (x) intro.push(x); }); return; }
        if (t === "UL" && target.rules.length) {
          // Bullets straight after a rule are its dot points.
          var last = target.rules[target.rules.length - 1];
          entries(el).forEach(function (pe) { last.points.push({ text: own(pe.li), sub: deeper(pe.lists) }); });
          return;
        }
        entries(el).forEach(function (e) { target.rules.push(ruleFrom(e)); });
        return;
      }
      var text = own(el);
      if (!text) return;
      if (!target) intro.push(text);
      else if (target.rules.length) target.rules.push({ text: text, points: [] });
      else target.intro = (target.intro ? target.intro + " " : "") + text;
    });
    d.intro = intro.join("\n");
    return d;
  }

  // The text form, matching the server's rules.FormatDoc.
  function one(s) { return String(s || "").split(/\s+/).filter(Boolean).join(" "); }
  function rulesText(rules, prefix) {
    return (rules || []).map(function (r, j) {
      var out = prefix + (j + 1) + " " + one(r.text) + "\n";
      (r.points || []).forEach(function (p) {
        if (!one(p.text)) return;
        out += "- " + one(p.text) + "\n";
        (p.sub || []).forEach(function (sp) { if (one(sp)) out += "  - " + one(sp) + "\n"; });
      });
      return out;
    }).join("");
  }
  function toText(d) {
    var head = String(d.intro || "").split("\n").map(one).filter(Boolean).map(function (p) { return "> " + p + "\n"; }).join("");
    return (head ? head + "\n" : "") + d.sections.map(function (s, i) {
      var out = "# " + one(s.title) + "\n";
      if (one(s.intro)) out += "> " + one(s.intro) + "\n";
      out += rulesText(s.rules, (i + 1) + ".");
      (s.subs || []).forEach(function (sub, k) {
        out += "## " + one(sub.title) + "\n";
        if (one(sub.intro)) out += "> " + one(sub.intro) + "\n";
        out += rulesText(sub.rules, (i + 1) + "." + ((s.rules || []).length + k + 1) + ".");
      });
      return out;
    }).join("\n");
  }

  page.innerHTML = toHTML(doc);
  var ed = window.DocEditor(root, { mode: "rules", onChange: function () { dirty = true; } });

  function setMode(mode) {
    if (mode === modeInput.value) return;
    if (mode === "text") {
      textarea.value = toText(fromPage());
      textDirty = false;
    } else {
      if (textDirty && !confirm("Switch back to the document? Your text edits will be lost (publish them first to keep them).")) return;
      ed.refresh();
    }
    modeInput.value = mode;
    root.hidden = mode !== "sections";
    textPane.hidden = mode !== "text";
    tabs.forEach(function (t) {
      var on = t.getAttribute("data-rules-tab") === mode;
      t.classList.toggle("active", on);
      t.setAttribute("aria-selected", on ? "true" : "false");
    });
  }
  tabs.forEach(function (t) { t.addEventListener("click", function () { setMode(t.getAttribute("data-rules-tab")); }); });
  textarea.addEventListener("input", function () { textDirty = true; dirty = true; });

  form.addEventListener("submit", function () {
    if (modeInput.value === "sections") jsonInput.value = JSON.stringify(fromPage());
    dirty = false;
  });
  document.addEventListener("keydown", function (e) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") { e.preventDefault(); form.requestSubmit(); }
  });
  window.addEventListener("beforeunload", function (e) {
    if (dirty) { e.preventDefault(); e.returnValue = ""; }
  });

  // For tests and debugging.
  window.RulesEditor = { fromPage: fromPage, toText: function () { return toText(fromPage()); } };
})();
