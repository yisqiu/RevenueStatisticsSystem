import assert from 'node:assert/strict';
import { test } from 'node:test';
import { requestBlob, requestJson } from '../frontend/apps/web-antd/src/views/revenue/request.ts';

async function withFetch(fake, check) {
  const original = globalThis.fetch;
  globalThis.fetch = fake;
  try { await check(); } finally { globalThis.fetch = original; }
}

test('GET retries a dropped connection and a truncated response body', async () => {
  for (const stage of ['fetch', 'body']) {
    let calls = 0;
    await withFetch(async () => {
      if (++calls > 1) return Response.json({ recovered: true });
      if (stage === 'fetch') throw new TypeError('Failed to fetch');
      const response = Response.json({});
      response.json = async () => { throw new TypeError('body connection lost'); };
      return response;
    }, async () => {
      assert.deepEqual(await requestJson('/api/system/bootstrap'), { recovered: true });
      assert.equal(calls, 2);
    });
  }
});

test('invalid JSON is rejected instead of being returned as successful data', async () => {
  let calls = 0;
  await withFetch(async () => { calls++; return new Response('{"users":'); }, async () => {
    await assert.rejects(requestJson('/api/system/bootstrap'), /服务返回的数据异常/);
    assert.equal(calls, 2);
  });
});

test('persistent GET failure stops after two attempts with a readable message', async () => {
  let calls = 0;
  await withFetch(async () => { calls++; throw new TypeError('Failed to fetch'); }, async () => {
    await assert.rejects(requestJson('/api/system/logs'), /网络连接暂时中断/);
    assert.equal(calls, 2);
  });
});

test('HTTP business errors retain their message and are not retried', async () => {
  let calls = 0;
  await withFetch(async () => { calls++; return Response.json({ message: '记录版本已变化' }, { status: 409 }); }, async () => {
    await assert.rejects(requestJson('/api/records'), /记录版本已变化/);
    assert.equal(calls, 1);
  });
});

test('writes never retry, including loss while reading their result', async () => {
  for (const method of ['POST', 'PUT', 'DELETE']) {
    for (const stage of ['fetch', 'body']) {
      let calls = 0;
      await withFetch(async () => {
        calls++;
        if (stage === 'fetch') throw new TypeError('Failed to fetch');
        return new Response('{');
      }, async () => {
        await assert.rejects(requestJson('/api/records', { method }), /刷新页面核对操作结果/);
        assert.equal(calls, 1);
      });
    }
  }
});

test('authorization failures notify the existing login prompt once', async () => {
  const originalWindow = globalThis.window;
  const target = new EventTarget();
  globalThis.window = target;
  try {
    for (const status of [401, 403]) {
      let calls = 0;
      const notices = [];
      const listener = event => notices.push(event.detail);
      target.addEventListener('team-auth-error', listener);
      try {
        await withFetch(async () => { calls++; return Response.json({ message: '登录或应用授权已失效' }, { status }); }, async () => {
          await assert.rejects(requestJson('/api/system/bootstrap'), /登录或应用授权已失效/);
          assert.equal(calls, 1);
          assert.deepEqual(notices, ['登录或应用授权已失效']);
        });
      } finally { target.removeEventListener('team-auth-error', listener); }
    }
  } finally { globalThis.window = originalWindow; }
});

test('downloads retry a body failure and return only the complete blob', async () => {
  let calls = 0;
  await withFetch(async () => {
    if (++calls > 1) return new Response('complete-csv');
    const response = new Response('partial');
    response.blob = async () => { throw new TypeError('body connection lost'); };
    return response;
  }, async () => {
    assert.equal(await (await requestBlob('/api/export')).text(), 'complete-csv');
    assert.equal(calls, 2);
  });
});
