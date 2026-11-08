document.addEventListener("click", async (event) => {
  const button = event.target.closest("[data-copy-target]");
  if (!button) return;

  const target = document.getElementById(button.dataset.copyTarget);
  if (!target) return;

  const label = button.querySelector("span:last-child");
  try {
    await copyText(target.textContent.trim());
    if (!label) return;
    const original = label.textContent;
    label.textContent = "Copied!";
    window.setTimeout(() => { label.textContent = original; }, 1800);
  } catch (_) {
    if (label) label.textContent = "Copy failed";
  }
});

async function copyText(value) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand("copy");
  textarea.remove();
  if (!copied) throw new Error("clipboard unavailable");
}
