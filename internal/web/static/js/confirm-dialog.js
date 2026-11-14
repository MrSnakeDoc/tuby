(() => {
  let pending;
  const dialog = createDialog();
  const cancel = dialog.querySelector('[data-confirm-cancel]');
  const confirm = dialog.querySelector('[data-confirm-submit]');

  document.addEventListener('htmx:confirm', event => {
    if (!event.target.hasAttribute('hx-confirm')) return;
    event.preventDefault();
    pending = {element: event.target, issueRequest: event.detail.issueRequest};
    dialog.querySelector('.confirm-message').textContent = event.detail.question;
    showError('');
    setBusy(false);
    dialog.showModal();
  });

  document.addEventListener('htmx:afterRequest', event => {
    if (!pending || event.detail.elt !== pending.element) return;
    setBusy(false);
    if (event.detail.successful) {
      const successMessage = pending.element.dataset.successMessage;
      dialog.close();
      pending = undefined;
      if (successMessage) window.tubyShowSuccess?.(successMessage);
      return;
    }
    showError(window.tubyErrorMessage?.(event.detail.xhr, 'Tuby could not complete this action.') ??
      'Tuby could not complete this action.');
  });

  cancel.addEventListener('click', () => {
    pending = undefined;
    dialog.close();
  });

  confirm.addEventListener('click', () => {
    if (!pending || dialog.dataset.busy === 'true') return;
    showError('');
    setBusy(true);
    pending.issueRequest(true);
  });

  dialog.addEventListener('close', () => {
    if (dialog.dataset.busy !== 'true') pending = undefined;
    showError('');
  });

  dialog.addEventListener('cancel', event => {
    if (dialog.dataset.busy === 'true') event.preventDefault();
  });

  function setBusy(busy) {
    dialog.dataset.busy = String(busy);
    dialog.setAttribute('aria-busy', String(busy));
    cancel.disabled = busy;
    confirm.disabled = busy;
  }

  function showError(message) {
    const error = dialog.querySelector('[data-confirm-error]');
    error.textContent = message;
    error.hidden = message === '';
  }

  function createDialog() {
    const element = document.createElement('dialog');
    element.id = 'confirm-dialog';
    element.className = 'add-dialog confirm-dialog';
    element.innerHTML = `<div class="dialog-heading"><div><span class="eyebrow">CONFIRM ACTION</span>
      <h2>Are you sure?</h2></div></div><p class="confirm-message"></p>
      <span class="field-error" role="alert" data-confirm-error hidden></span>
      <div class="confirm-actions"><button class="secondary-button" type="button" data-confirm-cancel>Cancel</button>
      <button class="primary-button" type="button" data-confirm-submit>Confirm</button></div>`;
    document.body.append(element);
    return element;
  }
})();
