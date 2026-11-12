// The admin panel's Live Map (/admin/map): players in game on Altis.
// Polls /admin/map.json every 30 s while the tab is visible. The SVG is in
// world metres (y flipped: Arma's y grows north); zooming changes the
// viewBox, and dots / labels are resized so they stay the same on screen.
(function () {
  "use strict";
  var svg = document.querySelector("svg.lm");
  if (!svg) return;
  var NS = "http://www.w3.org/2000/svg";
  var W = +svg.getAttribute("data-world");
  var gGrid = svg.querySelector("[data-lm-grid]"), gPlaces = svg.querySelector("[data-lm-places]"), gPlayers = svg.querySelector("[data-lm-players]");
  var tip = document.querySelector("[data-lm-tip]"), status = document.querySelector("[data-lm-status]");
  var list = document.querySelector("[data-lm-list]"), find = document.querySelector("[data-lm-find]");
  var vb = { x: 0, y: 0, w: W, h: W };
  var players = [], places = [], drawnPlaces = false;

  function el(name, attrs, parent) {
    var e = document.createElementNS(NS, name);
    for (var k in attrs) e.setAttribute(k, attrs[k]);
    if (parent) parent.appendChild(e);
    return e;
  }
  function sy(y) { return W - y; } // world y -> svg y
  function px() { return vb.w / (svg.clientWidth || 1); } // metres per screen pixel

  // ---- grid (1 km; every 5 km stronger and labelled) ----
  for (var m = 0; m <= W; m += 1000) {
    var major = m % 5000 === 0;
    el("line", { x1: m, y1: 0, x2: m, y2: W, class: major ? "lm-major" : "lm-minor" }, gGrid);
    el("line", { x1: 0, y1: m, x2: W, y2: m, class: major ? "lm-major" : "lm-minor" }, gGrid);
  }
  var gridLabels = [];
  for (var k = 5000; k < W; k += 5000) {
    var label = ("0" + k / 1000).slice(-2);
    gridLabels.push(el("text", { x: k, y: W, class: "lm-gridlabel", "text-anchor": "middle" }, gGrid));
    gridLabels[gridLabels.length - 1].textContent = label;
    gridLabels.push(el("text", { x: 0, y: sy(k), class: "lm-gridlabel" }, gGrid));
    gridLabels[gridLabels.length - 1].textContent = label;
  }

  function drawPlaces() {
    if (drawnPlaces || !places.length) return;
    drawnPlaces = true;
    places.forEach(function (p) {
      var g = el("g", { class: p.town ? "lm-town" : "lm-poi", "data-x": p.x, "data-y": sy(p.y) }, gPlaces);
      if (!p.town) el("circle", { cx: p.x, cy: sy(p.y), r: 1 }, g);
      var t = el("text", { x: p.x, y: sy(p.y) }, g);
      t.textContent = p.name;
    });
    sizes();
  }

  function drawPlayers() {
    gPlayers.textContent = "";
    if (list) list.textContent = "";
    players.forEach(function (p) {
      var g = el("g", { class: "lm-p lm-" + (p.faction || "civ"), tabindex: "0", role: "link", "aria-label": p.name, "data-id": p.id }, gPlayers);
      if (p.watched) el("circle", { cx: p.x, cy: sy(p.y), class: "lm-ring lm-ring-watch", "data-ring": 2 }, g);
      if (p.wanted) el("circle", { cx: p.x, cy: sy(p.y), class: "lm-ring lm-ring-wanted", "data-ring": p.watched ? 3 : 2 }, g);
      el("circle", { cx: p.x, cy: sy(p.y), class: "lm-dot" }, g);
      g._p = p;
      if (list) {
        var li = document.createElement("li");
        var a = document.createElement("a");
        a.href = "/admin/players?id=" + p.id;
        a.textContent = p.name + " · " + ({ cop: "Police", medic: "EMS" }[p.faction] || "Civilian") + " · grid " + grid(p);
        li.appendChild(a);
        list.appendChild(li);
      }
    });
    sizes();
    highlight();
  }

  function grid(p) { return ("00" + Math.floor(p.x / 100)).slice(-3) + " " + ("00" + Math.floor(p.y / 100)).slice(-3); }

  // Dots and text keep their screen size whatever the zoom.
  function sizes() {
    var s = px();
    gPlayers.querySelectorAll(".lm-dot").forEach(function (c) { c.setAttribute("r", 5.5 * s); c.setAttribute("stroke-width", 1.5 * s); });
    gPlayers.querySelectorAll(".lm-ring").forEach(function (c) { c.setAttribute("r", (5.5 + 3.5 * +c.getAttribute("data-ring")) * s); c.setAttribute("stroke-width", 2 * s); });
    gPlaces.querySelectorAll(".lm-town text").forEach(function (t) { t.setAttribute("font-size", 14 * s); t.setAttribute("dy", -14 * s); });
    gPlaces.querySelectorAll(".lm-poi text").forEach(function (t) { t.setAttribute("font-size", 11 * s); t.setAttribute("dx", 6 * s); t.setAttribute("dy", 4 * s); });
    gPlaces.querySelectorAll(".lm-poi circle").forEach(function (c) { c.setAttribute("r", 2.5 * s); });
    gGrid.querySelectorAll("line").forEach(function (l) { l.setAttribute("stroke-width", (l.classList.contains("lm-major") ? 1.2 : 0.6) * s); });
    gridLabels.forEach(function (t) { t.setAttribute("font-size", 11 * s); t.setAttribute("dy", t.getAttribute("text-anchor") ? -4 * s : 4 * s); t.setAttribute("dx", t.getAttribute("text-anchor") ? 0 : 4 * s); });
    // Small places' names only when zoomed in enough to read them.
    svg.classList.toggle("lm-close", s < 12);
  }

  function setView(x, y, w) {
    w = Math.max(400, Math.min(W * 1.2, w));
    vb = { x: Math.max(-W * 0.1, Math.min(W * 1.1 - w, x)), y: Math.max(-W * 0.1, Math.min(W * 1.1 - w, y)), w: w, h: w };
    svg.setAttribute("viewBox", vb.x + " " + vb.y + " " + vb.w + " " + vb.h);
    sizes();
  }
  function toWorld(cx, cy) {
    var r = svg.getBoundingClientRect();
    var side = Math.min(r.width, r.height), ox = r.left + (r.width - side) / 2, oy = r.top + (r.height - side) / 2;
    return { x: vb.x + (cx - ox) / side * vb.w, y: vb.y + (cy - oy) / side * vb.h };
  }
  function zoomAt(cx, cy, f) {
    var p = toWorld(cx, cy), w = vb.w * f;
    setView(p.x - (p.x - vb.x) * f, p.y - (p.y - vb.y) * f, w);
  }
  svg.addEventListener("wheel", function (e) { e.preventDefault(); zoomAt(e.clientX, e.clientY, e.deltaY > 0 ? 1.25 : 0.8); }, { passive: false });
  svg.addEventListener("dblclick", function (e) { zoomAt(e.clientX, e.clientY, 0.5); });
  var drag = null;
  svg.addEventListener("pointerdown", function (e) {
    if (e.target.closest(".lm-p")) return;
    drag = { x: e.clientX, y: e.clientY, vx: vb.x, vy: vb.y };
    svg.setPointerCapture(e.pointerId);
  });
  svg.addEventListener("pointermove", function (e) {
    if (drag) {
      var r = svg.getBoundingClientRect(), side = Math.min(r.width, r.height);
      setView(drag.vx - (e.clientX - drag.x) / side * vb.w, drag.vy - (e.clientY - drag.y) / side * vb.h, vb.w);
      return;
    }
    var g = e.target.closest(".lm-p");
    if (!g || !tip) { if (tip) tip.hidden = true; return; }
    var p = g._p, bits = [({ cop: "Police", medic: "EMS" }[p.faction] || "Civilian")];
    if (p.gang) bits.push(p.gang);
    if (p.wanted) bits.push("wanted");
    if (p.watched) bits.push("watched");
    if (p.flags) bits.push(p.flags + " open flag" + (p.flags === 1 ? "" : "s"));
    tip.textContent = "";
    var b = document.createElement("strong"); b.textContent = p.name; tip.appendChild(b);
    var s = document.createElement("span"); s.textContent = bits.join(" · ") + " · grid " + grid(p); tip.appendChild(s);
    var wr = tip.parentNode.getBoundingClientRect();
    tip.style.left = Math.min(e.clientX - wr.left + 14, wr.width - 240) + "px";
    tip.style.top = (e.clientY - wr.top + 14) + "px";
    tip.hidden = false;
  });
  svg.addEventListener("pointerup", function () { drag = null; });
  svg.addEventListener("pointerleave", function () { drag = null; if (tip) tip.hidden = true; });
  function open(g) { if (g && g._p) location.href = "/admin/players?id=" + g._p.id; }
  svg.addEventListener("click", function (e) { open(e.target.closest(".lm-p")); });
  svg.addEventListener("keydown", function (e) { if (e.key === "Enter") open(e.target.closest(".lm-p")); });
  var reset = document.querySelector("[data-lm-reset]");
  if (reset) reset.addEventListener("click", function () { setView(0, 0, W); });
  window.addEventListener("resize", sizes);

  // ---- find ----
  function highlight() {
    var q = find ? find.value.trim().toLowerCase() : "";
    var first = null;
    gPlayers.querySelectorAll(".lm-p").forEach(function (g) {
      var hit = q && g._p.name.toLowerCase().indexOf(q) >= 0;
      g.classList.toggle("lm-hit", !!hit);
      g.classList.toggle("lm-dim", !!q && !hit);
      if (hit && !first) first = g._p;
    });
    return first;
  }
  if (find) find.addEventListener("keydown", function (e) {
    if (e.key !== "Enter") return;
    var p = highlight();
    if (p) setView(p.x - 1500, sy(p.y) - 1500, 3000);
  });
  if (find) find.addEventListener("input", highlight);

  // ---- data ----
  function load() {
    if (document.hidden) return;
    fetch("/admin/map.json", { credentials: "same-origin" })
      .then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); })
      .then(function (d) {
        places = d.places || [];
        drawPlaces();
        players = d.players || [];
        drawPlayers();
        var missing = d.online - players.length;
        status.textContent = players.length + " on the map" + (missing > 0 ? " (" + missing + " joined too recently to have a position)" : "") + " · updated " + d.at;
      })
      .catch(function () { status.textContent = "Couldn't load positions. Retrying…"; });
  }
  load();
  setInterval(load, 30000);
  document.addEventListener("visibilitychange", function () { if (!document.hidden) load(); });
})();
