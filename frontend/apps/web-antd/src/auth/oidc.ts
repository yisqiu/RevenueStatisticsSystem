import type { UserInfo } from '@vben/types';

export const isOidcCallback = () => window.location.pathname === '/oidc-callback';
export function startOidcLogin(redirect = '/dashboard') {
  window.location.assign(`/api/auth/login?redirect=${encodeURIComponent(redirect)}`);
}
export async function completeOidcCallback() {
  const response = await fetch('/api/auth/session', { cache: 'no-store' });
  if (!response.ok) throw new Error('统一认证会话无效，请重新登录');
  const redirect = new URLSearchParams(window.location.search).get('redirect');
  const allowed = ['/dashboard', '/ranking', '/settings', '/integrations', '/module/wenchuang', '/module/naicha', '/module/ticket', '/module/hanfu', '/module/guide', '/module/bus', '/module/hotel', '/module/park', '/module/online', '/profile'];
  allowed.push('/system','/members');
  return { redirectPath: allowed.includes(redirect || '') ? redirect! : '/dashboard' };
}
export async function fetchOidcUserInfo(): Promise<UserInfo & { permissions: string[]; super: boolean; role: string }> {
  const response = await fetch('/api/auth/me', { cache: 'no-store' });
  if (!response.ok) throw new Error('统一认证会话或应用授权已失效');
  return response.json();
}
export async function startOidcLogout() {
  const response = await fetch('/api/auth/logout', { method: 'POST' });
  if (!response.ok) throw new Error('退出失败，请重试');
  window.location.assign('/?signedOut=1');
}
