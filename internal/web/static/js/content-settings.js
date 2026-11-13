(() => {
  let settingsLoad = 0;

  const syncCardSettings = () => {
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    document.querySelectorAll('.category-list .channel-card').forEach(card => {
      if (card.querySelector('[data-card-settings-button]')) return;
      const button = document.createElement('button');
      button.className = 'channel-card-settings';
      button.type = 'button';
      button.dataset.cardSettingsButton = '';
      button.title = 'Channel settings';
      button.setAttribute('aria-label', `Settings for ${channelName(card)}`);
      button.innerHTML = `<img src="/static/img/gear.svg?v=${encodeURIComponent(build)}" alt="">`;
      card.append(button);
    });
  };

  document.addEventListener('click', event => {
    const button = event.target.closest('[data-card-settings-button]');
    if (button) void openCardSettings(button);
  });

  document.addEventListener('change', event => {
    const form = event.target.closest('[data-content-settings]');
    if (form) clearValidity(form);
  });

  document.addEventListener('submit', event => {
    const form = event.target.closest('[data-content-settings]');
    if (!form) return;
    if (!hasSelection(form)) {
      event.preventDefault();
      const first = form.querySelector('input[type="checkbox"]');
      first.setCustomValidity('Select Long, Shorts, or both.');
      first.reportValidity();
      return;
    }
    if (form.matches('[data-card-settings]')) {
      event.preventDefault();
      void saveCardSettings(form);
    }
  });

  document.addEventListener('close', event => {
    const form = event.target.querySelector?.('[data-content-settings]');
    if (!form) return;
    if (form.matches('[data-card-settings]')) settingsLoad++;
    form.reset();
    clearValidity(form);
    showCardError(form, '');
  }, true);

  const openCardSettings = async button => {
    const card = button.closest('.channel-card');
    const dialog = cardSettingsDialog();
    const form = dialog.querySelector('form');
    form.dataset.channelId = card.dataset.channelId;
    dialog.querySelector('[data-card-settings-title]').textContent = channelName(card);
    populateCategories(form.querySelector('[data-category-select]'));
    showCardError(form, '');
    setCardFormLoading(form, true);
    dialog.showModal();
    const load = ++settingsLoad;
    let loaded = false;
    try {
      const response = await fetch(`/api/channels/${encodeURIComponent(card.dataset.channelId)}/settings`, {
        headers: window.tubyBuildHeaders?.() ?? {},
      });
      if (load !== settingsLoad) return;
      window.tubyReloadIfStale?.(response);
      if (!response.ok) {
        showCardError(form, await fetchErrorMessage(response, 'Could not load these settings.'));
        return;
      }
      const settings = await response.json();
      form.querySelector('[name="category_id"]').value = String(settings.category_id);
      form.querySelector('[name="include_long"]').checked = settings.include_long;
      form.querySelector('[name="include_shorts"]').checked = settings.include_shorts;
      dialog.querySelector('[data-card-settings-title]').textContent = settings.title;
      loaded = true;
    } catch {
      if (load === settingsLoad) showCardError(form, 'Could not load these settings.');
    } finally {
      if (load === settingsLoad) setCardFormLoading(form, !loaded);
    }
  };

  const saveCardSettings = async form => {
    const submit = form.querySelector('[type="submit"]');
    submit.disabled = true;
    showCardError(form, '');
    try {
      const response = await fetch(`/api/channels/${encodeURIComponent(form.dataset.channelId)}/settings`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          ...(window.tubyBuildHeaders?.() ?? {}),
        },
        body: new URLSearchParams(new FormData(form)),
      });
      window.tubyReloadIfStale?.(response);
      if (!response.ok) {
        showCardError(form, await fetchErrorMessage(response, 'Could not save these settings.'));
        return;
      }
      form.closest('dialog').close();
      window.tubyShowSuccess?.('Channel settings saved.');
      document.dispatchEvent(new Event('tuby:update'));
    } catch {
      showCardError(form, 'Could not save these settings.');
    } finally {
      submit.disabled = false;
    }
  };

  const cardSettingsDialog = () => {
    const existing = document.querySelector('#channel-card-settings-dialog');
    if (existing) return existing;
    const dialog = document.createElement('dialog');
    dialog.className = 'add-dialog';
    dialog.id = 'channel-card-settings-dialog';
    dialog.innerHTML = `<form data-content-settings data-card-settings>
      <div class="dialog-heading"><div><span class="eyebrow">CHANNEL SETTINGS</span>
      <h2 data-card-settings-title></h2></div>
      <button class="icon-button close-button" type="button" data-card-settings-close>×</button></div>
      <div class="category-field"><label for="card-settings-category">Category</label>
      <div class="category-select-row"><select class="category-select" id="card-settings-category"
      name="category_id" data-category-select required></select></div></div>
      <fieldset class="content-options"><legend>Content types</legend>
      <label class="content-option"><input type="checkbox" name="include_long"><span>Long</span></label>
      <label class="content-option"><input type="checkbox" name="include_shorts"><span>Shorts</span></label>
      </fieldset><span class="field-error" data-card-settings-error hidden></span>
      <div class="confirm-actions"><button class="secondary-button" type="button"
      data-card-settings-close>Cancel</button><button class="primary-button" type="submit">Save</button></div>
      </form>`;
    dialog.querySelectorAll('[data-card-settings-close]').forEach(button => {
      button.addEventListener('click', () => dialog.close());
    });
    document.body.append(dialog);
    return dialog;
  };

  const populateCategories = select => {
    const selected = select.value;
    select.replaceChildren(...[...document.querySelectorAll('.channel-category')].map(section => {
      const option = document.createElement('option');
      option.value = section.dataset.categoryId;
      option.textContent = section.querySelector('[data-category-name]').textContent.trim();
      return option;
    }));
    if ([...select.options].some(option => option.value === selected)) select.value = selected;
  };

  const setCardFormLoading = (form, loading) => {
    form.querySelectorAll('select, input, [type="submit"]').forEach(control => {
      control.disabled = loading;
    });
  };

  const showCardError = (form, message) => {
    const error = form.querySelector('[data-card-settings-error]');
    if (!error) return;
    error.textContent = message;
    error.hidden = message === '';
  };

  const fetchErrorMessage = (response, fallback) => window.tubyFetchErrorMessage?.(response, fallback) ??
    Promise.resolve(fallback);

  const channelName = card => card.querySelector('.channel-name')?.textContent?.trim() ?? 'channel';

  const hasSelection = form => [...form.querySelectorAll('input[type="checkbox"]')]
    .some(input => input.checked);

  const clearValidity = form => {
    form.querySelectorAll('input[type="checkbox"]').forEach(input => input.setCustomValidity(''));
  };

  syncCardSettings();
  document.addEventListener('htmx:afterSwap', syncCardSettings);
  document.addEventListener('htmx:historyRestore', syncCardSettings);
})();
