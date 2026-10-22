import { requestJson } from './request';

export async function systemApi(path: string, method = 'GET', data?: unknown): Promise<any> {
  return requestJson(`/api${path}`, { method, headers: { 'Content-Type': 'application/json' }, body: data === undefined ? undefined : JSON.stringify(data) });
}
