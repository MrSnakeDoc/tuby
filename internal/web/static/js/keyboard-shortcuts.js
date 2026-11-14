(() => {
  const sequenceTimeout = 1000;
  const prefixes = new Set(['g', 'a', 'c', 'e', 'n']);
  const shortcuts = new Map([
    ['gh', () => navigate('/')],
    ['gm', () => navigate('/missed')],
    ['gw', () => navigate('/watch-later')],
    ['ac', () => document.querySelector('[data-add-channel-button]')?.click()],
    ['ag', () => homeAction(() => document.querySelector('[data-create-category-button]')?.click())],
    ['ca', () => homeAction(() => window.tubySetAllCategories?.(true))],
    ['ea', () => homeAction(() => window.tubySetAllCategories?.(false))],
    ['na', () => document.dispatchEvent(new Event('tuby:request-notification-permission'))],
  ]);
  let prefix = '';
  let timer;

  document.addEventListener('keydown', event => {
    if (event.key === 'Escape') {
      resetSequence();
      return;
    }
    if (event.repeat || event.metaKey || event.ctrlKey || event.altKey) return;
    if (isTyping(event.target) || document.querySelector('dialog[open]')) {
      resetSequence();
      return;
    }
    if (event.key === '/') {
      focusSearch(event);
      return;
    }
    if (event.key === '?') {
      showShortcuts(event);
      return;
    }
    handleSequence(event);
  });

  const handleSequence = event => {
    const key = event.key.toLowerCase();
    if (prefix === '') {
      if (prefixes.has(key)) startSequence(event, key);
      return;
    }
    const action = shortcuts.get(prefix + key);
    resetSequence();
    if (action) {
      event.preventDefault();
      action();
      return;
    }
    if (prefixes.has(key)) startSequence(event, key);
  };

  const startSequence = (event, key) => {
    event.preventDefault();
    prefix = key;
    clearTimeout(timer);
    timer = setTimeout(resetSequence, sequenceTimeout);
  };

  const resetSequence = () => {
    prefix = '';
    clearTimeout(timer);
  };

  const focusSearch = event => {
    const input = document.querySelector('#channel-search');
    if (!input) return;
    event.preventDefault();
    resetSequence();
    input.focus();
  };

  const showShortcuts = event => {
    const dialog = document.querySelector('#shortcuts-dialog');
    if (!dialog) return;
    event.preventDefault();
    resetSequence();
    dialog.showModal();
  };

  const navigate = path => {
    document.querySelector(`.dock-link[href="${path}"]`)?.click();
  };

  const homeAction = action => {
    if (!document.querySelector('.category-list')) return;
    action();
  };

  const isTyping = target => target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement || target?.isContentEditable;
})();
