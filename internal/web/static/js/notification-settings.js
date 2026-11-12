(() => {
  const levels = {
    off: {icon: 'cloud-hail', label: 'Off'},
    low: {icon: 'bell-electric', label: 'Wake Me Up Before You Go-Go!'},
    medium: {icon: 'disc-3', label: 'I Like to Move It!'},
    critical: {icon: 'dog', label: 'Who Let the Dogs Out?!'},
  };
  let openPicker;
  let openMenu;

  const togglePicker = picker => {
    if (!picker || picker.closest('.is-reordering')) return;
    if (picker === openPicker) {
      closePicker(true);
      return;
    }
    closePicker(false);
    openPicker = picker;
    const toggle = picker.querySelector('[data-notification-toggle]');
    const menu = picker.querySelector('[data-notification-menu]');
    openMenu = menu;
    toggle.setAttribute('aria-expanded', 'true');
    document.body.append(menu);
    menu.hidden = false;
    positionMenu(toggle, menu);
    const selected = menu.querySelector(`[data-level="${picker.dataset.notificationLevel}"]`);
    (selected ?? menu.querySelector('[data-notification-option]'))?.focus();
  };

  const closePicker = restoreFocus => {
    if (!openPicker) return;
    const picker = openPicker;
    const menu = openMenu;
    openPicker = undefined;
    openMenu = undefined;
    const toggle = picker.querySelector('[data-notification-toggle]');
    toggle?.setAttribute('aria-expanded', 'false');
    if (menu) {
      menu.hidden = true;
      menu.style.removeProperty('left');
      menu.style.removeProperty('top');
      if (picker.isConnected) picker.append(menu);
    }
    if (restoreFocus && toggle?.isConnected) toggle.focus();
  };

  const positionMenu = (toggle, menu) => {
    const toggleRect = toggle.getBoundingClientRect();
    const menuRect = menu.getBoundingClientRect();
    const margin = 12;
    const left = Math.min(
      Math.max(margin, toggleRect.right - menuRect.width),
      window.innerWidth - menuRect.width - margin,
    );
    let top = toggleRect.bottom + 8;
    if (top + menuRect.height > window.innerHeight - margin) {
      top = Math.max(margin, toggleRect.top - menuRect.height - 8);
    }
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
  };

  const moveMenuFocus = event => {
    const options = [...openMenu.querySelectorAll('[data-notification-option]:not(:disabled)')];
    if (!options.length) return;
    event.preventDefault();
    const current = options.indexOf(document.activeElement);
    let next = event.key === 'Home' ? 0 : options.length - 1;
    if (event.key === 'ArrowDown') next = (current + 1) % options.length;
    if (event.key === 'ArrowUp') next = (current - 1 + options.length) % options.length;
    options[next].focus();
  };

  const saveLevel = async (picker, level) => {
    if (!picker || !levels[level] || picker.dataset.saving === 'true') return;
    setSaving(picker, true);
    showError(picker, '');
    try {
      const response = await fetch(
        `/api/channels/${encodeURIComponent(picker.dataset.channelId)}/notification`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            ...(window.tubyBuildHeaders?.() ?? {}),
          },
          body: JSON.stringify({level}),
        },
      );
      window.tubyReloadIfStale?.(response);
      if (!response.ok) throw new Error('notification update failed');
      const saved = await response.json();
      updateChannelPickers(saved.channel_id, saved.level);
      closePicker(true);
    } catch {
      showError(picker, 'Could not save this notification level.');
    } finally {
      setSaving(picker, false);
    }
  };

  const updateChannelPickers = (channelID, level) => {
    if (!levels[level]) return;
    document.querySelectorAll('[data-notification-picker]').forEach(picker => {
      if (picker.dataset.channelId === channelID) syncPicker(picker, level);
    });
  };

  const syncPickers = () => {
    document.querySelectorAll('[data-notification-picker]').forEach(picker => {
      syncPicker(picker, picker.dataset.notificationLevel);
    });
  };

  const syncPicker = (picker, level) => {
    const selected = levels[level] ? level : 'off';
    const details = levels[selected];
    const toggle = picker.querySelector('[data-notification-toggle]');
    const icon = toggle.querySelector('img');
    const build = document.querySelector('meta[name="tuby-build"]')?.content ?? '';
    picker.dataset.notificationLevel = selected;
    icon.src = `/static/img/${details.icon}.svg?v=${encodeURIComponent(build)}`;
    toggle.title = `Notifications: ${details.label}`;
    toggle.setAttribute('aria-label', `Notifications for ${channelName(picker)}: ${details.label}`);
    pickerMenu(picker)?.querySelectorAll('[data-notification-option]').forEach(option => {
      option.setAttribute('aria-checked', String(option.dataset.level === selected));
    });
  };

  const setSaving = (picker, saving) => {
    picker.dataset.saving = String(saving);
    const menu = pickerMenu(picker);
    menu?.setAttribute('aria-busy', String(saving));
    menu?.querySelectorAll('[data-notification-option]').forEach(option => {
      option.disabled = saving;
    });
  };

  const showError = (picker, message) => {
    const error = pickerMenu(picker)?.querySelector('[data-notification-error]');
    if (!error) return;
    error.textContent = message;
    error.hidden = message === '';
  };

  const pickerMenu = picker => picker === openPicker
    ? openMenu
    : picker.querySelector('[data-notification-menu]');

  const channelName = picker => picker.closest('.channel-card')
    ?.querySelector('.channel-name')?.textContent?.trim()
    ?? document.querySelector('.page-title')?.textContent?.trim()
    ?? 'channel';

  document.addEventListener('click', event => {
    const toggle = event.target.closest('[data-notification-toggle]');
    if (toggle) {
      event.preventDefault();
      event.stopPropagation();
      togglePicker(toggle.closest('[data-notification-picker]'));
      return;
    }
    const option = event.target.closest('[data-notification-option]');
    if (option) {
      event.preventDefault();
      event.stopPropagation();
      void saveLevel(openPicker, option.dataset.level);
      return;
    }
    if (openPicker && !event.target.closest('[data-notification-picker]') &&
      !event.target.closest('[data-notification-menu]')) closePicker(true);
  });

  document.addEventListener('keydown', event => {
    if (!openPicker) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closePicker(true);
      return;
    }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      moveMenuFocus(event);
    }
  });

  document.addEventListener('htmx:beforeSwap', () => closePicker(false));
  document.addEventListener('htmx:afterSwap', syncPickers);
  document.addEventListener('htmx:historyRestore', syncPickers);
  window.addEventListener('resize', () => closePicker(false));
  window.addEventListener('scroll', () => closePicker(false), true);

  syncPickers();
})();
