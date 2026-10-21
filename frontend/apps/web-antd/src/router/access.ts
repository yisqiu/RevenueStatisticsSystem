import type { GenerateMenuAndRoutesOptions } from '@vben/types';
import { generateAccessible } from '@vben/access';
import { preferences } from '@vben/preferences';
import { fetchOidcUserInfo } from '#/auth/oidc';

export async function generateAccess(options: GenerateMenuAndRoutesOptions) {
  await fetchOidcUserInfo();
  const routes = options.routes;
  return generateAccessible(preferences.app.accessMode, {
    ...options, routes,
    forbiddenComponent: () => import('#/views/_core/fallback/forbidden.vue'),
  });
}
