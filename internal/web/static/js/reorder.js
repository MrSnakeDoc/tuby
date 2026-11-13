(() => {
  const movementUnavailable = window.matchMedia('(max-width: 850px), (pointer: coarse)');

  const syncReorder = () => {
    const actions = document.querySelector('.top-actions');
    const list = document.querySelector('.category-list');
    const existing = actions?.querySelector('[data-reorder]');
    if (!actions || !list) {
      existing?.remove();
      if (actions) setPageActionsDisabled(actions, null, false);
      return;
    }
    if (existing?.reorderList === list) return;
    existing?.remove();
    mountReorder(list, actions);
  };

  const mountReorder = (list, actions) => {
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    const button = document.createElement('button');
    button.className = 'icon-button';
    button.type = 'button';
    button.dataset.reorder = '';
    button.reorderList = list;
    button.title = 'Reorder categories and channels';
    actions.append(button);

    const state = {
      editing: false,
      collapsed: new Set(),
      dragged: null,
      placeholder: null,
      kind: '',
      dragIntent: '',
      pointerX: 0,
      pointerY: 0,
      scrollFrame: 0,
      scrollVelocity: 0,
      categoryMode: null,
      categoryWasCollapsed: false,
      compactChannelDrag: false,
      pointerID: null,
      touchPreview: null,
    };
    updateEmptyStates(list);
    setButtonIcon(button, build, false);

    button.addEventListener('click', () => {
      if (movementUnavailable.matches && !state.editing) return;
      if (!state.editing) {
        enterEditMode(list, actions, button, build, state);
        return;
      }
      void saveOrder(list, actions, button, build, state);
    });
    list.addEventListener('click', event => {
      const trigger = event.target.closest('[data-category-channel-order]');
      if (!trigger) return;
      if (movementUnavailable.matches && !state.editing) return;
      const section = trigger.closest('.channel-category');
      if (!state.editing) {
        enterCategoryEditMode(list, actions, section, trigger, build, state);
        return;
      }
      if (state.categoryMode === section) void saveCategoryOrder(list, actions, section, trigger, build, state);
    });
    list.addEventListener('pointerdown', event => {
      state.dragIntent = dragIntent(event.target);
      if (state.editing && event.pointerType !== 'mouse') startPointerDrag(event, list, state);
    });
    list.addEventListener('pointermove', event => movePointerDrag(event, list, state));
    list.addEventListener('pointerup', event => finishPointerDrag(event, list, state));
    list.addEventListener('pointercancel', event => finishPointerDrag(event, list, state));
    list.addEventListener('keydown', event => moveWithKeyboard(event, list, state));
    list.addEventListener('dragstart', event => startDrag(event, list, state));
    list.addEventListener('dragover', event => movePlaceholder(event, list, state));
    list.addEventListener('drop', event => {
      event.preventDefault();
      finishDrag(state);
    });
  };

  const enterEditMode = (list, actions, button, build, state) => {
    clearSearch();
    cancelRenames(list);
    state.editing = true;
    state.collapsed = new Set(
      [...list.querySelectorAll('.channel-category.is-collapsed')].map(section => section.dataset.categoryId),
    );
    list.querySelectorAll('.channel-category').forEach(section => setCategoryExpanded(section, true));
    setDraggable(list, true);
    list.classList.add('is-reordering');
    setListActionsDisabled(list, null, true);
    setPageActionsDisabled(actions, button, true);
    setButtonIcon(button, build, true);
  };

  const leaveEditMode = (list, actions, button, build, state) => {
    finishDrag(state);
    setDraggable(list, false);
    list.classList.remove('is-reordering');
    setListActionsDisabled(list, null, false);
    updateEmptyStates(list);
    list.querySelectorAll('.channel-category').forEach(section => {
      setCategoryExpanded(section, !state.collapsed.has(section.dataset.categoryId));
    });
    setPageActionsDisabled(actions, button, false);
    setButtonIcon(button, build, false);
    state.editing = false;
  };

  const enterCategoryEditMode = (list, actions, section, trigger, build, state) => {
    clearSearch();
    cancelRenames(list);
    state.editing = true;
    state.categoryMode = section;
    state.categoryWasCollapsed = section.classList.contains('is-collapsed');
    setCategoryExpanded(section, true);
    section.querySelectorAll('.channel-card').forEach(card => {
      setReorderable(card, true);
    });
    section.classList.add('is-active-category-reorder');
    list.classList.add('is-reordering', 'is-category-reordering');
    setListActionsDisabled(list, trigger, true);
    setPageActionsDisabled(actions, null, true);
    setCategoryButtonIcon(trigger, build, true);
  };

  const leaveCategoryEditMode = (list, actions, section, trigger, build, state) => {
    finishDrag(state);
    section.querySelectorAll('.channel-card').forEach(card => {
      setReorderable(card, false);
    });
    list.classList.remove('is-reordering', 'is-category-reordering');
    section.classList.remove('is-active-category-reorder');
    setListActionsDisabled(list, trigger, false);
    setCategoryExpanded(section, !state.categoryWasCollapsed);
    updateEmptyStates(list);
    setPageActionsDisabled(actions, null, false);
    setCategoryButtonIcon(trigger, build, false);
    state.editing = false;
    state.categoryMode = null;
  };

  const saveOrder = async (list, actions, button, build, state) => {
    finishDrag(state);
    button.disabled = true;
    try {
      const response = await fetch('/api/categories/order', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', ...(window.tubyBuildHeaders?.() ?? {})},
        body: JSON.stringify(libraryOrder(list)),
      });
      window.tubyReloadIfStale?.(response);
      if (!response.ok) {
        showSaveFailure(await saveErrorMessage(response));
        return;
      }
      leaveEditMode(list, actions, button, build, state);
      document.dispatchEvent(new Event('tuby:reorder-finished'));
    } catch {
      showSaveFailure('Tuby could not reach the server. Your changes are still editable.');
    } finally {
      button.disabled = false;
    }
  };

  const saveCategoryOrder = async (list, actions, section, trigger, build, state) => {
    finishDrag(state);
    trigger.disabled = true;
    try {
      const response = await persistOrder(list);
      window.tubyReloadIfStale?.(response);
      if (!response.ok) {
        showSaveFailure(await saveErrorMessage(response));
        return;
      }
      leaveCategoryEditMode(list, actions, section, trigger, build, state);
      document.dispatchEvent(new Event('tuby:reorder-finished'));
    } catch {
      showSaveFailure('Tuby could not reach the server. Your changes are still editable.');
    } finally {
      trigger.disabled = false;
    }
  };

  const persistOrder = list => fetch('/api/categories/order', {
    method: 'POST',
    headers: {'Content-Type': 'application/json', ...(window.tubyBuildHeaders?.() ?? {})},
    body: JSON.stringify(libraryOrder(list)),
  });

  const libraryOrder = list => ({
    categories: [...list.querySelectorAll(':scope > .channel-category')].map(section => ({
      id: Number(section.dataset.categoryId),
      channel_ids: [...section.querySelectorAll('.channel-grid > .channel-card')].map(card => card.dataset.channelId),
    })),
  });

  const startDrag = (event, list, state) => {
    if (!state.editing) {
      event.preventDefault();
      return;
    }
    const target = dragTarget(event.target, state);
    if (!target) {
      event.preventDefault();
      resetDragState(state);
      return;
    }
    state.kind = target.kind;
    state.dragged = target.element;
    createNativeDragPreview(event, state.dragged, state.kind);
    prepareDrag(list, state);
    event.dataTransfer.effectAllowed = 'move';
    state.dragged.addEventListener('dragend', () => finishDrag(state), {once: true});
  };

  const startPointerDrag = (event, list, state) => {
    if (state.pointerID !== null) return;
    const target = dragTarget(event.target, state);
    if (!target) return;
    event.preventDefault();
    state.kind = target.kind;
    state.dragged = target.element;
    state.pointerID = event.pointerId;
    state.touchPreview = createTouchDragPreview(state.dragged, state.kind, event.clientX, event.clientY);
    prepareDrag(list, state);
    list.setPointerCapture(event.pointerId);
  };

  const prepareDrag = (list, state) => {
    state.compactChannelDrag = state.kind === 'channel' && state.categoryMode === null;
    if (state.kind === 'category') list.classList.add('is-dragging-category');
    if (state.compactChannelDrag) list.classList.add('is-dragging-channel');
    state.placeholder = createPlaceholder(state.dragged, state.kind, state.compactChannelDrag);
    state.dragged.before(state.placeholder);
    state.dragged.classList.add('is-dragging');
    setTimeout(() => {
      if (state.dragged) document.body.append(state.dragged);
    });
    list.classList.add('has-active-drag');
  };

  const movePointerDrag = (event, list, state) => {
    if (state.pointerID !== event.pointerId || !state.dragged) return;
    event.preventDefault();
    positionTouchPreview(state.touchPreview, event.clientX, event.clientY);
    updateAutoScroll(event.clientX, event.clientY, list, state);
    const target = document.elementFromPoint(event.clientX, event.clientY);
    placePlaceholder(target, event.clientX, event.clientY, list, state);
  };

  const finishPointerDrag = (event, list, state) => {
    if (state.pointerID !== event.pointerId) return;
    if (list.hasPointerCapture(event.pointerId)) list.releasePointerCapture(event.pointerId);
    finishDrag(state);
  };

  const movePlaceholder = (event, list, state) => {
    if (!state.dragged || !state.placeholder) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    updateAutoScroll(event.clientX, event.clientY, list, state);
    placePlaceholder(event.target, event.clientX, event.clientY, list, state);
  };

  const placePlaceholder = (target, clientX, clientY, list, state) => {
    if (state.kind === 'category') {
      moveCategoryPlaceholder(target, clientY, list, state.placeholder);
      return;
    }
    moveChannelPlaceholder(target, clientX, clientY, list, state.placeholder);
  };

  const moveCategoryPlaceholder = (element, clientY, list, placeholder) => {
    const category = element?.closest('.channel-category');
    if (category && category.parentElement === list) {
      const box = category.getBoundingClientRect();
      list.insertBefore(placeholder, clientY > box.top + box.height / 2 ? category.nextSibling : category);
      return;
    }
    const categories = [...list.querySelectorAll(':scope > .channel-category')];
    const lastCategory = categories[categories.length - 1];
    if (lastCategory && clientY > lastCategory.getBoundingClientRect().bottom) list.append(placeholder);
  };

  const moveChannelPlaceholder = (element, clientX, clientY, list, placeholder) => {
    const section = element?.closest('.channel-category');
    if (!section || !list.contains(section)) return;
    const grid = section.querySelector('.channel-grid');
    if (element.closest('.channel-placeholder')) return;
    const target = element.closest('.channel-card');
    if (!target || target.parentElement !== grid) {
      if (placeholder.parentElement !== grid) grid.append(placeholder);
      return;
    }
    const box = target.getBoundingClientRect();
    const after = clientY > box.top + box.height / 2 ||
      (Math.abs(clientY - (box.top + box.height / 2)) < box.height / 3 && clientX > box.left + box.width / 2);
    const reference = after ? target.nextSibling : target;
    if (reference === placeholder || placeholder.nextSibling === reference) return;
    grid.insertBefore(placeholder, reference);
  };

  const updateAutoScroll = (clientX, clientY, list, state) => {
    state.pointerX = clientX;
    state.pointerY = clientY;
    state.scrollVelocity = autoScrollVelocity(clientY);
    if (state.scrollVelocity === 0) {
      stopAutoScroll(state);
      return;
    }
    if (state.scrollFrame === 0) state.scrollFrame = requestAnimationFrame(() => autoScroll(list, state));
  };

  const autoScroll = (list, state) => {
    state.scrollFrame = 0;
    if (!state.dragged || state.scrollVelocity === 0) return;
    window.scrollBy(0, state.scrollVelocity);
    const target = document.elementFromPoint(state.pointerX, state.pointerY);
    placePlaceholder(target, state.pointerX, state.pointerY, list, state);
    state.scrollFrame = requestAnimationFrame(() => autoScroll(list, state));
  };

  const autoScrollVelocity = clientY => {
    const edge = Math.min(150, window.innerHeight * 0.22);
    if (clientY < edge) return -Math.ceil(4 + 20 * ((edge - clientY) / edge));
    if (clientY > window.innerHeight - edge) {
      return Math.ceil(4 + 20 * ((clientY - (window.innerHeight - edge)) / edge));
    }
    return 0;
  };

  const stopAutoScroll = state => {
    if (state.scrollFrame !== 0) cancelAnimationFrame(state.scrollFrame);
    state.scrollFrame = 0;
    state.scrollVelocity = 0;
  };

  const finishDrag = state => {
    stopAutoScroll(state);
    if (state.dragged && state.placeholder?.parentNode) {
      state.placeholder.replaceWith(state.dragged);
      state.dragged.classList.remove('is-dragging');
    }
    document.querySelector('.category-list')?.classList.remove(
      'has-active-drag',
      'is-dragging-category',
      'is-dragging-channel',
    );
    resetDragState(state);
  };

  const resetDragState = state => {
    state.touchPreview?.remove();
    state.dragged = null;
    state.placeholder = null;
    state.kind = '';
    state.dragIntent = '';
    state.compactChannelDrag = false;
    state.pointerID = null;
    state.touchPreview = null;
  };

  const createPlaceholder = (dragged, kind, compact) => {
    const placeholder = document.createElement('div');
    placeholder.className = `${kind}-placeholder`;
    placeholder.style.height = compact ? '54px' : `${dragged.offsetHeight}px`;
    return placeholder;
  };

  const createNativeDragPreview = (event, dragged, kind) => {
    const preview = dragged.cloneNode(true);
    preview.className = `${dragged.className} drag-preview ${kind}-drag-preview`;
    preview.style.width = `${dragged.getBoundingClientRect().width}px`;
    document.body.append(preview);
    event.dataTransfer.setDragImage(preview, Math.min(event.offsetX, preview.offsetWidth / 2), 24);
    requestAnimationFrame(() => preview.remove());
  };

  const createTouchDragPreview = (dragged, kind, clientX, clientY) => {
    const preview = dragged.cloneNode(true);
    preview.className = `${dragged.className} touch-drag-preview ${kind}-touch-preview`;
    preview.style.width = `${Math.min(dragged.getBoundingClientRect().width, window.innerWidth - 32)}px`;
    document.body.append(preview);
    positionTouchPreview(preview, clientX, clientY);
    return preview;
  };

  const positionTouchPreview = (preview, clientX, clientY) => {
    if (!preview) return;
    const width = preview.getBoundingClientRect().width;
    const left = Math.max(16, Math.min(window.innerWidth - width - 16, clientX - width / 2));
    preview.style.left = `${left}px`;
    preview.style.top = `${Math.max(12, clientY - 28)}px`;
  };

  const dragTarget = (element, state) => {
    const card = element.closest('.channel-card');
    if (card) return {kind: 'channel', element: card};
    if (state.categoryMode !== null || state.dragIntent !== 'category') return null;
    const category = element.closest('.channel-category');
    return category ? {kind: 'category', element: category} : null;
  };

  const setDraggable = (list, enabled) => {
    list.querySelectorAll('.channel-category, .channel-card').forEach(element => {
      setReorderable(element, enabled);
    });
  };

  const setReorderable = (element, enabled) => {
    element.draggable = enabled;
    if (!enabled) {
      element.removeAttribute('tabindex');
      element.removeAttribute('aria-label');
      return;
    }
    element.tabIndex = 0;
    const name = element.querySelector('.channel-name, [data-category-name]')?.textContent?.trim() ?? '';
    element.setAttribute('aria-label', `Move ${name} with the arrow keys`);
  };

  const moveWithKeyboard = (event, list, state) => {
    if (!state.editing || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
    if (event.target.matches('.channel-card')) {
      event.preventDefault();
      moveChannelWithKeyboard(event, list, state);
      return;
    }
    if (event.target.matches('.channel-category') && state.categoryMode === null) {
      event.preventDefault();
      moveCategoryWithKeyboard(event.target, event.key);
    }
  };

  const moveChannelWithKeyboard = (event, list, state) => {
    const card = event.target;
    const backwards = event.key === 'ArrowLeft' || event.key === 'ArrowUp';
    if (event.ctrlKey && state.categoryMode === null) {
      moveChannelToAdjacentCategory(card, list, backwards ? -1 : 1);
      card.focus();
      return;
    }
    const cards = [...card.parentElement.querySelectorAll(':scope > .channel-card')];
    const index = cards.indexOf(card);
    const sibling = cards[index + (backwards ? -1 : 1)];
    if (sibling) sibling[backwards ? 'before' : 'after'](card);
    card.focus();
  };

  const moveChannelToAdjacentCategory = (card, list, offset) => {
    const sections = [...list.querySelectorAll(':scope > .channel-category')];
    const current = sections.indexOf(card.closest('.channel-category'));
    const target = sections[current + offset];
    target?.querySelector('.channel-grid').append(card);
  };

  const moveCategoryWithKeyboard = (category, key) => {
    const backwards = key === 'ArrowLeft' || key === 'ArrowUp';
    const categories = [...category.parentElement.querySelectorAll(':scope > .channel-category')];
    const index = categories.indexOf(category);
    const sibling = categories[index + (backwards ? -1 : 1)];
    if (sibling) sibling[backwards ? 'before' : 'after'](category);
    category.focus();
  };

  const updateEmptyStates = list => {
    list.querySelectorAll('.channel-category').forEach(section => {
      const empty = section.querySelector('.channel-grid').children.length === 0;
      section.querySelector('.category-empty').hidden = !empty;
      section.classList.toggle('is-empty-default', empty && section.dataset.categoryId === '1');
    });
  };

  const setCategoryExpanded = (section, expanded) => {
    section.classList.toggle('is-collapsed', !expanded);
    const toggle = section.querySelector('.category-toggle');
    toggle.setAttribute('aria-expanded', String(expanded));
    toggle.querySelector('img').src = expanded ? toggle.dataset.openIcon : toggle.dataset.closedIcon;
  };

  const setPageActionsDisabled = (actions, reorder, disabled) => {
    actions.querySelectorAll('button').forEach(button => {
      if (button !== reorder) button.disabled = disabled;
    });
    const search = document.querySelector('#channel-search');
    if (search) search.disabled = disabled;
    document.body.classList.toggle('is-editing-order', disabled);
    document.querySelectorAll('.brand, .dock-link').forEach(link => {
      if (disabled) {
        link.tabIndex = -1;
        link.setAttribute('aria-disabled', 'true');
        return;
      }
      link.removeAttribute('tabindex');
      link.removeAttribute('aria-disabled');
    });
  };

  const setListActionsDisabled = (list, exempt, disabled) => {
    list.querySelectorAll('button').forEach(button => {
      if (button !== exempt) button.disabled = disabled;
    });
    list.querySelectorAll('.channel-card a').forEach(link => {
      if (disabled) {
        link.tabIndex = -1;
        link.setAttribute('aria-disabled', 'true');
        return;
      }
      link.removeAttribute('tabindex');
      link.removeAttribute('aria-disabled');
    });
  };

  const setButtonIcon = (button, build, editing) => {
    const icon = editing ? 'lock' : 'pencil';
    button.innerHTML = `<img src="/static/img/${icon}.svg?v=${encodeURIComponent(build)}" alt="">`;
    button.title = editing ? 'Save category and channel order' : 'Reorder categories and channels';
    button.setAttribute('aria-label', button.title);
  };

  const setCategoryButtonIcon = (button, build, editing) => {
    const icon = editing ? 'lock' : 'arrow-left-right';
    button.innerHTML = `<img src="/static/img/${icon}.svg?v=${encodeURIComponent(build)}" alt="">`;
    button.title = editing ? 'Save channel order' : 'Reorder channels in this category';
    button.setAttribute('aria-label', button.title);
  };

  const clearSearch = () => {
    const search = document.querySelector('#channel-search');
    if (!search) return;
    search.value = '';
    window.tubyFilterChannels?.(search);
    search.blur();
  };

  const cancelRenames = list => {
    list.querySelectorAll('.channel-category.is-renaming [data-category-cancel]').forEach(button => button.click());
  };

  const saveErrorMessage = async response => {
    const message = await (window.tubyFetchErrorMessage?.(
      response,
      'Could not save this order.',
    ) ?? Promise.resolve('Could not save this order.'));
    return `${message} Your changes are still editable.`;
  };

  const showSaveFailure = message => window.tubyShowError?.(message);

  const dragIntent = target => {
    if (target.closest('.channel-card')) return 'channel';
    if (target.closest('.category-heading')) return 'category';
    return '';
  };

  syncReorder();
  document.addEventListener('htmx:afterSwap', syncReorder);
  document.addEventListener('htmx:historyRestore', syncReorder);
})();
