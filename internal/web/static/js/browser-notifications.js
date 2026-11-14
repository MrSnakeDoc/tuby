(() => {
  const lockName = 'tuby-browser-notification-leader';
  let permissionGranted = false;
  let leader = !navigator.locks;
  let registration;

  const initialize = async () => {
    if (!('Notification' in window)) return;
    if (Notification.permission === 'granted') {
      await activate();
      return;
    }
    if (Notification.permission === 'denied') return;
    const permission = await requestPermission();
    if (permission === 'granted') {
      await activate();
    }
  };

  const requestPermission = async () => {
    try {
      return await Notification.requestPermission();
    } catch {
      return Notification.permission;
    }
  };

  const activate = async () => {
    if (permissionGranted) return;
    permissionGranted = true;
    if ('serviceWorker' in navigator) registration = await registerWorker();
    electLeader();
  };

  const registerWorker = async () => {
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    try {
      return await navigator.serviceWorker.register(
        `/static/js/browser-notification-worker.js?v=${encodeURIComponent(build)}`,
      );
    } catch {
      return undefined;
    }
  };

  const electLeader = () => {
    if (!navigator.locks) return;
    navigator.locks.request(lockName, async () => {
      leader = true;
      await pageLifetime();
      leader = false;
    }).catch(() => {
      leader = true;
    });
  };

  const pageLifetime = () => new Promise(resolve => {
    window.addEventListener('pagehide', resolve, {once: true});
  });

  const notify = async value => {
    if (!permissionGranted || !leader) return;
    const options = {
      body: value.video_title,
      icon: value.thumbnail_url || '/static/img/favicon.svg',
      tag: `tuby-video-${value.video_id}`,
      data: {videoID: value.video_id},
    };
    try {
      if (registration) {
        await registration.showNotification(value.channel_title, options);
        return;
      }
      const notification = new Notification(value.channel_title, options);
      notification.addEventListener('click', () => window.focus());
    } catch {
      // Browser notifications are optional and must never affect live updates.
    }
  };

  document.addEventListener('tuby:sse-update', event => {
    event.detail.browser_notifications?.forEach(value => void notify(value));
  });

  document.addEventListener('tuby:request-notification-permission', async () => {
    if (!('Notification' in window)) {
      window.tubyShowError?.('This browser does not support notifications.');
      return;
    }
    if (Notification.permission === 'denied') {
      window.tubyShowError?.('Browser notifications are blocked. Allow them in the site permissions.');
      return;
    }
    if (Notification.permission === 'granted') {
      await activate();
      window.tubyShowSuccess?.('Browser notifications are enabled.');
      return;
    }
    if (await requestPermission() === 'granted') {
      await activate();
      window.tubyShowSuccess?.('Browser notifications are enabled.');
      return;
    }
    window.tubyShowError?.('The browser did not grant notification permission.');
  });

  void initialize();
})();
