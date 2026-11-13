// The public site's header: the More menu, the account menu and the phone
// menu are plain <details> (they work without JavaScript); this closes them
// on an outside click or Esc and keeps only one open. Also the theme switch
// in the account menu (Dark / Light / System, remembered in this browser;
// the <head> script applies it before the first paint).
(function () {
  "use strict";
  var menus = document.querySelectorAll(".site-header details");
  function closeAll(except) {
    menus.forEach(function (d) { if (d !== except) d.open = false; });
  }
  menus.forEach(function (d) {
    d.addEventListener("toggle", function () { if (d.open) closeAll(d); });
  });
  document.addEventListener("click", function (e) {
    if (!e.target.closest(".site-header details")) closeAll(null);
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var open = document.querySelector(".site-header details[open]");
    if (open) {
      open.open = false;
      var s = open.querySelector("summary");
      if (s) s.focus();
    }
  });

  // Faction command's section menu: open beside the page on wide screens,
  // a closed "Menu" bar on tablets and phones (without JavaScript it just
  // stays open). Only re-synced when the width crosses the breakpoint, so
  // opening it on a phone sticks.
  var cmdMenu = document.querySelector(".cmd-menu");
  var narrow = window.matchMedia ? matchMedia("(max-width: 1000px)") : null;
  if (cmdMenu && narrow) {
    var syncMenu = function () { cmdMenu.open = !narrow.matches; };
    syncMenu();
    if (narrow.addEventListener) narrow.addEventListener("change", syncMenu);
    cmdMenu.addEventListener("toggle", function () { if (!narrow.matches && !cmdMenu.open) cmdMenu.open = true; });
  }

  var KEY = "alife.theme";
  var media = window.matchMedia ? matchMedia("(prefers-color-scheme: light)") : null;
  function theme() { try { return localStorage.getItem(KEY) || "dark"; } catch (e) { return "dark"; } }
  function apply() {
    var t = theme();
    if (t === "light" || (t === "system" && media && media.matches)) document.documentElement.setAttribute("data-theme", "light");
    else document.documentElement.removeAttribute("data-theme");
    document.querySelectorAll("[data-theme-set]").forEach(function (b) {
      b.setAttribute("aria-pressed", b.getAttribute("data-theme-set") === t ? "true" : "false");
    });
  }
  document.addEventListener("click", function (e) {
    var b = e.target.closest("[data-theme-set]");
    if (!b) return;
    try { localStorage.setItem(KEY, b.getAttribute("data-theme-set")); } catch (err) { /* not remembered */ }
    apply();
  });
  if (media && media.addEventListener) media.addEventListener("change", apply);
  apply();
})();
