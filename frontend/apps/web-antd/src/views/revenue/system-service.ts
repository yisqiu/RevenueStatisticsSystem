export async function systemApi(path: string, method = 'GET', data?: unknown): Promise<any> {
  const response = await fetch(`/api${path}`, { method, cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: data === undefined ? undefined : JSON.stringify(data) });
  const result = await response.json().catch(() => ({ message: '服务暂不可用，请稍后重试' }));
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new CustomEvent('team-auth-error', { detail: result.message }));
    throw new Error(result.message || '操作失败，请稍后重试');
  }
  return result;
}
