(() => {
  document.addEventListener('htmx:responseError', event => showError(event.detail.elt));
  document.addEventListener('htmx:sendError', event => showError(event.detail.elt));

  function showError(element) {
    const button = element?.closest?.('.duration-button');
    if (!button) return;
    button.classList.add('is-error');
    button.title = 'Duration unavailable. Click to retry.';
    setTimeout(() => {
      button.classList.remove('is-error');
      button.title = 'Fetch video duration';
    }, 2500);
  }
})();
