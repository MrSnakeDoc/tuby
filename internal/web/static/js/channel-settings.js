(() => {
  document.addEventListener('input', event => clearError(event.target));
  document.addEventListener('change', event => clearError(event.target));

  document.addEventListener('htmx:beforeRequest', event => {
    showError(settingsForm(event.detail.elt), '');
  });

  document.addEventListener('htmx:responseError', event => {
    showError(
      settingsForm(event.detail.elt),
      window.tubyErrorMessage?.(event.detail.xhr, 'Tuby could not save these settings.') ??
        'Tuby could not save these settings.',
    );
  });

  document.addEventListener('htmx:sendError', event => {
    showError(settingsForm(event.detail.elt), 'Tuby could not reach the server. Check your connection and try again.');
  });

  document.addEventListener('htmx:afterRequest', event => {
    const form = settingsForm(event.detail.elt);
    if (form && event.detail.successful) window.tubyShowSuccess?.('Channel settings saved.');
  });

  document.addEventListener('close', event => {
    showError(event.target.querySelector?.('[data-channel-settings]'), '');
  }, true);

  const clearError = element => showError(settingsForm(element), '');
  const settingsForm = element => element?.closest?.('[data-channel-settings]') ?? null;

  const showError = (form, message) => {
    const error = form?.querySelector('[data-channel-settings-error]');
    if (!error) return;
    error.textContent = message;
    error.hidden = message === '';
  };
})();
