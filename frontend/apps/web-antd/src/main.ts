import { initPreferences } from '@vben/preferences';
import { addIcon } from '@vben/icons';
import { unmountGlobalLoading } from '@vben/utils';
import { overridesPreferences } from './preferences';
import frameworkIcons from './icons/framework-icons.json';

async function initApplication() {
  const namespace = 'revenue-statistics-system-vben-v1';
  for (const { prefix, width, height, icons } of frameworkIcons) {
    for (const [name, icon] of Object.entries(icons)) {
      if (icon) addIcon(`${prefix}:${name}`, { width, height, ...icon });
    }
  }
  await initPreferences({ namespace, overrides: overridesPreferences });
  const { bootstrap } = await import('./bootstrap');
  await bootstrap(namespace);
  unmountGlobalLoading();
}
void initApplication().catch(() => {
  const root = document.querySelector('#app');
  if (root) {
    const message = document.createElement('p');
    message.textContent = '系统暂时无法加载，请检查网络后刷新页面。';
    const retry = document.createElement('button');
    retry.textContent = '重新加载';
    retry.onclick = () => window.location.reload();
    root.replaceChildren(message, retry);
  }
});
