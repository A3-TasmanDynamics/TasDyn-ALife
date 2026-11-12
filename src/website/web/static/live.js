// Live admin panel: listens to /admin/live (Server-Sent Events) and
//  - keeps the sidebar and tab badges current ([data-badge]),
//  - pops a toast for new anti-cheat flags, tickets, cases and watched
//    players joining (click to open; they fade after a while),
//  - refreshes the page's content in place when the page asked for those
//    topics (<div class="admin-content" data-live="flag case ...">), but
//    only while you're not typing, in a dialog or selecting text.
// The dot by the bell shows the connection. Without JavaScript, or if the
// stream drops, pages simply work as before.
(function () {
  "use strict";
  if (!window.EventSource) return;
  var dot = document.querySelector("[data-live-dot]");
  var content = document.querySelector(".admin-content[data-live]");
  var topics = content ? content.getAttribute("data-live").split(/\s+/) : [];
  var stack = null, pending = false, lastInput = 0, es = null, retry = 0;

  function setDot(state, label) {
    if (!dot) return;
    dot.hidden = false;
    dot.className = "live-dot live-" + state;
    dot.title = "Live updates: " + label;
  }

  // ---- badges ----
  function badge(key, n, hot) {
    document.querySelectorAll('[data-badge="' + key + '"]').forEach(function (el) {
      el.textContent = n;
      el.hidden = !n;
      if (hot !== undefined) el.classList.toggle("admin-nav-count-hot", !!hot);
    });
  }
  function counts(c) {
    if ("cases" in c) badge("cases", c.cases, true);
    if ("bans" in c) badge("bans", c.bans, false);
    if ("anticheat" in c) badge("anticheat", c.anticheat, true);
    if ("applications" in c) badge("applications", c.applications, true);
    if ("tickets_open" in c) {
      if (c.tickets_unassigned) badge("tickets", c.tickets_unassigned, true);
      else badge("tickets", c.tickets_open, false);
    }
    if ("online" in c) document.querySelectorAll('[data-live-count="online"]').forEach(function (el) { el.textContent = c.online; });
  }

  // ---- toasts ----
  function toast(t) {
    if (!stack) {
      stack = document.createElement("div");
      stack.className = "live-toasts";
      stack.setAttribute("role", "status");
      stack.setAttribute("aria-live", "polite");
      document.body.appendChild(stack);
    }
    var a = document.createElement("a");
    a.className = "live-toast live-toast-" + (t.level || "info");
    a.href = t.url;
    var h = document.createElement("strong");
    h.textContent = t.title;
    var p = document.createElement("span");
    p.textContent = t.text;
    var x = document.createElement("button");
    x.type = "button";
    x.className = "live-toast-x";
    x.setAttribute("aria-label", "Dismiss");
    x.textContent = "×";
    x.addEventListener("click", function (e) { e.preventDefault(); e.stopPropagation(); a.remove(); });
    a.appendChild(h); a.appendChild(p); a.appendChild(x);
    stack.appendChild(a);
    while (stack.children.length > 4) stack.firstChild.remove();
    var ttl = t.level === "warning" || t.level === "critical" ? 20000 : 9000;
    setTimeout(function () { a.classList.add("live-toast-out"); setTimeout(function () { a.remove(); }, 400); }, ttl);
  }

  // ---- in-place refresh ----
  document.addEventListener("input", function () { lastInput = Date.now(); }, true);
  document.addEventListener("keydown", function () { lastInput = Date.now(); }, true);
  function busy() {
    var a = document.activeElement;
    if (a && a !== document.body && content.contains(a) && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName)) return true;
    if (document.querySelector("dialog[open]")) return true;
    var sel = window.getSelection && window.getSelection();
    if (sel && String(sel).length) return true;
    return Date.now() - lastInput < 4000;
  }
  function refresh() {
    if (!content || pending) return;
    pending = true;
    (function attempt() {
      if (document.hidden || busy()) { setTimeout(attempt, 2500); return; }
      fetch(location.href, { credentials: "same-origin", headers: { "X-Live-Refresh": "1" } })
        .then(function (r) { if (!r.ok) throw new Error(r.status); return r.text(); })
        .then(function (html) {
          pending = false;
          if (busy()) { refresh(); return; }
          var doc = new DOMParser().parseFromString(html, "text/html");
          var fresh = doc.querySelector(".admin-content[data-live]");
          if (!fresh) return;
          var y = window.scrollY;
          content.innerHTML = fresh.innerHTML;
          window.scrollTo(0, y);
          content.dispatchEvent(new CustomEvent("panel:updated", { bubbles: true }));
        })
        .catch(function () { pending = false; });
    })();
  }
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden && content && content.getAttribute("data-live-stale") === "1") {
      content.removeAttribute("data-live-stale");
      refresh();
    }
  });
  function touched(topic) {
    if (topics.indexOf(topic) < 0) return;
    if (document.hidden) { content.setAttribute("data-live-stale", "1"); return; }
    clearTimeout(touched.t);
    touched.t = setTimeout(refresh, 600); // coalesce a burst
  }

  // ---- the stream ----
  var home = { flag: "/admin/anticheat", ticket: "/admin/tickets", "case": "/admin/cases", watch: "/admin/online" };
  function connect() {
    setDot("connecting", "connecting");
    es = new EventSource("/admin/live");
    es.onopen = function () { retry = 0; setDot("on", "on"); };
    es.onerror = function () {
      setDot("off", "reconnecting");
      if (es.readyState === EventSource.CLOSED) {
        retry = Math.min(retry + 1, 6);
        setTimeout(connect, 2000 * retry);
      }
    };
    es.addEventListener("counts", function (e) { counts(JSON.parse(e.data)); });
    es.addEventListener("online", function () { touched("online"); });
    es.addEventListener("gamecmd", function () { touched("gamecmd"); });
    ["flag", "ticket", "case", "watch"].forEach(function (topic) {
      es.addEventListener(topic, function (e) {
        var t = JSON.parse(e.data);
        touched(topic);
        // No toast on the page that lists these anyway (it refreshes).
        if (!(home[topic] && location.pathname.indexOf(home[topic]) === 0 && !document.hidden)) toast(t);
      });
    });
  }
  connect();
  window.addEventListener("pagehide", function () { if (es) es.close(); });

  // A hidden tab lets go of its stream after a while (browsers allow only
  // a few connections per site over HTTP/1.1) and catches up on return:
  // the counts arrive on reconnect, and a live page refreshes.
  var idle = 0;
  document.addEventListener("visibilitychange", function () {
    clearTimeout(idle);
    if (document.hidden) {
      idle = setTimeout(function () {
        if (es) { es.close(); es = null; }
        setDot("off", "paused while the tab is hidden");
        if (content) content.setAttribute("data-live-stale", "1");
      }, 30000);
    } else if (!es) {
      connect();
    }
  });
})();
