(() => {
  document.addEventListener('click', event => {
    const link = event.target.closest('.video-link, .video-title');
    const card = link?.closest('.video-card[data-unseen="true"]');
    if (!card) return;
    fetch(`/api/videos/${card.dataset.videoId}/seen`, {
      method: 'POST', keepalive: true, headers: window.tubyBuildHeaders?.() ?? {},
    })
      .then(response => window.tubyReloadIfStale?.(response) ?? response)
      .then(async response => {
        if (response.ok) {
          markSeen(card);
          return;
        }
        const message = await (window.tubyFetchErrorMessage?.(
          response,
          'Tuby could not mark this video as watched.',
        ) ?? Promise.resolve('Tuby could not mark this video as watched.'));
        window.tubyShowError?.(message);
      })
      .catch(() => window.tubyShowError?.('Tuby could not reach the server. Your watched state was not saved.'));
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
    if (badge.textContent.trim().endsWith('+')) return;
    const count = Number(badge.textContent) - 1;
    badge.textContent = String(Math.max(0, count));
    badge.hidden = count <= 0;
  }
})();
