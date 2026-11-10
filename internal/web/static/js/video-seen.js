(() => {
  document.addEventListener('click', event => {
    const link = event.target.closest('.video-link, .video-title');
    const card = link?.closest('.video-card[data-unseen="true"]');
    if (!card) return;
    fetch(`/api/videos/${card.dataset.videoId}/seen`, {
      method: 'POST', keepalive: true, headers: window.tubyBuildHeaders?.() ?? {},
    })
      .then(response => window.tubyReloadIfStale?.(response) ?? response)
      .then(response => { if (response.ok) markSeen(card); })
      .catch(() => {});
  });

  function markSeen(card) {
    card.dataset.unseen = 'false';
    card.querySelector('.seen-button')?.remove();
    card.querySelector('.new-pill')?.remove();
    if (location.pathname === '/missed') card.remove();
    updateMissedCount();
  }

  function updateMissedCount() {
    const badge = document.querySelector('#missed-count');
    if (!badge) return;
    const count = Number(badge.textContent) - 1;
    badge.textContent = String(Math.max(0, count));
    badge.hidden = count <= 0;
  }
})();
