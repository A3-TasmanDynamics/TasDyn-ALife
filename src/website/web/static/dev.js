// Admin → Development: the live website log and the project board's
// drag-and-drop. Dialogs use admin.js's data-dialog-open / -close.
(function () {
  // A task opened by URL (?task=ID) shows its dialog straight away.
  var auto = document.querySelector("dialog[data-open-on-load]");
  if (auto && typeof auto.showModal === "function") {
    auto.showModal();
    auto.addEventListener("cancel", function () { location.href = "/admin/dev/board"; });
  }

  // ---- Website logs ----
  var logs = document.querySelector("[data-dev-logs]");
  if (logs) {
    var view = logs.querySelector("[data-view]");
    var q = logs.querySelector("[data-q]");
    var pauseBtn = logs.querySelector("[data-pause]");
    var countEl = logs.querySelector("[data-count]");
    var entries = [], after = 0, level = "all", paused = false;

    function show(e) {
      if (level === "app" && e.level === "http") return false;
      if (level === "warn" && e.level !== "warn" && e.level !== "error") return false;
      if (level === "error" && e.level !== "error") return false;
      var needle = q.value.trim().toLowerCase();
      return !needle || (e.msg + " " + (e.attrs || "")).toLowerCase().indexOf(needle) !== -1;
    }
    function time(t) {
      var d = new Date(t);
      return d.toLocaleDateString(undefined, { day: "numeric", month: "short" }) + " " + d.toLocaleTimeString(undefined, { hour12: false });
    }
    function render() {
      var stick = view.scrollTop + view.clientHeight >= view.scrollHeight - 30;
      var shown = entries.filter(show);
      view.textContent = "";
      if (!shown.length) {
        var p = document.createElement("p");
        p.className = "dev-muted";
        p.textContent = entries.length ? "No lines match." : "Nothing logged yet.";
        view.appendChild(p);
      }
      shown.slice(-1000).forEach(function (e) {
        var row = document.createElement("div");
        row.className = "dev-log-line dev-log-" + e.level;
        var t = document.createElement("span"); t.className = "dev-log-time"; t.textContent = time(e.time);
        var l = document.createElement("span"); l.className = "dev-log-level"; l.textContent = e.level;
        var m = document.createElement("span"); m.className = "dev-log-msg"; m.textContent = e.msg;
        row.appendChild(t); row.appendChild(l); row.appendChild(m);
        if (e.attrs) { var a = document.createElement("span"); a.className = "dev-log-attrs"; a.textContent = e.attrs; row.appendChild(a); }
        view.appendChild(row);
      });
      countEl.textContent = shown.length + " of " + entries.length + " lines";
      if (stick) view.scrollTop = view.scrollHeight;
    }
    function poll() {
      if (paused) return;
      fetch("/admin/dev/logs.json?after=" + after, { headers: { Accept: "application/json" } })
        .then(function (r) { return r.ok ? r.json() : []; })
        .then(function (list) {
          if (!list || !list.length) { if (!entries.length) render(); return; }
          entries = entries.concat(list).slice(-2000);
          after = list[list.length - 1].id;
          render();
        })
        .catch(function () {});
    }
    logs.querySelectorAll("[data-level]").forEach(function (b) {
      b.addEventListener("click", function () {
        level = b.getAttribute("data-level");
        logs.querySelectorAll("[data-level]").forEach(function (x) { x.classList.toggle("active", x === b); });
        render();
      });
    });
    q.addEventListener("input", render);
    pauseBtn.addEventListener("click", function () {
      paused = !paused;
      pauseBtn.textContent = paused ? "Resume" : "Pause";
      if (!paused) poll();
    });
    poll();
    setInterval(poll, 3000);
  }

  // ---- Card checklists: tick items without reloading ----
  document.querySelectorAll("form[data-toggle]").forEach(function (form) {
    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var btn = form.querySelector(".dev-check");
      var doneInput = form.querySelector("input[name=done]");
      var li = form.closest(".dev-item");
      var list = form.closest("[data-checklist]");
      var nowDone = doneInput.value === "1";
      fetch(form.action, { method: "POST", headers: { Accept: "application/json" }, body: new URLSearchParams(new FormData(form)) })
        .then(function (r) {
          if (!r.ok) { form.submit(); return; }
          btn.classList.toggle("on", nowDone);
          btn.setAttribute("aria-checked", nowDone ? "true" : "false");
          li.classList.toggle("done", nowDone);
          doneInput.value = nowDone ? "0" : "1";
          var items = list.querySelectorAll(".dev-item");
          var done = list.querySelectorAll(".dev-item.done").length;
          var pct = items.length ? Math.floor(done * 100 / items.length) : 0;
          list.querySelector("[data-pct]").textContent = pct + "%";
          list.querySelector("[data-bar]").style.width = pct + "%";
          list.classList.toggle("complete", items.length > 0 && done === items.length);
        })
        .catch(function () { form.submit(); });
    });
  });

  // ---- Card dialog: description edit, click-to-reveal forms, feed filter ----
  var desc = document.querySelector("[data-desc]");
  if (desc) {
    var view = desc.querySelector("[data-desc-view]");
    var empty = desc.querySelector(".dev-desc-empty");
    var formBox = desc.querySelector("[data-desc-form]");
    var area = formBox.querySelector("textarea");
    var original = area.value;
    var editBtns = desc.querySelectorAll("[data-desc-edit]");
    function editing(on) {
      formBox.hidden = !on;
      editBtns.forEach(function (b) { b.hidden = on || (b === empty ? original !== "" : original === ""); });
      if (view) view.hidden = on || original === "";
      if (on) { area.focus(); area.setSelectionRange(area.value.length, area.value.length); }
      else area.value = original;
    }
    editBtns.forEach(function (b) { b.addEventListener("click", function () { editing(true); }); });
    desc.querySelector("[data-desc-cancel]").addEventListener("click", function () { editing(false); });
  }

  document.querySelectorAll("[data-reveal]").forEach(function (btn) {
    var form = btn.nextElementSibling;
    if (!form || !form.hasAttribute("data-reveal-form")) return;
    function show(on) {
      btn.hidden = on;
      form.hidden = !on;
      if (on) { var f = form.querySelector("input:not([type=hidden]), select"); if (f) { f.focus(); if (f.select) f.select(); } }
    }
    btn.addEventListener("click", function () { show(true); });
    var cancel = form.querySelector("[data-reveal-cancel]");
    if (cancel) cancel.addEventListener("click", function () { show(false); });
    form.addEventListener("keydown", function (e) { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); show(false); } });
  });

  var feed = document.querySelector("[data-feed]");
  var feedBtn = feed && feed.querySelector("[data-feed-toggle]");
  if (feedBtn) {
    var hide = false;
    try { hide = localStorage.getItem("devFeedHide") === "1"; } catch (e) {}
    function applyFeed() {
      feed.classList.toggle("hide-acts", hide);
      feedBtn.textContent = hide ? "Show details" : "Hide details";
      feedBtn.setAttribute("aria-pressed", hide ? "true" : "false");
    }
    feedBtn.addEventListener("click", function () {
      hide = !hide;
      try { localStorage.setItem("devFeedHide", hide ? "1" : "0"); } catch (e) {}
      applyFeed();
    });
    applyFeed();
  }

  // ---- Board labels: click one to hide or show every label's name ----
  var root = document.documentElement;
  try { if (localStorage.getItem("devLabelsMin") === "1") root.classList.add("dev-labels-min"); } catch (e) {}
  document.addEventListener("click", function (e) {
    var lab = e.target.closest("[data-label-toggle]");
    if (!lab) return;
    e.preventDefault();
    e.stopPropagation();
    var min = root.classList.toggle("dev-labels-min");
    try { localStorage.setItem("devLabelsMin", min ? "1" : "0"); } catch (err) {}
  }, true);

  // ---- Project board drag-and-drop ----
  var board = document.querySelector("[data-board]");
  if (!board) return;
  var csrf = board.getAttribute("data-csrf");
  var dragging = null;

  board.querySelectorAll("[data-task]").forEach(function (card) {
    card.addEventListener("dragstart", function (e) {
      dragging = card;
      card.classList.add("dragging");
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", card.getAttribute("data-task"));
    });
    card.addEventListener("dragend", function () {
      card.classList.remove("dragging");
      board.querySelectorAll(".drop-target").forEach(function (c) { c.classList.remove("drop-target"); });
      dragging = null;
    });
  });

  function afterCard(list, y) {
    var cards = Array.prototype.filter.call(list.querySelectorAll("[data-task]"), function (c) { return c !== dragging; });
    for (var i = 0; i < cards.length; i++) {
      var box = cards[i].getBoundingClientRect();
      if (y < box.top + box.height / 2) return cards[i];
    }
    return null;
  }

  board.querySelectorAll("[data-col]").forEach(function (col) {
    var list = col.querySelector("[data-cards]");
    col.addEventListener("dragover", function (e) {
      if (!dragging) return;
      e.preventDefault();
      col.classList.add("drop-target");
      var before = afterCard(list, e.clientY);
      var empty = list.querySelector(".dev-col-empty");
      if (empty) empty.remove();
      if (before) list.insertBefore(dragging, before); else list.appendChild(dragging);
    });
    col.addEventListener("dragleave", function (e) {
      if (!col.contains(e.relatedTarget)) col.classList.remove("drop-target");
    });
    col.addEventListener("drop", function (e) {
      if (!dragging) return;
      e.preventDefault();
      col.classList.remove("drop-target");
      var card = dragging;
      var next = card.nextElementSibling;
      while (next && !next.hasAttribute("data-task")) next = next.nextElementSibling;
      var body = new URLSearchParams();
      body.set("csrf_token", csrf);
      body.set("status", col.getAttribute("data-col"));
      body.set("before", next ? next.getAttribute("data-task") : "0");
      fetch("/admin/dev/board/" + card.getAttribute("data-task") + "/move", {
        method: "POST", headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded" }, body: body
      }).then(function (r) {
        if (!r.ok) { location.reload(); return; }
        board.querySelectorAll("[data-col]").forEach(function (c) {
          c.querySelector(".dev-col-count").textContent = c.querySelectorAll("[data-task]").length;
        });
      }).catch(function () { location.reload(); });
    });
  });
})();
