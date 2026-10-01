// Admin → Database: row clicks, copy buttons, CSV export and the SQL
// console's shortcuts, examples and recent queries.
(function () {
  // Click anywhere on a row to inspect it (cells with their own link keep it).
  document.querySelectorAll("#dbb-grid tr[data-href]").forEach(function (tr) {
    tr.addEventListener("click", function (e) {
      if (e.target.closest("a")) return;
      location.href = tr.getAttribute("data-href");
    });
  });

  document.querySelectorAll("[data-copy]").forEach(function (b) {
    b.addEventListener("click", function () {
      var text = b.getAttribute("data-copy");
      var done = function () { b.classList.add("ok"); setTimeout(function () { b.classList.remove("ok"); }, 900); };
      if (navigator.clipboard) navigator.clipboard.writeText(text).then(done, function () {});
    });
  });

  // CSV of a grid as shown (the current page, or the console result).
  function cell(td) {
    var t = td.querySelector(".dbb-null") ? "" : td.textContent.trim();
    return /[",\n]/.test(t) ? '"' + t.replace(/"/g, '""') + '"' : t;
  }
  document.querySelectorAll("[data-csv]").forEach(function (b) {
    b.addEventListener("click", function () {
      var table = document.querySelector(b.getAttribute("data-csv"));
      if (!table) return;
      var lines = [];
      var head = Array.prototype.map.call(table.querySelectorAll("thead th"), function (th) {
        var n = th.querySelector(".dbb-th-name");
        return (n ? n.textContent : th.textContent).replace(/[↑↓]|PK|FK/g, "").trim();
      });
      lines.push(head.join(","));
      table.querySelectorAll("tbody tr").forEach(function (tr) {
        if (tr.querySelector(".dbb-norows")) return;
        lines.push(Array.prototype.map.call(tr.children, cell).join(","));
      });
      var blob = new Blob([lines.join("\n") + "\n"], { type: "text/csv" });
      var a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = (b.getAttribute("data-csv-name") || "export") + ".csv";
      document.body.appendChild(a);
      a.click();
      a.remove();
    });
  });

  // SQL console.
  var form = document.querySelector("[data-sql-form]");
  if (!form) return;
  var box = form.querySelector("[data-sql]");
  box.addEventListener("keydown", function (e) {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); form.requestSubmit(); }
  });
  document.querySelectorAll("[data-example]").forEach(function (b) {
    b.addEventListener("click", function () { box.value = b.getAttribute("data-example"); box.focus(); });
  });

  var KEY = "dbb-recent";
  var recent = [];
  try { recent = JSON.parse(localStorage.getItem(KEY) || "[]") || []; } catch (e) { recent = []; }
  form.addEventListener("submit", function () {
    var q = box.value.trim();
    if (!q) return;
    recent = [q].concat(recent.filter(function (x) { return x !== q; })).slice(0, 8);
    try { localStorage.setItem(KEY, JSON.stringify(recent)); } catch (e) { /* private mode */ }
  });
  var holder = document.querySelector("[data-sql-recent]");
  if (holder && recent.length) {
    recent.forEach(function (q) {
      var b = document.createElement("button");
      b.type = "button";
      b.className = "dbb-chip dbb-chip-mono";
      b.textContent = q.length > 60 ? q.slice(0, 57) + "…" : q;
      b.title = q;
      b.addEventListener("click", function () { box.value = q; box.focus(); });
      holder.appendChild(b);
    });
    holder.hidden = false;
  }
})();
