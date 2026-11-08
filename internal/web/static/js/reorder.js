(() => {
  const grid = document.querySelector('.channel-grid');
  const actions = document.querySelector('.top-actions');
  if (!grid || !actions || actions.querySelector('[data-reorder]')) return;
  const button = document.createElement('button');
  button.className = 'icon-button'; button.dataset.reorder = ''; button.innerHTML = '<img src="/static/img/pencil.svg" alt="">'; actions.append(button);
  let editing = false; let dragged; let placeholder;
  const finishDrag = () => { if (dragged && placeholder && placeholder.parentNode) { grid.replaceChild(dragged, placeholder); dragged.classList.remove('is-dragging'); } dragged = undefined; placeholder = undefined; };
  const setEditing = value => { editing = value; grid.classList.toggle('is-reordering', value); grid.querySelectorAll('.channel-card').forEach(card => { card.draggable = value; }); button.innerHTML = `<img src="/static/img/${value ? 'lock' : 'pencil'}.svg" alt="">`; };
  button.onclick = async () => { if (!editing) return setEditing(true); finishDrag(); const ids = [...grid.children].filter(card => card.classList.contains('channel-card')).map(card => card.dataset.channelId); const response = await fetch('/api/channels/order', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({channel_ids:ids})}); if (response.ok) setEditing(false); };
  grid.addEventListener('dragstart', event => { if (!editing) return event.preventDefault(); dragged = event.target.closest('.channel-card'); placeholder = document.createElement('div'); placeholder.className = 'channel-placeholder'; placeholder.style.height = `${dragged.offsetHeight}px`; grid.replaceChild(placeholder, dragged); dragged.classList.add('is-dragging'); document.body.append(dragged); });
  grid.addEventListener('dragend', finishDrag);
  grid.addEventListener('drop', event => { event.preventDefault(); finishDrag(); });
  grid.addEventListener('dragover', event => { if (!editing || !dragged) return; event.preventDefault(); const target = event.target.closest('.channel-card'); if (!target) return; const box = target.getBoundingClientRect(); const after = event.clientY > box.top + box.height / 2 || (Math.abs(event.clientY - (box.top + box.height / 2)) < box.height / 3 && event.clientX > box.left + box.width / 2); const before = after ? target.nextSibling : target; if (placeholder.nextSibling !== before) grid.insertBefore(placeholder, before); });
})();
