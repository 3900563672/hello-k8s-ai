#!/usr/bin/env node
// 视觉验证：无头 Chrome CDP 截图 + 读取页面 / Grafana iframe 面板文本
// 用法：node hack/ui-check/grafana-panels.mjs [--url URL] [--out PNG] [--wait 秒]
// 前置：WSL 内 node >= 22（fetch / WebSocket 全局可用）；本机 Windows Chrome。
// 输出：stdout 为页面信息 JSON（Agent 解析用）；截图与控制台错误走 stderr。
import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';

const CHROME = process.env.CHROME_PATH || '/mnt/c/Users/hh/AppData/Local/Google/Chrome/Application/chrome.exe';
const args = process.argv.slice(2);
const get = (name, dflt) => {
  const i = args.indexOf(name);
  return i >= 0 && args[i + 1] ? args[i + 1] : dflt;
};
const URL = get('--url', 'http://localhost:8080/monitor');
const OUT = get('--out', '/root/hello-k8s-ai/.codex-tmp/monitor.png');
const WAIT = Number(get('--wait', '25'));
const PORT = 9230 + (Date.now() % 40);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
  '--remote-debugging-port=' + PORT, '--remote-debugging-address=0.0.0.0',
  '--user-data-dir=C:/Users/hh/AppData/Local/Temp/ch-cdp-' + Date.now(),
  '--window-size=1600,1000',
  'about:blank',
], { detached: true, stdio: 'ignore' });

let ok = false;
for (let i = 0; i < 30; i++) {
  try {
    const r = await fetch(`http://localhost:${PORT}/json/version`);
    if (r.ok) { ok = true; break; }
  } catch {}
  await sleep(500);
}
if (!ok) { console.error('CDP 未就绪（Windows Chrome 未启动？）'); chrome.kill(); process.exit(1); }

const created = await fetch(`http://localhost:${PORT}/json/new?${encodeURIComponent(URL)}`, { method: 'PUT' });
const target = await created.json();
const ws = new WebSocket(target.webSocketDebuggerUrl);
let nextId = 1;
const pending = new Map();
const consoleLogs = [];
ws.onmessage = (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); }
  if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error') {
    const text = (msg.params.args || []).map((a) => a.value ?? a.description ?? '').join(' ').slice(0, 200);
    consoleLogs.push('[console.error] ' + text);
  }
  if (msg.method === 'Runtime.exceptionThrown') {
    const d = msg.params.exceptionDetails;
    consoleLogs.push('[exception] ' + (d.exception?.description ?? d.text ?? '').slice(0, 300));
  }
};
await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
function send(method, params = {}) {
  const id = nextId++;
  ws.send(JSON.stringify({ id, method, params }));
  return new Promise((resolve) => pending.set(id, resolve));
}

await send('Page.enable');
await send('Runtime.enable');
console.error(`打开 ${URL}，等待 ${WAIT}s 加载…`);
await sleep(WAIT * 1000);

// Grafana 对屏外（懒渲染）面板不产出文本：先滚动 iframe 到底部再读取
await send('Runtime.evaluate', {
  expression: `(() => { const f = document.querySelector('iframe'); if (f && f.contentWindow) f.contentWindow.scrollTo(0, f.contentDocument.body.scrollHeight); })()`,
});
await sleep(3000);

const evalExpr = `(() => {
  const out = { url: location.href, title: document.title };
  const f = document.querySelector('iframe');
  if (!f) { out.panels = []; return JSON.stringify(out); }
  const doc = f.contentDocument;
  out.iframeUrl = doc.location.href;
  out.iframeTitle = doc.title;
  out.bodyLen = (doc.body ? doc.body.innerText : '').length;
  const containers = [...doc.querySelectorAll('[class*="panel-container"]')];
  if (!containers.length) containers.push(...doc.querySelectorAll('[class*="panel"]'));
  const seen = new Set();
  const panels = [];
  for (const el of containers) {
    const leaves = [...el.querySelectorAll('*')].filter((x) => x.children.length === 0 && x.textContent.trim());
    const title = leaves.length ? leaves[0].textContent.trim().slice(0, 40) : '?';
    if (seen.has(title)) continue;
    seen.add(title);
    const body = (el.innerText || '').replace(title, '').replace(/\\n+/g, ' | ').trim().slice(0, 600);
    panels.push({ title, body });
  }
  out.panels = panels;
  return JSON.stringify(out, null, 1);
})()`;
const res = await send('Runtime.evaluate', { expression: evalExpr, returnByValue: true });
const val = res.result?.result?.value;
if (val) {
  const parsed = JSON.parse(val);
  console.log(JSON.stringify(parsed, null, 1));
} else {
  console.log(JSON.stringify(res, null, 1));
}

const shot = await send('Page.captureScreenshot', { format: 'png' });
if (shot.result?.data) {
  writeFileSync(OUT, Buffer.from(shot.result.data, 'base64'));
  console.error(`截图已保存：${OUT}`);
}
if (consoleLogs.length) {
  console.error('=== 控制台错误 ===');
  for (const l of consoleLogs.slice(0, 20)) console.error(l);
}
ws.close();
chrome.kill();

