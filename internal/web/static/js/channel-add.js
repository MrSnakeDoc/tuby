(() => {
  document.addEventListener('input', event => clearFormError(event.target));
  document.addEventListener('change', event => clearFormError(event.target));

  document.addEventListener('htmx:beforeRequest', event => {
    showError(addChannelForm(event.detail.elt), '');
  });

  document.addEventListener('htmx:responseError', event => {
    showError(addChannelForm(event.detail.elt), responseErrorMessage(event.detail.xhr));
  });

  document.addEventListener('htmx:sendError', event => {
    showError(
      addChannelForm(event.detail.elt),
      'Tuby could not reach the server. Check your connection and try again.',
    );
  });

  document.addEventListener('htmx:beforeOnLoad', event => {
    if (!addChannelForm(event.detail.elt)) return;
    if (event.detail.xhr.status < 200 || event.detail.xhr.status >= 300) return;
    window.tubyQueueSuccess?.('Channel added successfully.');
  });

  document.addEventListener('close', event => {
    showError(event.target.querySelector?.('[data-add-channel]'), '');
  }, true);

  const clearFormError = element => showError(addChannelForm(element), '');

  const addChannelForm = element => element?.closest?.('[data-add-channel]') ?? null;

  const showError = (form, message) => {
    const error = form?.querySelector('[data-add-channel-error]');
    if (!error) return;
    error.textContent = message;
    error.hidden = message === '';
  };

  const responseErrorMessage = xhr => {
    try {
      const message = JSON.parse(xhr?.responseText ?? '').error;
      if (typeof message === 'string' && message.trim() !== '') return message;
    } catch {
      // The fallback below also covers non-JSON proxy responses.
    }
    return 'Tuby could not add this channel. Try again later.';
  };
})();
