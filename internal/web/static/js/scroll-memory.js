(() => {
  const homeScrollKey = 'tuby:home-scroll';
  let openChannel = false;

  document.addEventListener('click', event => {
    const link = event.target.closest('a[href^="/channels/"]');
    if (!link) return;
    sessionStorage.setItem(homeScrollKey, String(window.scrollY));
    openChannel = true;
  });

  document.addEventListener('htmx:afterSwap', () => {
    if (!openChannel) return;
    openChannel = false;
    window.scrollTo({top: 0, behavior: 'instant'});
  });

  if (location.pathname === '/') {
    const saved = sessionStorage.getItem(homeScrollKey);
    if (saved) window.scrollTo({top: Number(saved), behavior: 'instant'});
  }
})();
