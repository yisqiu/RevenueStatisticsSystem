import { defineOverridesPreferences } from '@vben/preferences';

export const overridesPreferences = defineOverridesPreferences({
  app: { accessMode: 'frontend', defaultHomePath: '/dashboard', enableCheckUpdates: false, layout: 'sidebar-nav', locale: 'zh-CN', name: '寿县古城景区营收统计系统' },
  header: { height: 66 },
  breadcrumb: { enable: false },
  logo: { source: '/revenue-logo.png', sourceDark: '/revenue-logo.png' },
  sidebar: { width: 238, fixedButton: true, collapsedButton: true },
  navigation: { styleType: 'plain' },
  theme: { mode: 'light', colorPrimary: '#a63a2b', semiDarkSidebar: true, semiDarkSidebarColor: '#2b2118' },
  tabbar: { enable: false, styleType: 'chrome' },
  widget: { notification: false, languageToggle: false, lockScreen: false, globalSearch: false, refresh: false, fullscreen: false, themeToggle: false, timezone: false, sidebarToggle: false },
  copyright: { companyName: '寿县古城景区', companySiteLink: '' },
});
