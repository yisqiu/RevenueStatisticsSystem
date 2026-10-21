import { createApp, watchEffect } from 'vue';
import { registerAccessDirective } from '@vben/access';
import { registerLoadingDirective } from '@vben/common-ui/es/loading';
import { globalShareState } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { initStores, resetAllStores, useAccessStore } from '@vben/stores';
import '@vben/styles';
import '@vben/styles/antd';
import { notification } from 'ant-design-vue';
import { completeOidcCallback, isOidcCallback } from '#/auth/oidc';
import { $t, setupI18n } from '#/locales';
import App from './app.vue';
import './views/revenue/theme.css';

export async function bootstrap(namespace: string) {
  const app = createApp(App);
  globalShareState.defineMessage({
    copyPreferencesSuccess: (title, content) => notification.success({ message: title, description: content, placement: 'bottomRight' }),
  });
  await setupI18n(app);
  await initStores(app, { namespace });
  const accessStore = useAccessStore();
  if (isOidcCallback()) {
    try {
      const { redirectPath } = await completeOidcCallback();
      resetAllStores();
      accessStore.setAccessToken('session');
      window.history.replaceState({}, '', `/#${redirectPath}`);
    } catch {
      resetAllStores();
      window.history.replaceState({}, '', '/#/auth/login?error=callback');
    }
  } else if (new URLSearchParams(window.location.search).has('signedOut')) {
    resetAllStores();
    window.history.replaceState({}, '', '/#/auth/login?signedOut=1');
  } else {
    const response = await fetch('/api/auth/session', { cache: 'no-store' });
    if (response.ok) accessStore.setAccessToken('session');
    else resetAllStores();
  }
  registerAccessDirective(app);
  registerLoadingDirective(app, { loading: 'loading', spinning: 'spinning' });
  const { initTippy } = await import('@vben/common-ui/es/tippy');
  initTippy(app);
  const { MotionPlugin } = await import('@vben/plugins/motion');
  app.use(MotionPlugin);
  // 先规范化回调或退出地址，再创建 history，确保首个路由读取最终 URL。
  const { router } = await import('./router');
  app.use(router);
  watchEffect(() => {
    const title = router.currentRoute.value.meta.title;
    document.title = title && preferences.app.dynamicTitle ? `${$t(title)} - ${preferences.app.name}` : preferences.app.name;
  });
  app.mount('#app');
}
