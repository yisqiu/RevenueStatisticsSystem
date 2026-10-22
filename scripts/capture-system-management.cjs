const fs = require('fs');
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
(async () => {
  const targets = await (await fetch('http://127.0.0.1:9223/json/list')).json();
  const target = targets.find(t => t.url.includes('8027/?system-management=1'));
  if (!target) throw new Error('Existing system management task tab missing');
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
  let sequence = 0; const pending = new Map();
  ws.onmessage = event => { const message = JSON.parse(event.data); const item = pending.get(message.id); if (item) { pending.delete(message.id); clearTimeout(item.timer); message.error ? item.reject(new Error(message.error.message)) : item.resolve(message.result); } };
  const send = (method, params = {}) => new Promise((resolve, reject) => { const id = ++sequence, timer = setTimeout(() => reject(new Error(method + ' timed out')), 20000); pending.set(id, { resolve, reject, timer }); ws.send(JSON.stringify({ id, method, params })); });
  const evaluate = async expression => { const result = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text); return result.result.value; };
  const directory = 'artifacts/system-management'; fs.mkdirSync(directory, { recursive: true });
  const capture = async name => { await evaluate('window.scrollTo(0,0)'); await delay(200); const shot = await send('Page.captureScreenshot', { format: 'png', fromSurface: true }); fs.writeFileSync(`${directory}/${name}.png`, Buffer.from(shot.data, 'base64')); };
  const tab = async key => { await evaluate(`document.getElementById('system-tab-${key}').click()`); await delay(600); };
  const metrics = [];
  try {
    await send('Page.bringToFront');
    await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
    for (const key of ['users', 'roles', 'logs']) { await tab(key); await capture(key + '-desktop'); }
    await tab('roles');
    await evaluate(`document.querySelector('[data-role-code="super_admin"] button').click()`); await delay(400); await capture('super-permissions');
    await evaluate(`[...document.querySelectorAll('.ant-modal-content button')].find(e=>e.innerText.trim()==='关闭').click()`); await delay(400);
    await send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true }); await delay(1000);
    for (const key of ['users', 'roles', 'logs']) {
      await tab(key); await capture(key + '-mobile');
      metrics.push(await evaluate(`(()=>{const panel=document.getElementById('system-panel-${key}'),wrap=panel.querySelector('.t-wrap');return {tab:'${key}',width:innerWidth,bodyWidth:document.body.scrollWidth,roleColumns:getComputedStyle(document.querySelector('.system-role-grid')).gridTemplateColumns,tableClient:wrap?.clientWidth,tableScroll:wrap?.scrollWidth,mask:!!document.querySelector('.bg-overlay')}})()`));
    }
    fs.writeFileSync(directory + '/responsive.json', JSON.stringify(metrics, null, 2));
    console.log(JSON.stringify(metrics));
  } finally {
    await send('Emulation.clearDeviceMetricsOverride');
    await evaluate(`(()=>{const key=Object.keys(localStorage).find(k=>k.endsWith('-preferences'));const saved=JSON.parse(localStorage.getItem(key));saved.value.sidebar.collapsed=false;localStorage.setItem(key,JSON.stringify(saved));location.reload();return true})()`);
    ws.close();
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
