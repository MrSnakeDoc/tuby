(() => {
  const rotationInterval = 15 * 60 * 1000;

  const rotateThumbnails = () => {
    document.querySelectorAll('[data-thumbnail-urls]').forEach(rotateThumbnail);
  };

  const rotateThumbnail = image => {
    let urls;
    try {
      urls = JSON.parse(image.dataset.thumbnailUrls);
    } catch {
      return;
    }
    if (!Array.isArray(urls) || urls.length === 0) return;
    const channelID = image.dataset.channelId ?? '';
    const hash = [...channelID].reduce((value, character) => value + character.charCodeAt(0), 0);
    image.src = urls[(Math.floor(Date.now() / rotationInterval) + hash) % urls.length];
  };

  rotateThumbnails();
  setInterval(rotateThumbnails, rotationInterval);
  document.addEventListener('htmx:afterSwap', rotateThumbnails);
  document.addEventListener('htmx:historyRestore', rotateThumbnails);
})();
