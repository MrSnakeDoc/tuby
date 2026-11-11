(() => {
  const storagePrefix = 'tuby.category.collapsed.';

  const syncCategories = () => {
    const list = document.querySelector('.category-list');
    const actions = document.querySelector('.top-actions');
    if (!list || !actions) {
      actions?.querySelector('[data-create-category-button]')?.remove();
      actions?.querySelector('[data-category-collapse]')?.remove();
      return;
    }
    list.querySelectorAll('.channel-category').forEach(section => {
      setCollapsed(section, storedCollapsed(section.dataset.categoryId), false);
    });
    syncCategoryOptions(list);
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    mountCategoryOrderButtons(list, build);
    mountPageActions(list, actions, build);
  };

  const mountPageActions = (list, actions, build) => {
    mountCreateCategoryButton(actions, build);
    let collapse = actions.querySelector('[data-category-collapse]');
    if (!collapse) {
      collapse = document.createElement('button');
      collapse.className = 'icon-button';
      collapse.type = 'button';
      collapse.dataset.categoryCollapse = '';
      actions.append(collapse);
    }
    updateCollapseButton(list, collapse, build);
  };

  const mountCreateCategoryButton = (actions, build) => {
    if (actions.querySelector('[data-create-category-button]')) return;
    const button = document.createElement('button');
    button.className = 'icon-button';
    button.type = 'button';
    button.dataset.createCategoryButton = '';
    button.title = 'Create a category';
    button.innerHTML = `<img src="/static/img/category.svg?v=${encodeURIComponent(build)}" alt="">`;
    actions.append(button);
  };

  const updateCollapseButton = (list, button, build) => {
    const sections = [...list.querySelectorAll('.channel-category')];
    const allCollapsed = sections.length > 0 && sections.every(section => section.classList.contains('is-collapsed'));
    const title = allCollapsed ? 'Expand all categories' : 'Collapse all categories';
    const icon = allCollapsed ? 'carret-down' : 'carret-right';
    button.title = title;
    button.setAttribute('aria-label', title);
    button.innerHTML = `<img src="/static/img/${icon}.svg?v=${encodeURIComponent(build)}" alt="">`;
  };

  const toggleAllCategories = button => {
    const list = document.querySelector('.category-list');
    if (!list) return;
    const sections = [...list.querySelectorAll('.channel-category')];
    const collapse = sections.some(section => !section.classList.contains('is-collapsed'));
    setAllCategories(collapse, list, button);
  };

  const setAllCategories = (collapsed, list, button) => {
    list.querySelectorAll('.channel-category').forEach(section => setCollapsed(section, collapsed, true));
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    updateCollapseButton(list, button, build);
  };

  window.tubySetAllCategories = collapsed => {
    const list = document.querySelector('.category-list');
    const button = document.querySelector('[data-category-collapse]');
    if (!list || !button) return false;
    setAllCategories(collapsed, list, button);
    return true;
  };

  const mountCategoryOrderButtons = (list, build) => {
    list.querySelectorAll('.channel-category').forEach(section => {
      const actions = section.querySelector('.category-actions');
      if (actions.querySelector('[data-category-channel-order]')) return;
      const button = document.createElement('button');
      button.className = 'category-action';
      button.type = 'button';
      button.dataset.categoryChannelOrder = '';
      button.title = 'Reorder channels in this category';
      button.setAttribute('aria-label', button.title);
      button.innerHTML = `<img src="/static/img/arrow-left-right.svg?v=${encodeURIComponent(build)}" alt="">`;
      actions.prepend(button);
    });
  };

  document.addEventListener('click', event => {
    const inlineStart = event.target.closest('[data-inline-category-start]');
    if (inlineStart) {
      startInlineCreate(inlineStart.closest('.category-field'));
      return;
    }
    const inlineCancel = event.target.closest('[data-inline-category-cancel]');
    if (inlineCancel) {
      resetInlineCreate(inlineCancel.closest('.category-field'));
      return;
    }
    const inlineSave = event.target.closest('[data-inline-category-save]');
    if (inlineSave) {
      void createInlineCategory(inlineSave.closest('.category-field'));
      return;
    }
    if (event.target.closest('[data-create-category-button]')) {
      document.querySelector('#category-dialog')?.showModal();
      return;
    }
    const collapse = event.target.closest('[data-category-collapse]');
    if (collapse) {
      toggleAllCategories(collapse);
      return;
    }
    const toggle = event.target.closest('.category-toggle');
    if (toggle) {
      const section = toggle.closest('.channel-category');
      setCollapsed(section, !section.classList.contains('is-collapsed'), true);
      const collapseButton = document.querySelector('[data-category-collapse]');
      const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
      if (collapseButton) updateCollapseButton(section.closest('.category-list'), collapseButton, build);
      return;
    }
    const rename = event.target.closest('[data-category-rename]');
    if (rename) {
      startRename(rename.closest('.channel-category'));
      return;
    }
    const cancel = event.target.closest('[data-category-cancel]');
    if (cancel) cancelRename(cancel.closest('.channel-category'));
  });

  document.addEventListener('submit', async event => {
    const form = event.target.closest('[data-category-create], .category-rename');
    if (!form) return;
    event.preventDefault();
    await submitCategory(form);
  });

  document.addEventListener('input', event => {
    const container = event.target.closest('[data-category-create], .category-rename, [data-inline-category-create]');
    if (container) showError(container, '');
  });

  document.addEventListener('keydown', event => {
    if (event.key !== 'Enter' || !event.target.matches('[data-inline-category-name]')) return;
    event.preventDefault();
    void createInlineCategory(event.target.closest('.category-field'));
  });

  document.addEventListener('close', event => {
    if (event.target.id === 'category-dialog') {
      const form = event.target.querySelector('form');
      form.reset();
      showError(form, '');
    }
    if (event.target.id === 'channel-dialog') {
      resetInlineCreate(event.target.querySelector('.category-field'));
    }
  }, true);

  const createInlineCategory = async field => {
    const creator = field.querySelector('[data-inline-category-create]');
    const input = creator.querySelector('[data-inline-category-name]');
    const save = creator.querySelector('[data-inline-category-save]');
    if (save.disabled) return;
    save.disabled = true;
    showError(creator, '');
    try {
      const response = await fetch('/api/categories', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          ...(window.tubyBuildHeaders?.() ?? {}),
        },
        body: new URLSearchParams({name: input.value}),
      });
      window.tubyReloadIfStale?.(response);
      if (response.headers.get('HX-Refresh') === 'true') return;
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        showError(creator, payload.error ?? 'Category operation failed.');
        return;
      }
      addCategoryOptions(payload, field.querySelector('[data-category-select]'));
      resetInlineCreate(field);
      document.dispatchEvent(new Event('tuby:update'));
    } catch {
      showError(creator, 'Category operation failed.');
    } finally {
      save.disabled = false;
    }
  };

  const submitCategory = async form => {
    const submit = form.querySelector('[type="submit"]');
    submit.disabled = true;
    showError(form, '');
    try {
      const response = await fetch(form.action, {
        method: form.matches('[data-category-create]') ? 'POST' : 'PUT',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          ...(window.tubyBuildHeaders?.() ?? {}),
        },
        body: new URLSearchParams(new FormData(form)),
      });
      window.tubyReloadIfStale?.(response);
      if (response.headers.get('HX-Refresh') === 'true') return;
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        showError(form, payload.error ?? 'Category operation failed.');
        return;
      }
      if (form.matches('[data-category-create]')) {
        addCategoryOptions(payload, null);
        form.closest('dialog').close();
      } else {
        finishRename(form.closest('.channel-category'), payload.name);
        updateCategoryOptions(payload);
      }
      document.dispatchEvent(new Event('tuby:update'));
    } catch {
      showError(form, 'Category operation failed.');
    } finally {
      submit.disabled = false;
    }
  };

  const startRename = section => {
    section.classList.add('is-renaming');
    const form = section.querySelector('.category-rename');
    form.hidden = false;
    showError(form, '');
    const input = form.querySelector('input[name="name"]');
    input.focus();
    input.select();
  };

  const cancelRename = section => {
    const form = section.querySelector('.category-rename');
    form.querySelector('input[name="name"]').value = section.querySelector('[data-category-name]').textContent.trim();
    showError(form, '');
    form.hidden = true;
    section.classList.remove('is-renaming');
  };

  const finishRename = (section, name) => {
    section.querySelector('[data-category-name]').textContent = name;
    section.querySelector('.category-rename input[name="name"]').value = name;
    section.querySelector('.category-rename').hidden = true;
    section.classList.remove('is-renaming');
  };

  const startInlineCreate = field => {
    field.querySelector('[data-category-select-row]').hidden = true;
    const creator = field.querySelector('[data-inline-category-create]');
    creator.hidden = false;
    const input = creator.querySelector('[data-inline-category-name]');
    input.disabled = false;
    input.focus();
  };

  const resetInlineCreate = field => {
    if (!field) return;
    field.querySelector('[data-category-select-row]').hidden = false;
    const creator = field.querySelector('[data-inline-category-create]');
    creator.hidden = true;
    const input = creator.querySelector('[data-inline-category-name]');
    input.value = '';
    input.disabled = true;
    showError(creator, '');
  };

  const addCategoryOptions = (category, selectedSelect) => {
    document.querySelectorAll('[data-category-select]').forEach(select => {
      let option = [...select.options].find(value => value.value === String(category.id));
      if (!option) {
        option = document.createElement('option');
        option.value = String(category.id);
        option.textContent = category.name;
        select.append(option);
      }
      if (select === selectedSelect) select.value = option.value;
    });
  };

  const updateCategoryOptions = category => {
    document.querySelectorAll('[data-category-select]').forEach(select => {
      const option = [...select.options].find(value => value.value === String(category.id));
      if (option) option.textContent = category.name;
    });
  };

  const syncCategoryOptions = list => {
    const categories = [...list.querySelectorAll('.channel-category')].map(section => ({
      id: section.dataset.categoryId,
      name: section.querySelector('[data-category-name]').textContent.trim(),
    }));
    const categoryIDs = new Set(categories.map(category => category.id));
    document.querySelectorAll('[data-category-select]').forEach(select => {
      const selected = select.value;
      [...select.options].forEach(option => {
        if (!categoryIDs.has(option.value)) option.remove();
      });
      categories.forEach(category => {
        let option = [...select.options].find(value => value.value === category.id);
        if (!option) {
          option = document.createElement('option');
          option.value = category.id;
        }
        option.textContent = category.name;
        select.append(option);
      });
      if (categoryIDs.has(selected)) select.value = selected;
    });
  };

  const showError = (form, message) => {
    const error = form.querySelector('[data-category-error]');
    error.textContent = message;
    error.hidden = message === '';
  };

  const setCollapsed = (section, collapsed, persist) => {
    if (!section) return;
    section.classList.toggle('is-collapsed', collapsed);
    const toggle = section.querySelector('.category-toggle');
    toggle.setAttribute('aria-expanded', String(!collapsed));
    toggle.querySelector('img').src = collapsed ? toggle.dataset.closedIcon : toggle.dataset.openIcon;
    if (persist) storeCollapsed(section.dataset.categoryId, collapsed);
  };

  const storedCollapsed = categoryID => {
    try {
      return localStorage.getItem(storagePrefix + categoryID) === 'true';
    } catch {
      return false;
    }
  };

  const storeCollapsed = (categoryID, collapsed) => {
    try {
      localStorage.setItem(storagePrefix + categoryID, String(collapsed));
    } catch {
      // The category still works when browser storage is unavailable.
    }
  };

  syncCategories();
  document.addEventListener('htmx:afterSwap', syncCategories);
  document.addEventListener('htmx:historyRestore', syncCategories);
})();
