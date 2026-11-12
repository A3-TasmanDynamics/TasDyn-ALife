// Hover for the admin panel's line charts (web/templates/partial_line_chart.html):
// a crosshair at the nearest snapshot and a tooltip with every series' value.
// Re-binds after live.js swaps a page's content (the "panel:updated" event).
(function () {
  function init(root) {
  root.querySelectorAll(".lc[data-hover]").forEach(function (box) {
    if (box.dataset.bound) return;
    box.dataset.bound = "1";
    var data;
    try { data = JSON.parse(box.dataset.hover); } catch (e) { return; }
    if (!data || !data.x || data.x.length < 2) return;
    var svg = box.querySelector("svg"), cross = box.querySelector(".lc-cross"), tip = box.querySelector(".lc-tip");
    var vb = svg.viewBox.baseVal;

    function show(clientX) {
      var r = svg.getBoundingClientRect();
      var sx = (clientX - r.left) / r.width * vb.width;
      var best = 0;
      for (var i = 1; i < data.x.length; i++) {
        if (Math.abs(data.x[i] - sx) < Math.abs(data.x[best] - sx)) best = i;
      }
      cross.setAttribute("x1", data.x[best]);
      cross.setAttribute("x2", data.x[best]);
      cross.setAttribute("visibility", "visible");
      tip.textContent = "";
      var h = document.createElement("strong");
      h.textContent = data.t[best];
      tip.appendChild(h);
      data.n.forEach(function (name, s) {
        var row = document.createElement("div");
        row.textContent = name + ": " + data.s[s][best];
        tip.appendChild(row);
      });
      tip.hidden = false;
      var px = data.x[best] / vb.width * r.width;
      var left = px + 12;
      if (left + tip.offsetWidth > r.width) left = px - tip.offsetWidth - 12;
      tip.style.left = Math.max(0, left) + "px";
    }
    function hide() { cross.setAttribute("visibility", "hidden"); tip.hidden = true; }

    svg.addEventListener("mousemove", function (e) { show(e.clientX); });
    svg.addEventListener("mouseleave", hide);
    svg.addEventListener("touchstart", function (e) { show(e.touches[0].clientX); }, { passive: true });
    svg.addEventListener("touchmove", function (e) { show(e.touches[0].clientX); }, { passive: true });
    svg.addEventListener("touchend", hide);
  });
  }
  init(document);
  document.addEventListener("panel:updated", function (e) { init(e.target); });
})();
