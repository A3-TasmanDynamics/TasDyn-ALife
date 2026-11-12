// Quick search (Ctrl+K / Cmd+K, or "/" when not typing): one box for pages,
// players, vehicles, gangs, cases and flags. Results come from
// /admin/search?q= and are only what the viewer may see. The sidebar's
// "Search players" box opens it too; without JavaScript that box still
// searches players.
(function () {
  "use strict";
  var dlg, input, list, status, items = [], active = -1, seq = 0, timer = 0, ctrl = null;

  function build() {
    dlg = document.createElement("dialog");
    dlg.className = "qs";
    dlg.setAttribute("aria-label", "Quick search");
    dlg.innerHTML =
      '<div class="qs-box">' +
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/></svg>' +
      '<input class="qs-input" type="search" autocomplete="off" spellcheck="false" placeholder="Search players, plates, gangs, cases, #flags, pages…" role="combobox" aria-expanded="true" aria-controls="qs-list" aria-autocomplete="list">' +
      '<kbd class="qs-kbd">Esc</kbd></div>' +
      '<div class="qs-list" id="qs-list" role="listbox"></div>' +
      '<div class="qs-foot"><span><kbd>↑</kbd><kbd>↓</kbd> move</span><span><kbd>Enter</kbd> open</span><span><kbd>Ctrl</kbd>+<kbd>Enter</kbd> new tab</span><span class="qs-status" aria-live="polite"></span></div>';
    document.body.appendChild(dlg);
    input = dlg.querySelector(".qs-input");
    list = dlg.querySelector(".qs-list");
    status = dlg.querySelector(".qs-status");
    input.addEventListener("input", function () {
      clearTimeout(timer);
      timer = setTimeout(run, 140);
    });
    input.addEventListener("keydown", keys);
    dlg.addEventListener("click", function (e) {
      if (e.target === dlg) dlg.close(); // the backdrop
    });
    list.addEventListener("mousemove", function (e) {
      var a = e.target.closest(".qs-item");
      if (a) select(items.indexOf(a), false);
    });
  }

  function open(text) {
    if (!dlg) build();
    if (!dlg.open) dlg.showModal();
    if (typeof text === "string" && text) input.value = text;
    input.focus();
    input.select();
    run();
  }

  function keys(e) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (items.length) select((active + (e.key === "ArrowDown" ? 1 : -1) + items.length) % items.length, true);
    } else if (e.key === "Enter") {
      e.preventDefault();
      var a = items[active] || items[0];
      if (!a) return;
      if (e.ctrlKey || e.metaKey) window.open(a.href, "_blank", "noopener");
      else location.href = a.href;
    }
  }

  function select(i, scroll) {
    if (items[active]) items[active].removeAttribute("aria-selected");
    active = i;
    if (items[i]) {
      items[i].setAttribute("aria-selected", "true");
      input.setAttribute("aria-activedescendant", items[i].id);
      if (scroll) items[i].scrollIntoView({ block: "nearest" });
    }
  }

  function run() {
    var q = input.value.trim(), mine = ++seq;
    if (ctrl) ctrl.abort();
    if (!q) {
      show([], "");
      status.textContent = "Type a name, Steam ID, plate, gang, case or #number";
      return;
    }
    ctrl = window.AbortController ? new AbortController() : null;
    status.textContent = "Searching…";
    fetch("/admin/search?q=" + encodeURIComponent(q), { credentials: "same-origin", signal: ctrl ? ctrl.signal : undefined })
      .then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.json();
      })
      .then(function (d) {
        if (mine === seq) show(d.groups || [], q);
      })
      .catch(function (err) {
        if (mine === seq && err.name !== "AbortError") status.textContent = "Search failed. Try again.";
      });
  }

  function show(groups, q) {
    list.textContent = "";
    items = [];
    active = -1;
    var n = 0;
    groups.forEach(function (g) {
      var h = document.createElement("div");
      h.className = "qs-group";
      h.textContent = g.name;
      list.appendChild(h);
      g.hits.forEach(function (hit) {
        var a = document.createElement("a");
        a.className = "qs-item";
        a.href = hit.url;
        a.id = "qs-item-" + n++;
        a.setAttribute("role", "option");
        var t = document.createElement("span");
        t.className = "qs-title";
        t.textContent = hit.title;
        a.appendChild(t);
        if (hit.sub) {
          var s = document.createElement("span");
          s.className = "qs-sub";
          s.textContent = hit.sub;
          a.appendChild(s);
        }
        list.appendChild(a);
        items.push(a);
      });
    });
    if (q && !items.length) {
      var e = document.createElement("p");
      e.className = "qs-empty";
      e.textContent = 'Nothing matches "' + q + '".';
      list.appendChild(e);
    }
    status.textContent = q ? items.length + " result" + (items.length === 1 ? "" : "s") : "";
    if (items.length) select(0, false);
  }

  function typing(el) {
    return el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName));
  }

  document.addEventListener("keydown", function (e) {
    if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && (e.key === "k" || e.key === "K")) {
      e.preventDefault();
      if (dlg && dlg.open) dlg.close();
      else open();
    } else if (e.key === "/" && !e.ctrlKey && !e.metaKey && !e.altKey && !typing(document.activeElement) && !document.querySelector("dialog[open]")) {
      e.preventDefault();
      open();
    }
  });

  // The sidebar box opens the palette with whatever was typed.
  var side = document.querySelector(".admin-sidebar-search input");
  if (side) {
    var hint = document.createElement("kbd");
    hint.className = "qs-hint";
    hint.textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K";
    side.parentNode.appendChild(hint);
    side.setAttribute("placeholder", "Search");
    side.addEventListener("focus", function () {
      var v = side.value;
      side.value = "";
      side.blur();
      open(v);
    });
  }
})();
