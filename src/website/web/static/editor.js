// A Google Docs-style editor: a toolbar with a style menu, an outline of
// the document's headings, and a page you type into. Used by the faction
// drive (mode "doc": the HTML is cleaned on the server by SanitizeDoc,
// which is what makes it safe) and the server rules (mode "rules": headings
// are sections, numbered lists are rules; see rules_editor.js).
//
//   const ed = DocEditor(document.querySelector("[data-doc-editor]"), { mode, onChange });
//   ed.page    the contenteditable element
//   ed.refresh()  re-number and rebuild the outline after changing the page
window.DocEditor = function (root, opts) {
  opts = opts || {};
  const mode = opts.mode || "doc";
  const rulesMode = mode === "rules";
  const page = root.querySelector("[data-doc-page]");
  const toolbar = root.querySelector("[data-doc-toolbar]");
  const outline = root.querySelector("[data-doc-outline]");
  const words = root.querySelector("[data-doc-words]");
  const onChange = opts.onChange || function () {};

  try { document.execCommand("defaultParagraphSeparator", false, "p"); } catch (_) { /* old browsers */ }

  // ---- Styles -------------------------------------------------------------
  const STYLES = rulesMode ? [
    { key: "p", label: "Normal text", hint: "Introductions" },
    { key: "h2", label: "Section", hint: "1. General rules" },
    { key: "h3", label: "Subsection", hint: "1.4 Chain of command" },
  ] : [
    { key: "p", label: "Normal text" },
    { key: "h1", label: "Title" },
    { key: "p.doc-subtitle", label: "Subtitle" },
    { key: "h2", label: "Heading 1" },
    { key: "h3", label: "Heading 2" },
    { key: "h4", label: "Heading 3" },
    { key: "blockquote", label: "Quote" },
  ];
  const styleLabel = (key) => (STYLES.find((s) => s.key === key) || STYLES[0]).label;

  const svg = (d) => '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + d + "</svg>";
  const I = {
    undo: svg('<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>'),
    redo: svg('<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>'),
    print: svg('<path d="M6 9V2h12v7"/><path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/><path d="M6 14h12v8H6z"/>'),
    bold: svg('<path d="M6 4h8a4 4 0 0 1 0 8H6z"/><path d="M6 12h9a4 4 0 0 1 0 8H6z"/>'),
    italic: svg('<path d="M19 4h-9M14 20H5M15 4 9 20"/>'),
    underline: svg('<path d="M6 4v6a6 6 0 0 0 12 0V4"/><path d="M4 20h16"/>'),
    strike: svg('<path d="M16 4H9a3 3 0 0 0-2.83 4"/><path d="M14 12a4 4 0 0 1 0 8H6"/><path d="M4 12h16"/>'),
    color: svg('<path d="m6 16 6-12 6 12"/><path d="M8 12h8"/>'),
    highlight: svg('<path d="m9 11-6 6v3h9l3-3"/><path d="m22 12-4.6 4.6a2 2 0 0 1-2.8 0l-5.2-5.2a2 2 0 0 1 0-2.8L14 4"/>'),
    link: svg('<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>'),
    table: svg('<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18M15 3v18"/>'),
    hr: svg('<path d="M3 12h18"/>'),
    alignLeft: svg('<path d="M21 6H3M15 12H3M17 18H3"/>'),
    alignCenter: svg('<path d="M21 6H3M17 12H7M19 18H5"/>'),
    alignRight: svg('<path d="M21 6H3M21 12H9M21 18H7"/>'),
    alignJustify: svg('<path d="M21 6H3M21 12H3M21 18H3"/>'),
    ul: svg('<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4" cy="6" r="1"/><circle cx="4" cy="12" r="1"/><circle cx="4" cy="18" r="1"/>'),
    ol: svg('<path d="M10 6h11M10 12h11M10 18h11"/><path d="M4 6h1v4M4 10h2"/><path d="M6 18H4c0-1 2-2 2-3s-1-1.5-2-1"/>'),
    outdent: svg('<path d="M21 6H11M21 12H11M21 18H11"/><path d="m7 8-4 4 4 4"/>'),
    indent: svg('<path d="M21 6H11M21 12H11M21 18H11"/><path d="m3 8 4 4-4 4"/>'),
    clear: svg('<path d="M4 7V4h16v3"/><path d="M5 20h6"/><path d="M13 4 8 20"/><path d="m15 15 5 5M20 15l-5 5"/>'),
    caret: '<svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M7 10l5 5 5-5z"/></svg>',
  };

  const btn = (cmd, icon, label, extra) =>
    '<button type="button" class="gd-btn' + (extra ? " " + extra : "") + '" data-cmd="' + cmd + '" title="' + label + '" aria-label="' + label + '">' + icon + "</button>";
  const sep = '<span class="gd-sep" aria-hidden="true"></span>';
  const COLORS = ["#dbe3ef", "#9ca3af", "#f87171", "#fb923c", "#fbbf24", "#4ade80", "#2dd4bf", "#60a5fa", "#a78bfa", "#f472b6"];
  const HIGHLIGHTS = ["transparent", "#7f1d1d", "#7c2d12", "#78350f", "#14532d", "#134e4a", "#1e3a8a", "#4c1d95", "#831843"];
  const swatches = (cmd, list) => list.map((c) =>
    '<button type="button" class="gd-swatch" data-cmd="' + cmd + '" data-value="' + c + '" style="background:' + (c === "transparent" ? "none" : c) + '" title="' + (c === "transparent" ? "None" : c) + '" aria-label="' + (c === "transparent" ? "No highlight" : "Colour " + c) + '">' + (c === "transparent" ? "&#8856;" : "") + "</button>").join("");

  let html =
    btn("undo", I.undo, "Undo (Ctrl+Z)") + btn("redo", I.redo, "Redo (Ctrl+Y)") + btn("print", I.print, "Print (Ctrl+P)") + sep +
    '<label class="visually-hidden" for="gd-zoom">Zoom</label><select id="gd-zoom" class="gd-zoom" data-zoom title="Zoom"><option value="0.75">75%</option><option value="0.9">90%</option><option value="1" selected>100%</option><option value="1.25">125%</option><option value="1.5">150%</option></select>' + sep +
    '<div class="gd-menu-wrap"><button type="button" class="gd-style" data-menu="style" aria-haspopup="true" aria-expanded="false" title="Styles"><span data-style-label>Normal text</span>' + I.caret + '</button>' +
    '<div class="gd-menu gd-style-menu" data-menu-for="style" role="menu" hidden>' +
    STYLES.map((s) => '<button type="button" role="menuitem" class="gd-style-item gd-sty-' + s.key.replace(".", "-") + '" data-cmd="style" data-value="' + s.key + '"><span>' + s.label + "</span>" + (s.hint ? '<small>' + s.hint + "</small>" : "") + "</button>").join("") +
    "</div></div>" + sep;
  if (rulesMode) {
    html += btn("insertOrderedList", I.ol, "Numbered rule (Ctrl+Shift+7)", "gd-wide") .replace("</button>", " <span>Rule</span></button>") +
      btn("dotPoint", I.ul, "Dot point under a rule (Tab)", "gd-wide").replace("</button>", " <span>Dot point</span></button>") + sep +
      btn("outdent", I.outdent, "Decrease indent (Shift+Tab)") + btn("indent", I.indent, "Increase indent (Tab)") + sep +
      btn("removeFormat", I.clear, "Clear formatting");
  } else {
    html += btn("bold", I.bold, "Bold (Ctrl+B)") + btn("italic", I.italic, "Italic (Ctrl+I)") + btn("underline", I.underline, "Underline (Ctrl+U)") + btn("strikeThrough", I.strike, "Strikethrough") +
      '<div class="gd-menu-wrap"><button type="button" class="gd-btn" data-menu="color" title="Text colour" aria-label="Text colour" aria-haspopup="true">' + I.color + '<span class="gd-colorbar" data-colorbar></span></button><div class="gd-menu gd-swatches" data-menu-for="color" hidden>' + swatches("foreColor", COLORS) + "</div></div>" +
      '<div class="gd-menu-wrap"><button type="button" class="gd-btn" data-menu="hl" title="Highlight colour" aria-label="Highlight colour" aria-haspopup="true">' + I.highlight + '</button><div class="gd-menu gd-swatches" data-menu-for="hl" hidden>' + swatches("hiliteColor", HIGHLIGHTS) + "</div></div>" + sep +
      btn("createLink", I.link, "Insert link (Ctrl+K)") + btn("insertTable", I.table, "Insert table") + btn("insertHorizontalRule", I.hr, "Horizontal line") + sep +
      '<div class="gd-menu-wrap"><button type="button" class="gd-btn" data-menu="align" title="Align" aria-label="Align" aria-haspopup="true">' + I.alignLeft + I.caret + '</button><div class="gd-menu gd-row" data-menu-for="align" hidden>' +
      btn("justifyLeft", I.alignLeft, "Left align") + btn("justifyCenter", I.alignCenter, "Centre") + btn("justifyRight", I.alignRight, "Right align") + btn("justifyFull", I.alignJustify, "Justify") + "</div></div>" + sep +
      btn("insertUnorderedList", I.ul, "Bulleted list (Ctrl+Shift+8)") + btn("insertOrderedList", I.ol, "Numbered list (Ctrl+Shift+7)") +
      btn("outdent", I.outdent, "Decrease indent") + btn("indent", I.indent, "Increase indent") + sep +
      btn("removeFormat", I.clear, "Clear formatting (Ctrl+\\)");
  }
  toolbar.innerHTML = html;
  toolbar.setAttribute("role", "toolbar");
  toolbar.setAttribute("aria-label", "Formatting");

  // ---- Helpers ------------------------------------------------------------
  let savedRange = null;
  const saveSel = () => {
    const sel = window.getSelection();
    if (sel.rangeCount && page.contains(sel.anchorNode)) savedRange = sel.getRangeAt(0).cloneRange();
  };
  const restoreSel = () => {
    page.focus({ preventScroll: true });
    if (savedRange) {
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(savedRange);
    }
  };
  const blockAt = () => {
    const sel = window.getSelection();
    if (!sel.rangeCount) return null;
    let n = sel.anchorNode;
    while (n && n !== page) {
      if (n.nodeType === 1 && /^(P|H[1-6]|BLOCKQUOTE|LI|DIV|PRE)$/.test(n.tagName)) return n;
      n = n.parentNode;
    }
    return null;
  };
  const blockKey = (b) => {
    if (!b) return "p";
    if (b.tagName === "P" && b.classList.contains("doc-subtitle")) return "p.doc-subtitle";
    const t = b.tagName.toLowerCase();
    return t === "div" || t === "li" ? "p" : t;
  };
  const closeMenus = () => toolbar.querySelectorAll("[data-menu-for]").forEach((m) => {
    m.hidden = true;
    const b = toolbar.querySelector('[data-menu="' + m.dataset.menuFor + '"]');
    if (b) b.setAttribute("aria-expanded", "false");
  });

  const setStyle = (key) => {
    const [tag, cls] = key.split(".");
    document.execCommand("formatBlock", false, "<" + tag + ">");
    const b = blockAt();
    if (b && b.tagName.toLowerCase() === tag) {
      b.removeAttribute("class");
      if (cls) b.classList.add(cls);
    }
  };

  const run = (cmd, value) => {
    restoreSel();
    switch (cmd) {
      case "style": setStyle(value); break;
      case "print": window.print(); return;
      case "createLink": {
        const url = prompt("Link address (https://…)", "https://");
        if (url && /^https?:\/\//i.test(url)) document.execCommand("createLink", false, url);
        break;
      }
      case "insertTable": {
        const rows = Math.min(Math.max(parseInt(prompt("How many rows?", "3"), 10) || 0, 1), 30);
        const cols = Math.min(Math.max(parseInt(prompt("How many columns?", "3"), 10) || 0, 1), 8);
        let t = "<table><thead><tr>";
        for (let c = 0; c < cols; c++) t += "<th>Heading</th>";
        t += "</tr></thead><tbody>";
        for (let r = 0; r < rows - 1; r++) {
          t += "<tr>";
          for (let c = 0; c < cols; c++) t += "<td>&nbsp;</td>";
          t += "</tr>";
        }
        document.execCommand("insertHTML", false, t + "</tbody></table><p><br></p>");
        break;
      }
      case "foreColor":
      case "hiliteColor":
        // Colours as inline CSS (span style), which the sanitiser keeps.
        document.execCommand("styleWithCSS", false, true);
        document.execCommand(cmd, false, value);
        document.execCommand("styleWithCSS", false, false);
        if (cmd === "foreColor") toolbar.querySelector("[data-colorbar]").style.background = value;
        break;
      case "dotPoint": {
        // Inside a rule, a dot point is the next level down; elsewhere it
        // starts a bulleted list (which becomes points of the rule above).
        const li = blockAt() && blockAt().closest("li");
        if (li) document.execCommand("indent", false, null);
        else document.execCommand("insertUnorderedList", false, null);
        break;
      }
      default:
        document.execCommand(cmd, false, null);
    }
    closeMenus();
    changed();
  };

  toolbar.addEventListener("mousedown", (e) => {
    // Keep the page's selection when clicking the toolbar.
    if (e.target.closest("button")) e.preventDefault();
  });
  toolbar.addEventListener("click", (e) => {
    const m = e.target.closest("[data-menu]");
    if (m) {
      const menu = toolbar.querySelector('[data-menu-for="' + m.dataset.menu + '"]');
      const open = menu.hidden;
      closeMenus();
      menu.hidden = !open;
      m.setAttribute("aria-expanded", open ? "true" : "false");
      return;
    }
    const b = e.target.closest("[data-cmd]");
    if (b && b.tagName === "BUTTON") run(b.dataset.cmd, b.dataset.value);
  });
  document.addEventListener("click", (e) => { if (!toolbar.contains(e.target)) closeMenus(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeMenus(); });

  const zoom = toolbar.querySelector("[data-zoom]");
  zoom.addEventListener("change", () => { page.style.zoom = zoom.value; });

  // Show the styles that apply at the cursor.
  const syncState = () => {
    saveSel();
    toolbar.querySelectorAll("button[data-cmd]").forEach((b) => {
      if (!/^(bold|italic|underline|strikeThrough|insertUnorderedList|insertOrderedList|justify\w+)$/.test(b.dataset.cmd)) return;
      let on = false;
      try { on = document.queryCommandState(b.dataset.cmd); } catch (_) { /* not a state command */ }
      b.classList.toggle("on", on);
      b.setAttribute("aria-pressed", on ? "true" : "false");
    });
    const key = blockKey(blockAt());
    toolbar.querySelector("[data-style-label]").textContent = styleLabel(key);
    toolbar.querySelectorAll(".gd-style-item").forEach((i) => i.classList.toggle("on", i.dataset.value === key));
  };
  document.addEventListener("selectionchange", () => { if (page.contains(document.getSelection().anchorNode)) syncState(); });

  // ---- Outline, numbering and word count -----------------------------------
  const plain = (el) => {
    const c = el.cloneNode(true);
    c.querySelectorAll("ul,ol").forEach((l) => l.remove());
    return c.textContent.replace(/\s+/g, " ").trim();
  };

  // Rules: number sections (1.), rules and subsections (1.1), and rules in
  // subsections (1.4.1), the way the public page will.
  const numberRules = () => {
    page.querySelectorAll("[data-num]").forEach((el) => {
      if (el.parentNode !== page && !(el.tagName === "LI" && el.parentNode.parentNode === page)) delete el.dataset.num;
    });
    let s = 0, k = 0, sub = null, r = 0;
    for (const el of page.children) {
      const t = el.tagName;
      if (t === "H1" || t === "H2") { s++; k = 0; sub = null; el.dataset.num = s + "."; }
      else if (t === "H3" || t === "H4") { if (!s) { el.dataset.num = "?"; continue; } k++; sub = s + "." + k; r = 0; el.dataset.num = sub; }
      else if ((t === "OL" || t === "UL") && s) {
        for (const li of el.children) {
          if (li.tagName !== "LI") continue;
          if (sub) { r++; li.dataset.num = sub + "." + r; } else { k++; li.dataset.num = s + "." + k; }
        }
      } else if (t === "P" && s && plain(el) && (sub ? r : k)) {
        // A paragraph after rules is saved as the next rule.
        if (sub) { r++; el.dataset.num = sub + "." + r; } else { k++; el.dataset.num = s + "." + k; }
        el.classList.add("gd-rule-p");
        continue;
      }
      el.classList && el.classList.remove("gd-rule-p");
    }
  };

  const buildOutline = () => {
    if (!outline) return;
    const sel = rulesMode ? "h1,h2,h3,h4" : "h1,h2,h3,h4";
    const heads = [...page.querySelectorAll(sel)].filter((h) => plain(h));
    const list = outline.querySelector("[data-outline-list]");
    list.textContent = "";
    if (!heads.length) {
      const p = document.createElement("p");
      p.className = "gd-outline-empty";
      p.textContent = rulesMode ? "Sections (Section style) and subsections appear here." : "Headings you add to the document appear here.";
      list.appendChild(p);
      return;
    }
    heads.forEach((h) => {
      const a = document.createElement("button");
      a.type = "button";
      const lvl = rulesMode ? (h.tagName === "H3" || h.tagName === "H4" ? 2 : 1) : Math.max(1, parseInt(h.tagName[1], 10) - (h.tagName === "H1" ? 0 : 1));
      a.className = "gd-outline-item gd-ol-" + lvl + (h.tagName === "H1" && !rulesMode ? " gd-ol-title" : "");
      a.textContent = (rulesMode && h.dataset.num ? h.dataset.num + " " : "") + plain(h);
      a.addEventListener("click", () => {
        h.scrollIntoView({ behavior: "smooth", block: "center" });
        const r = document.createRange();
        r.selectNodeContents(h);
        r.collapse(false);
        const s = window.getSelection();
        s.removeAllRanges();
        s.addRange(r);
        page.focus({ preventScroll: true });
      });
      list.appendChild(a);
    });
  };

  const countWords = () => {
    if (!words) return;
    const n = (page.innerText.match(/\S+/g) || []).length;
    words.textContent = n + (n === 1 ? " word" : " words");
  };

  let timer = 0;
  const refresh = () => {
    if (rulesMode) numberRules();
    buildOutline();
    countWords();
  };
  const changed = () => {
    clearTimeout(timer);
    timer = setTimeout(refresh, 150);
    onChange();
  };
  page.addEventListener("input", changed);

  // ---- Keyboard ------------------------------------------------------------
  page.addEventListener("keydown", (e) => {
    const mod = e.ctrlKey || e.metaKey;
    if (e.key === "Tab") {
      const b = blockAt();
      if (b && b.closest("li")) {
        e.preventDefault();
        document.execCommand(e.shiftKey ? "outdent" : "indent", false, null);
        changed();
      }
      return;
    }
    if (!mod) return;
    const k = e.key.toLowerCase();
    if (e.altKey && /^[0-4]$/.test(k)) {
      e.preventDefault();
      const map = rulesMode ? { "0": "p", "1": "h2", "2": "h3" } : { "0": "p", "1": "h2", "2": "h3", "3": "h4" };
      if (map[k]) { setStyle(map[k]); changed(); }
    } else if (e.shiftKey && (e.code === "Digit7" || e.code === "Digit8")) {
      e.preventDefault();
      document.execCommand(e.code === "Digit7" ? "insertOrderedList" : "insertUnorderedList", false, null);
      changed();
    } else if (k === "k" && !rulesMode) {
      e.preventDefault();
      run("createLink");
    } else if (k === "\\") {
      e.preventDefault();
      document.execCommand("removeFormat", false, null);
      changed();
    } else if (rulesMode && /^[biu]$/.test(k)) {
      e.preventDefault(); // the rules are plain text
    }
  });

  // Paste without the source's styling (rules: plain text only).
  page.addEventListener("paste", (e) => {
    const htmlData = e.clipboardData.getData("text/html");
    const text = e.clipboardData.getData("text/plain");
    if (!htmlData && !text) return;
    e.preventDefault();
    if (htmlData && !rulesMode) {
      const tmp = document.createElement("div");
      tmp.innerHTML = htmlData;
      tmp.querySelectorAll("*").forEach((el) => { el.removeAttribute("style"); el.removeAttribute("class"); });
      tmp.querySelectorAll("script,style,meta,link,iframe,object,img").forEach((el) => el.remove());
      document.execCommand("insertHTML", false, tmp.innerHTML);
    } else {
      document.execCommand("insertText", false, text);
    }
    changed();
  });

  refresh();
  return { page, refresh };
};

// The drive's document editor page.
(() => {
  const form = document.querySelector("[data-editor-form]");
  if (!form) return;
  const body = form.querySelector("[data-editor-body]");
  const status = form.querySelector("[data-editor-status]");
  let dirty = false;
  const changed = () => {
    dirty = true;
    if (status) status.textContent = "Unsaved changes";
  };
  const ed = window.DocEditor(form.querySelector("[data-doc-editor]"), { mode: "doc", onChange: changed });
  form.querySelectorAll("input,select").forEach((el) => {
    if (!el.closest("[data-doc-toolbar]")) el.addEventListener("change", changed);
  });
  form.addEventListener("submit", () => {
    body.value = ed.page.innerHTML;
    dirty = false;
    if (status) status.textContent = "Saving…";
  });
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      form.requestSubmit();
    }
  });
  window.addEventListener("beforeunload", (e) => {
    if (dirty) { e.preventDefault(); e.returnValue = ""; }
  });
})();
