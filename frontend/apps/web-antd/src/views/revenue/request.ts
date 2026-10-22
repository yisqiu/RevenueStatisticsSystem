async function request<T>(url: string, init: RequestInit, read: (response: Response) => Promise<T>): Promise<T> {
  const options: RequestInit = { cache: 'no-store', ...init };
  const isGet = (options.method || 'GET').toUpperCase() === 'GET';
  for (let attempt = 0; ; attempt++) {
    try {
      const response = await fetch(url, options);
      if (!response.ok) {
        let text = '操作失败，请稍后重试';
        try { text = (await response.json()).message || text; } catch {}
        if (response.status === 401 || response.status === 403) window.dispatchEvent(new CustomEvent('team-auth-error', { detail: text }));
        throw new Error(text);
      }
      return await read(response);
    } catch (error) {
      if (!(error instanceof TypeError || error instanceof SyntaxError)) throw error;
      if (isGet && attempt === 0) {
        await new Promise(resolve => setTimeout(resolve, 250));
        continue;
      }
      if (!isGet) throw new Error('操作响应读取失败，请刷新页面核对操作结果');
      throw new Error(error instanceof SyntaxError ? '服务返回的数据异常，请重新加载' : '网络连接暂时中断，请检查连接后重新加载');
    }
  }
}

export function requestJson<T>(url: string, init: RequestInit = {}): Promise<T> {
  return request(url, init, response => response.json());
}

export function requestBlob(url: string): Promise<Blob> {
  return request(url, {}, response => response.blob());
}
