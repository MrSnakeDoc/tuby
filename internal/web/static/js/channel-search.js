(() => {
  const normalize = value => value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();

  window.tubyFilterChannels = input => {
    const grid = document.querySelector('.channel-grid');
    const empty = document.querySelector('#channel-search-empty');
    if (!grid || !empty) return;
    const query = normalize(input.value.trim());
    let visible = 0;
    grid.querySelectorAll('.channel-card').forEach(card => {
      const name = card.querySelector('.channel-name')?.textContent ?? '';
      const matches = normalize(name).includes(query);
      card.style.setProperty('display', matches ? 'block' : 'none', 'important');
      if (matches) visible++;
    });
    const showEmpty = visible === 0 && query !== '';
    empty.hidden = !showEmpty;
    empty.style.setProperty('display', showEmpty ? 'flex' : 'none', 'important');
  };

  document.addEventListener('keydown', event => {
    if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey && !isTyping(event.target)) {
      const input = document.querySelector('#channel-search');
      if (input) {
        event.preventDefault();
        input.focus();
      }
      return;
    }

    if (event.key !== 'Escape' || event.target.id !== 'channel-search') return;
    event.target.value = '';
    window.tubyFilterChannels(event.target);
    event.target.blur();
  });

  function isTyping(target) {
    return target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target?.isContentEditable;
  }
})();
