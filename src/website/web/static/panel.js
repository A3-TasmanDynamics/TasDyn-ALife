// Admin panel chrome: the phone drawer and collapsible sidebar sections.
//
// - [data-nav-toggle] opens / closes the sidebar as a drawer on narrow
//   screens; the backdrop, Esc, or following a link closes it.
// - Sidebar sections (<details data-nav-section>) remember being collapsed
//   in this browser. The section holding the current page always opens.
(function () {
  "use strict";
  var KEY = "alife.nav.closed";
  var body = document.body;
  var toggle = document.querySelector("[data-nav-toggle]");
  var backdrop = document.querySelector(".admin-nav-backdrop");
  var sidebar = document.getElementById("admin-sidebar");

  function setOpen(open) {
    body.classList.toggle("nav-open", open);
    if (toggle) {
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      toggle.setAttribute("aria-label", open ? "Close the menu" : "Open the menu");
    }
    if (backdrop) backdrop.hidden = !open;
    if (open && sidebar) {
      var first = sidebar.querySelector("a, button, input");
      if (first) first.focus();
    }
  }
  if (toggle) toggle.addEventListener("click", function () { setOpen(!body.classList.contains("nav-open")); });
  if (backdrop) backdrop.addEventListener("click", function () { setOpen(false); });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && body.classList.contains("nav-open")) {
      setOpen(false);
      if (toggle) toggle.focus();
    }
  });
  if (sidebar) sidebar.addEventListener("click", function (e) {
    if (e.target.closest("a")) setOpen(false);
  });

  // Collapsed sections.
  var closed = [];
  try { closed = JSON.parse(localStorage.getItem(KEY) || "[]") || []; } catch (e) { closed = []; }
  document.querySelectorAll("details[data-nav-section]").forEach(function (d) {
    var key = d.getAttribute("data-nav-section");
    if (closed.indexOf(key) >= 0 && !d.hasAttribute("data-active")) d.open = false;
    d.addEventListener("toggle", function () {
      var i = closed.indexOf(key);
      if (d.open && i >= 0) closed.splice(i, 1);
      if (!d.open && i < 0) closed.push(key);
      try { localStorage.setItem(KEY, JSON.stringify(closed)); } catch (e) { /* private mode: not remembered */ }
    });
  });

  // Keep the current page's link in view in a long sidebar.
  var cur = sidebar && sidebar.querySelector('[aria-current="page"]');
  if (cur && cur.scrollIntoView && sidebar.scrollHeight > sidebar.clientHeight) cur.scrollIntoView({ block: "nearest" });

  // Theme: Dark (default), Light, or System (follows the OS). The <head>
  // script applies it before the first paint; this keeps it in step.
  var THEME = "alife.theme";
  var media = window.matchMedia ? matchMedia("(prefers-color-scheme: light)") : null;
  function theme() {
    try { return localStorage.getItem(THEME) || "dark"; } catch (e) { return "dark"; }
  }
  function applyTheme() {
    var t = theme();
    var light = t === "light" || (t === "system" && media && media.matches);
    if (light) document.documentElement.setAttribute("data-theme", "light");
    else document.documentElement.removeAttribute("data-theme");
    document.querySelectorAll("[data-theme-set]").forEach(function (b) {
      b.setAttribute("aria-pressed", b.getAttribute("data-theme-set") === t ? "true" : "false");
    });
  }
  document.addEventListener("click", function (e) {
    var b = e.target.closest("[data-theme-set]");
    if (!b) return;
    try { localStorage.setItem(THEME, b.getAttribute("data-theme-set")); } catch (err) { /* not remembered */ }
    applyTheme();
  });
  if (media && media.addEventListener) media.addEventListener("change", applyTheme);
  applyTheme();

  // In-game tools: confirm before a kick goes out.
  document.addEventListener("submit", function (e) {
    var f = e.target;
    var act = f.querySelector && f.querySelector('select[name="action"], input[name="action"]');
    if (!act || act.value !== "kick" || f.getAttribute("action") !== "/admin/game/command") return;
    var who = f.querySelector('select[name="player"]');
    var name = who && who.selectedIndex > 0 ? who.options[who.selectedIndex].text : (f.getAttribute("data-player") || "this player");
    if (!window.confirm("Kick " + name + " from the game server?")) e.preventDefault();
  });
})();
