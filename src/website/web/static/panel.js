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
})();
