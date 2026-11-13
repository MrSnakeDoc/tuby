(() => {
  document.addEventListener('htmx:responseError', event => {
    showError(event.detail.elt, window.tubyErrorMessage?.(
      event.detail.xhr,
      'The video duration could not be fetched. Try again later.',
    ));
  });
  document.addEventListener('htmx:sendError', event => {
    showError(event.detail.elt, 'Tuby could not reach the server. Check your connection and try again.');
  });

  function showError(element, message) {
    const button = element?.closest?.('.duration-button');
    if (!button) return;
    button.classList.add('is-error');
    button.title = 'Duration unavailable. Click to retry.';
    window.tubyShowError?.(message ?? 'The video duration could not be fetched. Try again later.');
    setTimeout(() => {
      button.classList.remove('is-error');
      button.title = 'Fetch video duration';
    }, 2500);
  }
})();
