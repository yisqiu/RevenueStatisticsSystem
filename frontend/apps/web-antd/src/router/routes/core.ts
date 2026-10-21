import type { RouteRecordRaw } from 'vue-router';
import { preferences } from '@vben/preferences';

export const fallbackNotFoundRoute: RouteRecordRaw = {
  path: '/:path(.*)*', name: 'FallbackNotFound',
  component: () => import('#/views/_core/fallback/not-found.vue'),
  meta: { title: '404', hideInMenu: true, hideInTab: true, hideInBreadcrumb: true },
};
export const coreRoutes: RouteRecordRaw[] = [
  { name: 'Root', path: '/', redirect: preferences.app.defaultHomePath,
    component: () => import('#/layouts/basic.vue'),
    meta: { title: 'Root', hideInBreadcrumb: true }, children: [] },
  { name: 'Login', path: '/auth/login',
    component: () => import('#/views/_core/authentication/login.vue'),
    meta: { title: '统一身份认证', hideInTab: true } },
];
