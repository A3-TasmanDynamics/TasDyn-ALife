// Admin → Server Control: polls the live log (the page works without it;
// only the log panel needs JavaScript).
(() => {
  const box = document.querySelector("[data-srv-logs]");
  if (!box) return;
  const out = box.querySelector("[data-srv-out]");
  const filter = box.querySelector("[data-srv-filter]");
  const pause = box.querySelector("[data-srv-pause]");
  const note = box.querySelector("[data-srv-note]");
  let lines = [];
  let paused = false;

  const draw = () => {
    const q = filter.value.trim().toLowerCase();
    const shown = q ? lines.filter((l) => l.toLowerCase().includes(q)) : lines;
    const atBottom = out.scrollHeight - out.scrollTop - out.clientHeight < 40;
    out.textContent = shown.length ? shown.join("\n") : (q ? "No lines match." : "The log is empty.");
    if (atBottom) out.scrollTop = out.scrollHeight;
  };

  const load = async () => {
    if (paused || document.hidden) return;
    try {
      const res = await fetch(box.dataset.srvLogs, { headers: { Accept: "application/json" }, credentials: "same-origin" });
      const body = await res.json();
      if (body.error) {
        note.textContent = "Couldn't read the log: " + body.error;
        return;
      }
      lines = body.lines || [];
      note.textContent = "Refreshes every 5 seconds. Last updated " + new Date().toLocaleTimeString() + ".";
      draw();
    } catch (err) {
      note.textContent = "Couldn't reach the website to refresh the log.";
    }
  };

  filter.addEventListener("input", draw);
  pause.addEventListener("click", () => {
    paused = !paused;
    pause.textContent = paused ? "Resume" : "Pause";
    pause.setAttribute("aria-pressed", String(paused));
    if (!paused) load();
  });
  load().then(() => { out.scrollTop = out.scrollHeight; });
  setInterval(load, 5000);
})();
