// Faction command → Ranks & gear: a cabinet rank is always a command rank.
(() => {
  const cmd = document.querySelector("[data-rank-cmd]");
  const cab = document.querySelector("[data-rank-cab]");
  if (!cmd || !cab) return;
  cab.addEventListener("change", () => { if (cab.checked) cmd.checked = true; });
  cmd.addEventListener("change", () => { if (!cmd.checked) cab.checked = false; });
})();

// Faction command → Discipline: keeps the points field inside the chosen
// offence's range and previews what the entry does to the officer's points
// (layout plan "Issue discipline"). The server re-checks everything; the
// form works without this script.
(() => {
  const form = document.querySelector("[data-disc-form]");
  if (!form) return;
  const $ = (sel) => form.querySelector(sel);
  const who = $("[data-disc-who]");
  const off = $("[data-disc-off]");
  const mvw = $("[data-disc-mvw]");
  const pts = $("[data-disc-pts]");
  const ptsWrap = $("[data-disc-points-wrap]");
  const range = $("[data-disc-range]");
  const preview = $("[data-disc-preview]");
  const title = $("[data-disc-preview-title]");
  const body = $("[data-disc-preview-body]");
  const applyWrap = $("[data-disc-apply-wrap]");
  const applyLabel = $("[data-disc-apply-label]");
  const steps = [...document.querySelectorAll("[data-disc-step]")].map((el) => ({
    el, at: Number(el.dataset.discStep), label: el.querySelector(".cmd-strong").textContent,
  }));

  const update = (fromOffence) => {
    const o = off.selectedOptions[0];
    const p = who.selectedOptions[0];
    if (!o || !p) return;
    const min = Number(o.dataset.min), max = Number(o.dataset.max);
    const allowMvw = o.dataset.mvw === "1";
    mvw.disabled = !allowMvw;
    if (!allowMvw) mvw.checked = false;
    pts.min = min;
    pts.max = max;
    if (fromOffence || Number(pts.value) < min || Number(pts.value) > max) pts.value = min;
    ptsWrap.hidden = mvw.checked;
    range.textContent = "Guide for this offence: " + o.dataset.range + ".";

    const name = p.textContent.split(" · ")[0];
    const before = Number(p.dataset.points);
    const mvws = Number(p.dataset.mvws);
    let after = before;
    let converted = false;
    if (mvw.checked) {
      converted = mvws + 1 >= 3;
      if (converted) after += 10;
    } else {
      after += Number(pts.value) || 0;
    }
    const crossed = steps.filter((s) => before < s.at && after >= s.at).pop();
    const band = steps.filter((s) => after >= s.at).pop();
    steps.forEach((s) => s.el.classList.toggle("disc-step-hit", !!band && s === band));

    preview.hidden = false;
    preview.classList.toggle("disc-preview-hot", !!crossed);
    if (mvw.checked && !converted) {
      title.textContent = name + " gets a marked verbal warning";
      body.textContent = mvws ? "They already have " + mvws + " active warning" + (mvws === 1 ? "" : "s") + "; a third converts to 10 points." : "No other active warnings.";
    } else {
      title.textContent = name + ": " + before + " → " + after + " points" + (converted ? " (third warning becomes 10 points)" : "");
      body.textContent = crossed
        ? "This crosses " + crossed.at + " points. Suggested: " + crossed.label.toLowerCase() + "."
        : band ? "Still in the " + band.label.toLowerCase() + " band. No new action suggested." : "Below the first action at 10 points.";
    }
    applyWrap.hidden = !crossed;
    if (crossed) applyLabel.textContent = "Apply “" + crossed.label + "” now";
  };

  off.addEventListener("change", () => update(true));
  who.addEventListener("change", () => update(false));
  mvw.addEventListener("change", () => update(false));
  pts.addEventListener("input", () => update(false));
  update(true);
})();

// Faction command → Roster: each editable row saves as it's edited. RC and
// EA are one or the other. Without JavaScript each row has a Save button.
(() => {
  const forms = document.querySelectorAll("[data-roster-form]");
  if (!forms.length) return;
  document.querySelectorAll(".roster-save").forEach((b) => { b.hidden = true; });
  const msg = document.querySelector("[data-roster-msg]");
  const say = (text, bad) => {
    if (!msg) return;
    msg.textContent = text;
    msg.hidden = !text;
    msg.classList.toggle("banner-error", !!bad);
  };
  const save = async (form) => {
    const row = document.querySelector(`[data-roster-row="${form.id}"]`);
    row.classList.remove("roster-saved", "roster-error");
    row.classList.add("roster-saving");
    row.title = "";
    try {
      const res = await fetch(form.action, { method: "POST", body: new URLSearchParams(new FormData(form)), headers: { Accept: "application/json" }, credentials: "same-origin" });
      const body = await res.json();
      row.classList.remove("roster-saving");
      const name = row.querySelector(".roster-name a")?.textContent || "That row";
      if (body.error) {
        row.classList.add("roster-error");
        row.title = body.error;
        say(name + " wasn't saved: " + body.error.charAt(0).toUpperCase() + body.error.slice(1) + ".", true);
        return;
      }
      say(body.changed && body.changed.length ? name + ": " + body.changed.join("; ") + ". Saved." : "", false);
      row.classList.add("roster-saved");
      setTimeout(() => row.classList.remove("roster-saved"), 1500);
    } catch (err) {
      row.classList.remove("roster-saving");
      row.classList.add("roster-error");
      row.title = "Couldn't reach the website. Not saved.";
      say("Couldn't reach the website. That change wasn't saved.", true);
    }
  };
  forms.forEach((form) => {
    const fields = document.querySelectorAll(`[form="${form.id}"]`);
    const rc = document.querySelector(`[form="${form.id}"][data-roster-rc]`);
    const ea = document.querySelector(`[form="${form.id}"][data-roster-ea]`);
    fields.forEach((el) => {
      el.addEventListener("change", () => {
        if (el === rc && rc.checked && ea) ea.checked = false;
        if (el === ea && ea.checked && rc) rc.checked = false;
        save(form);
      });
    });
    form.addEventListener("submit", (e) => { e.preventDefault(); save(form); });
  });
})();
