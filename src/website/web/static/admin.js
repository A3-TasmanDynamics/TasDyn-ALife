// Admin Panel behaviour that needs a little JavaScript. Kept in a static
// file (not inline) so a Content-Security-Policy can be added later without
// touching templates.
//
// <button data-dialog-open="id"> opens <dialog id="id"> as a modal;
// <button data-dialog-close> inside a dialog closes it. Everything else
// is plain HTML forms, so pages still work (minus the dialogs) without JS.
document.addEventListener("click", (e) => {
  const opener = e.target.closest("[data-dialog-open]");
  if (opener) {
    const dlg = document.getElementById(opener.dataset.dialogOpen);
    if (dlg && typeof dlg.showModal === "function") {
      e.preventDefault();
      dlg.showModal();
      const first = dlg.querySelector("textarea, input:not([type=hidden]), select");
      if (first) first.focus();
    }
    return;
  }
  const closer = e.target.closest("[data-dialog-close]");
  if (closer) {
    const dlg = closer.closest("dialog");
    if (dlg) {
      e.preventDefault();
      dlg.close();
    }
  }
});
