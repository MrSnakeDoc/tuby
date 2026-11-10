(() => {
  document.addEventListener('change', event => {
    const form = event.target.closest('[data-content-settings]');
    if (form) clearValidity(form);
  });

  document.addEventListener('submit', event => {
    const form = event.target.closest('[data-content-settings]');
    if (!form || hasSelection(form)) return;
    event.preventDefault();
    const first = form.querySelector('input[type="checkbox"]');
    first.setCustomValidity('Select Long, Shorts, or both.');
    first.reportValidity();
  });

  document.addEventListener('close', event => {
    const form = event.target.querySelector?.('[data-content-settings]');
    if (!form) return;
    form.reset();
    clearValidity(form);
  }, true);

  const hasSelection = form => [...form.querySelectorAll('input[type="checkbox"]')]
    .some(input => input.checked);

  const clearValidity = form => {
    form.querySelectorAll('input[type="checkbox"]').forEach(input => input.setCustomValidity(''));
  };
})();
