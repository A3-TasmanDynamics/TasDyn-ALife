// Faster triage in the admin panel:
//  - Bulk bars: <form data-bulk> with checkboxes (form="<id>") in the list.
//    The bar appears once something is ticked, counts the selection, has
//    "select all" and "clear", and returns you to the same filtered list.
//  - Keyboard: j / k move through the list, Enter opens, x ticks the row,
//    g then a letter jumps to a page, ? lists everything. Page actions are
//    marked data-key="n" (focus a field) / data-key="c" (follow a link).
//  - Saved views: on a filter bar, "Save view" keeps the current filters
//    under a name (in this browser) and shows them as one-click chips.
(function () {
  "use strict";

  // ---------- bulk bars ----------
  document.querySelectorAll("form[data-bulk]").forEach(function (form) {
    var boxes = function () { return Array.prototype.slice.call(document.querySelectorAll('input.bulk-check[form="' + form.id + '"]')); };
    var all = form.querySelector("[data-bulk-all]");
    var count = form.querySelector("[data-bulk-count]");
    function update() {
      var b = boxes(), n = b.filter(function (x) { return x.checked; }).length;
      form.hidden = n === 0;
      if (count) count.textContent = n;
      if (all) { all.checked = n > 0 && n === b.length; all.indeterminate = n > 0 && n < b.length; }
      document.body.classList.toggle("bulk-active", n > 0);
    }
    document.addEventListener("change", function (e) {
      if (e.target.matches('input.bulk-check[form="' + form.id + '"]')) update();
    });
    if (all) all.addEventListener("change", function () { boxes().forEach(function (x) { x.checked = all.checked; }); update(); });
    var clear = form.querySelector("[data-bulk-clear]");
    if (clear) clear.addEventListener("click", function () { boxes().forEach(function (x) { x.checked = false; }); update(); });
    form.addEventListener("submit", function () {
      var back = form.querySelector('input[name="back"]');
      if (back) back.value = location.pathname + location.search;
    });
    update();
  });

  // ---------- keyboard ----------
  function typing(el) {
    return el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName));
  }
  function rows() {
    var root = document.querySelector(".admin-content") || document;
    return Array.prototype.slice.call(root.querySelectorAll("[data-row], a.lrow")).filter(function (el) {
      return el.offsetParent !== null;
    });
  }
  function move(dir) {
    var r = rows();
    if (!r.length) return;
    var i = r.indexOf(document.activeElement);
    if (i < 0) i = r.findIndex(function (el) { return el.classList.contains("active"); });
    i = i < 0 ? (dir > 0 ? 0 : r.length - 1) : Math.max(0, Math.min(r.length - 1, i + dir));
    r[i].focus();
    r[i].scrollIntoView({ block: "nearest" });
  }
  function rowBox(el) {
    var row = el && (el.closest(".ac-row, tr, .lrow") || el);
    return row && row.querySelector("input.bulk-check");
  }
  var pages = { d: "/admin", o: "/admin/online", c: "/admin/cases", f: "/admin/anticheat", b: "/admin/bans", w: "/admin/watchlist",
    p: "/admin/players", e: "/admin/events", v: "/admin/vehicles", t: "/admin/tickets", s: "/admin/staff-log", m: "/admin/economy" };
  var gPending = 0;

  function help() {
    var d = document.getElementById("kbd-help");
    if (!d) {
      d = document.createElement("dialog");
      d.id = "kbd-help";
      d.className = "modal kbd-help";
      d.setAttribute("aria-labelledby", "kbd-help-title");
      var rows = [["j / k", "Next / previous item in the list"], ["Enter", "Open it"], ["x", "Tick it for a bulk action"],
        ["/ or Ctrl+K", "Quick search"], ["n", "Note field (flag review)"], ["c", "Open a case (flag review)"], ["?", "This list"]];
      var go = [["d", "Dashboard"], ["o", "Online Now"], ["c", "Cases"], ["f", "Anti-Cheat Flags"], ["b", "Bans"], ["w", "Watchlist"],
        ["p", "Player Lookup"], ["e", "Event Log"], ["v", "Vehicles"], ["t", "Tickets"], ["s", "Staff Log"], ["m", "Economy"]];
      var html = '<div class="modal-head"><h2 id="kbd-help-title">Keyboard shortcuts</h2><button class="modal-x" type="button" data-dialog-close aria-label="Close">&times;</button></div><div class="modal-body kbd-grid"><div><h3>On a page</h3>';
      rows.forEach(function (r) { html += "<p><kbd>" + r[0] + "</kbd><span>" + r[1] + "</span></p>"; });
      html += "</div><div><h3>Go to (press g, then)</h3>";
      go.forEach(function (r) { html += "<p><kbd>g " + r[0] + "</kbd><span>" + r[1] + "</span></p>"; });
      html += "</div></div>";
      d.innerHTML = html;
      document.body.appendChild(d);
    }
    d.showModal();
  }

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-kbd-help]")) help();
  });

  document.addEventListener("keydown", function (e) {
    if (e.ctrlKey || e.metaKey || e.altKey || typing(document.activeElement) || document.querySelector("dialog[open]")) return;
    var k = e.key;
    if (gPending && Date.now() - gPending < 1500) {
      gPending = 0;
      if (pages[k]) { e.preventDefault(); location.href = pages[k]; }
      return;
    }
    if (k === "g") { gPending = Date.now(); return; }
    if (k === "?") { e.preventDefault(); help(); return; }
    if (k === "j" || k === "k") { e.preventDefault(); move(k === "j" ? 1 : -1); return; }
    if (k === "x") {
      var box = rowBox(document.activeElement);
      if (box) { e.preventDefault(); box.checked = !box.checked; box.dispatchEvent(new Event("change", { bubbles: true })); }
      return;
    }
    var target = document.querySelector('[data-key="' + CSS.escape(k) + '"]');
    if (target && target.offsetParent !== null) {
      e.preventDefault();
      if (typing(target)) target.focus();
      else target.click();
    }
  });

  // ---------- saved views ----------
  var form = document.querySelector(".admin-content form.filterbar[method='get'], .admin-content form.filter-bar[method='get']");
  if (form) {
    var key = "alife.views:" + location.pathname;
    // The filters only: not the toast message, the open item or the page.
    var current = (function () {
      var p = new URLSearchParams(location.search);
      ["notice", "error", "flag", "entry", "id", "page", "before", "after"].forEach(function (k) { p.delete(k); });
      var q = p.toString();
      return q ? "?" + q : "";
    })();
    var load = function () { try { return JSON.parse(localStorage.getItem(key) || "[]") || []; } catch (e) { return []; } };
    var save = function (v) { try { localStorage.setItem(key, JSON.stringify(v)); } catch (e) { /* not remembered */ } };
    var bar = document.createElement("div");
    bar.className = "saved-views";
    bar.setAttribute("aria-label", "Saved views");
    form.parentNode.insertBefore(bar, form);
    var draw = function () {
      var views = load();
      bar.textContent = "";
      views.forEach(function (v, i) {
        var chip = document.createElement("span");
        chip.className = "view-chip" + (v.q === current ? " active" : "");
        var a = document.createElement("a");
        a.href = location.pathname + v.q;
        a.textContent = v.name;
        var x = document.createElement("button");
        x.type = "button";
        x.setAttribute("aria-label", "Forget the view " + v.name);
        x.textContent = "×";
        x.addEventListener("click", function () { var all = load(); all.splice(i, 1); save(all); draw(); });
        chip.appendChild(a); chip.appendChild(x); bar.appendChild(chip);
      });
      if (current && !views.some(function (v) { return v.q === current; })) {
        var add = document.createElement("button");
        add.type = "button";
        add.className = "view-save";
        add.textContent = "+ Save this view";
        add.addEventListener("click", function () {
          var name = (window.prompt("Name this view (it stays in this browser):", "") || "").trim().slice(0, 40);
          if (!name) return;
          var all = load();
          all.push({ name: name, q: current });
          save(all.slice(-8));
          draw();
        });
        bar.appendChild(add);
      }
      bar.hidden = !bar.childNodes.length;
    };
    draw();
  }
})();
