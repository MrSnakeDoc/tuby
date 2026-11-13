document.addEventListener('click', event => {
  const dialog = event.target;
  if (!(dialog instanceof HTMLDialogElement) || !dialog.open) return;
  if (dialog.dataset.busy === 'true') return;

  const bounds = dialog.getBoundingClientRect();
  const isOutside = event.clientX < bounds.left
    || event.clientX > bounds.right
    || event.clientY < bounds.top
    || event.clientY > bounds.bottom;

  if (isOutside) dialog.close();
});
