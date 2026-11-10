(() => {
  let timer;
  let pending = false;
  let refreshing = false;
  let lastRevision = -1;
  let latestUpdate;
  let reloading = false;
  const deletedChannels = new Set();
  const documentBuild = document.querySelector('meta[name="tuby-build"]')?.content ?? '';

  window.tubyBuildHeaders = () => ({'X-Tuby-Build': documentBuild});
  window.tubyReloadIfStale = response => {
    if (response.headers.get('HX-Refresh') === 'true') reloadPage();
    return response;
  };

  document.addEventListener('htmx:configRequest', event => {
    Object.assign(event.detail.headers, window.tubyBuildHeaders());
  });

  const source = new EventSource('/api/events');
  source.addEventListener('update', event => {
    let update;
    try {
      update = JSON.parse(event.data);
    } catch {
      return;
    }
    if (update.build_id !== documentBuild) {
      reloadPage();
      return;
    }
    if (update.reason === 'sync') {
      lastRevision = update.revision;
    } else {
      if (update.revision <= lastRevision) return;
      lastRevision = update.revision;
    }
    latestUpdate = update;
    if (update.reason === 'channel.deleted') {
      update.channel_ids?.forEach(channelID => deletedChannels.add(channelID));
    }
    scheduleRefresh();
  });

  const reloadPage = () => {
    if (reloading) return;
    reloading = true;
    source.close();
    window.location.reload();
  };

  const scheduleRefresh = () => {
    pending = true;
    clearTimeout(timer);
    timer = setTimeout(refresh, 200);
  };

  const refresh = async () => {
    if (!pending || refreshing || document.hidden || document.querySelector('.is-reordering') || document.querySelector('dialog[open]')) return;
    pending = false;
    refreshing = true;
    const search = document.querySelector('#channel-search')?.value ?? '';
    const scrollY = window.scrollY;
    const update = latestUpdate;
    let failed = false;
    try {
      if (deletedCurrentChannel(update)) {
        await htmx.ajax('GET', '/', {target: '#app', swap: 'innerHTML', push: '/'});
      } else {
        await htmx.ajax('GET', location.pathname, {target: '#app', swap: 'innerHTML'});
      }
      restoreView(search, scrollY);
      deletedChannels.clear();
    } catch {
      failed = true;
      pending = true;
    } finally {
      refreshing = false;
      if (pending) {
        clearTimeout(timer);
        timer = setTimeout(refresh, failed ? 1000 : 200);
      }
    }
  };

  const deletedCurrentChannel = update => {
    if (!location.pathname.startsWith('/channels/')) return false;
    const channelID = location.pathname.slice('/channels/'.length);
    return deletedChannels.has(channelID) ||
      (update?.reason === 'channel.deleted' && update.channel_ids?.includes(channelID));
  };

  const restoreView = (search, scrollY) => {
    const input = document.querySelector('#channel-search');
    if (input && search) {
      input.value = search;
      window.tubyFilterChannels(input);
    }
    window.scrollTo({top: scrollY, behavior: 'instant'});
  };

  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) refresh();
  });
  document.addEventListener('tuby:update', scheduleRefresh);
  document.addEventListener('tuby:reorder-finished', refresh);
  document.addEventListener('close', refresh, true);
})();
