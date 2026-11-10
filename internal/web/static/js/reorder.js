(() => {
  const syncReorder = () => {
    const actions = document.querySelector('.top-actions');
    if (!actions) return;

    const existing = actions.querySelector('[data-reorder]');
    const grid = document.querySelector('.channel-grid');
    if (!grid) {
      existing?.remove();
      return;
    }
    if (existing?.reorderGrid === grid) return;

    existing?.remove();
    mountReorder(grid, actions);
  };

  const mountReorder = (grid, actions) => {
    const button = document.createElement('button');
    button.className = 'icon-button';
    button.dataset.reorder = '';
    button.reorderGrid = grid;
    button.title = 'Reorder channels';
    button.innerHTML = '<img src="/static/img/pencil.svg" alt="">';
    actions.append(button);

    let editing = false;
    let dragged;
    let placeholder;

    const finishDrag = () => {
      if (dragged && placeholder?.parentNode) {
        grid.replaceChild(dragged, placeholder);
        dragged.classList.remove('is-dragging');
      }
      dragged = undefined;
      placeholder = undefined;
    };

    const setEditing = value => {
      editing = value;
      grid.classList.toggle('is-reordering', value);
      grid.querySelectorAll('.channel-card').forEach(card => {
        card.draggable = value;
      });
      button.innerHTML = `<img src="/static/img/${value ? 'lock' : 'pencil'}.svg" alt="">`;
    };

    button.addEventListener('click', async () => {
      if (!editing) {
        setEditing(true);
        return;
      }
      finishDrag();
      const channelIDs = [...grid.children]
        .filter(card => card.classList.contains('channel-card'))
        .map(card => card.dataset.channelId);
      const response = await fetch('/api/channels/order', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({channel_ids: channelIDs}),
      });
      if (response.ok) {
        setEditing(false);
        document.dispatchEvent(new Event('tuby:reorder-finished'));
      }
    });

    grid.addEventListener('dragstart', event => {
      if (!editing) {
        event.preventDefault();
        return;
      }
      dragged = event.target.closest('.channel-card');
      placeholder = document.createElement('div');
      placeholder.className = 'channel-placeholder';
      placeholder.style.height = `${dragged.offsetHeight}px`;
      grid.replaceChild(placeholder, dragged);
      dragged.classList.add('is-dragging');
      dragged.addEventListener('dragend', finishDrag, {once: true});
      document.body.append(dragged);
    });

    grid.addEventListener('drop', event => {
      event.preventDefault();
      finishDrag();
    });

    grid.addEventListener('dragover', event => {
      if (!editing || !dragged) return;
      event.preventDefault();
      const target = event.target.closest('.channel-card');
      if (!target) return;
      const box = target.getBoundingClientRect();
      const after = event.clientY > box.top + box.height / 2 ||
        (Math.abs(event.clientY - (box.top + box.height / 2)) < box.height / 3 &&
          event.clientX > box.left + box.width / 2);
      const before = after ? target.nextSibling : target;
      if (placeholder.nextSibling !== before) grid.insertBefore(placeholder, before);
    });
  };

  syncReorder();
  document.addEventListener('htmx:afterSwap', syncReorder);
  document.addEventListener('htmx:historyRestore', syncReorder);
})();
