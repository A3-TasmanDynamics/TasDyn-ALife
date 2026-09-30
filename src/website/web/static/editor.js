// Faction drive → document editor. A contenteditable page with a toolbar;
// the HTML is copied into the form on submit and cleaned on the server
// (internal/factions/drive.go, SanitizeDoc), which is what makes it safe.
(() => {
  const form = document.querySelector("[data-editor-form]");
  if (!form) return;
  const page = form.querySelector("[data-editor]");
  const body = form.querySelector("[data-editor-body]");
  const status = form.querySelector("[data-editor-status]");
  const toolbar = form.querySelector("[data-editor-toolbar]");
  let dirty = false;

  document.execCommand("defaultParagraphSeparator", false, "p");

  const run = (cmd, value) => {
    page.focus();
    switch (cmd) {
      case "formatBlock":
        document.execCommand("formatBlock", false, "<" + value + ">");
        break;
      case "createLink": {
        const url = prompt("Link address (https://…)", "https://");
        if (url && /^https?:\/\//i.test(url)) document.execCommand("createLink", false, url);
        break;
      }
      case "insertTable": {
        const rows = Math.min(Math.max(parseInt(prompt("How many rows?", "3"), 10) || 0, 1), 30);
        const cols = Math.min(Math.max(parseInt(prompt("How many columns?", "3"), 10) || 0, 1), 8);
        let html = "<table><thead><tr>";
        for (let c = 0; c < cols; c++) html += "<th>Heading</th>";
        html += "</tr></thead><tbody>";
        for (let r = 0; r < rows - 1; r++) {
          html += "<tr>";
          for (let c = 0; c < cols; c++) html += "<td>&nbsp;</td>";
          html += "</tr>";
        }
        html += "</tbody></table><p></p>";
        document.execCommand("insertHTML", false, html);
        break;
      }
      default:
        document.execCommand(cmd, false, null);
    }
    changed();
    syncState();
  };

  toolbar.addEventListener("click", (e) => {
    const b = e.target.closest("button[data-cmd]");
    if (b) { e.preventDefault(); run(b.dataset.cmd); }
  });
  toolbar.querySelector("select[data-cmd]").addEventListener("change", (e) => run("formatBlock", e.target.value));

  // Show which styles apply at the cursor.
  const syncState = () => {
    toolbar.querySelectorAll("button[data-cmd]").forEach((b) => {
      let on = false;
      try { on = document.queryCommandState(b.dataset.cmd); } catch (_) { /* not a state command */ }
      b.classList.toggle("on", on);
      b.setAttribute("aria-pressed", on ? "true" : "false");
    });
    const block = (document.queryCommandValue("formatBlock") || "p").toLowerCase().replace(/[<>]/g, "");
    const sel = toolbar.querySelector("select[data-cmd]");
    if ([...sel.options].some((o) => o.value === block)) sel.value = block;
  };
  document.addEventListener("selectionchange", () => { if (document.activeElement === page) syncState(); });

  // Paste as plain text inside paragraphs, so pasted pages don't bring their styling.
  page.addEventListener("paste", (e) => {
    const html = e.clipboardData.getData("text/html");
    const text = e.clipboardData.getData("text/plain");
    if (!html && !text) return;
    e.preventDefault();
    if (html) {
      const tmp = document.createElement("div");
      tmp.innerHTML = html;
      tmp.querySelectorAll("*").forEach((el) => { el.removeAttribute("style"); el.removeAttribute("class"); });
      tmp.querySelectorAll("script,style,meta,link,iframe,object").forEach((el) => el.remove());
      document.execCommand("insertHTML", false, tmp.innerHTML);
    } else {
      document.execCommand("insertText", false, text);
    }
    changed();
  });

  const changed = () => {
    dirty = true;
    if (status) status.textContent = "Unsaved changes.";
  };
  page.addEventListener("input", changed);
  form.querySelectorAll("input,select").forEach((el) => el.addEventListener("change", changed));

  form.addEventListener("submit", () => {
    body.value = page.innerHTML;
    dirty = false;
  });
  // Ctrl/Cmd+S saves.
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      form.requestSubmit();
    }
  });
  window.addEventListener("beforeunload", (e) => {
    if (dirty) { e.preventDefault(); e.returnValue = ""; }
  });
})();
