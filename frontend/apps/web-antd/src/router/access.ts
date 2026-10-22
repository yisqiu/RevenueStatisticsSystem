import type { GenerateMenuAndRoutesOptions } from '@vben/types';
import { generateAccessible } from '@vben/access';
import { preferences } from '@vben/preferences';
import { fetchOidcUserInfo } from '#/auth/oidc';

export async function generateAccess(options: GenerateMenuAndRoutesOptions) {
  const user = await fetchOidcUserInfo();
  const routes = options.routes.map(route => route.name === 'Integrations' && route.meta && !user.super && !user.permissions.includes('Integration.Manage') ? { ...route, meta: { ...route.meta, authority: ['casdoor-super-admin'] } } : route);
  return generateAccessible(preferences.app.accessMode, {
    ...options, routes,
    forbiddenComponent: () => import('#/views/_core/fallback/forbidden.vue'),
  });
}
