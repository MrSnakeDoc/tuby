self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil(focusTuby());
});

const focusTuby = async () => {
  const windows = await self.clients.matchAll({type: 'window', includeUncontrolled: true});
  const client = windows.find(value => 'focus' in value);
  if (client) await client.focus();
};
