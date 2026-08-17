#!/usr/bin/env node
// 白天无人值守：按时间表轮换流量档位 + 健康检查 + 快照采集（运行期 0 Token）
// 用法：
//   node hack/night-run/day-watch.mjs --once                                  # 单轮
//   node hack/night-run/day-watch.mjs --loop --interval 900                    # 常驻（默认 15 分钟一轮）
//   node hack/night-run/day-watch.mjs --baseline-qps 35 --peak-qps 50 --peak-minutes 15 --cycle-minutes 60
// 剧本：每个 cycle-minutes 周期内，前 (cycle - peak) 分钟跑基线档，最后 peak 分钟跑压测档。
// 输出：JSON 行到 stdout；进程日志到 stderr。建议 setsid nohup 启动，日志落 .runtime/day-run/<日期>/。

import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const BASE = get('--base-url', 'http://localhost:8080');
const INTERVAL = Number(get('--interval', '900'));
const LOOP = args().includes('--loop');
const TENANT = get('--tenant', 'tenant-core');
const BASELINE_QPS = Number(get('--baseline-qps', '35'));
const PEAK_QPS = Number(get('--peak-qps', '50'));
const PEAK_MINUTES = Number(get('--peak-minutes', '15'));
const CYCLE_MINUTES = Number(get('--cycle-minutes', '60'));
const SNAPSHOT_EVERY = Number(get('--snapshot-every', '2')); // 每 N 轮抓一次快照

const utcNow = () => new Date().toISOString();
let idemCounter = 0;

function args() { return process.argv.slice(2); }
function get(name, fallback) {
  const index = args().indexOf(name);
  return index >= 0 ? args()[index + 1] : fallback;
}

async function httpJson(method, urlPath, body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15000);
  try {
    const response = await fetch(BASE + urlPath, {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...(method === 'PATCH' ? { 'Idempotency-Key': `day-watch-${Date.now()}-${idemCounter++}` } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    });
    const text = await response.text();
    let json = null;
    try { json = JSON.parse(text); } catch { /* 非 JSON 响应 */ }
    return { ok: response.ok, status: response.status, json, body: text.slice(0, 300) };
  } catch (error) {
    return { ok: false, status: 0, body: String(error.message || error) };
  } finally {
    clearTimeout(timer);
  }
}

function targetQps(now) {
  const minutes = now.getHours() * 60 + now.getMinutes();
  const position = minutes % CYCLE_MINUTES;
  return position >= CYCLE_MINUTES - PEAK_MINUTES ? PEAK_QPS : BASELINE_QPS;
}

function runHelper(script, extraArgs) {
  const result = spawnSync('node', [path.join(REPO, 'hack/night-run', script), ...extraArgs], {
    cwd: REPO,
    encoding: 'utf8',
    timeout: 180000,
  });
  return {
    ok: result.status === 0,
    status: result.status,
    stdout: (result.stdout || '').trim().slice(0, 400),
    stderr: (result.stderr || '').trim().slice(0, 400),
  };
}

async function adjustTraffic(target) {
  const current = await httpJson('GET', '/api/v1/traffic');
  const tenant = current.json?.data?.tenants?.find((t) => t.tenant?.name === TENANT);
  const currentQps = tenant?.allocatedQPS ?? tenant?.requestedQPS;
  if (!current.ok) {
    return { patched: false, reason: `traffic read failed: ${current.status}`, currentQps };
  }
  if (currentQps === target) {
    return { patched: false, reason: 'already-at-target', currentQps };
  }
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const result = await httpJson('PATCH', `/api/v1/tenants/${TENANT}/traffic`, { qps: target });
    if (result.ok) return { patched: true, reason: `patched to ${target}qps (attempt ${attempt})`, currentQps };
    await new Promise((resolve) => setTimeout(resolve, 10000));
  }
  return { patched: false, reason: `patch failed after 3 attempts`, currentQps };
}

async function runOnce(snapshotDue) {
  const now = new Date();
  const target = targetQps(now);
  const traffic = await adjustTraffic(target);
  const keepalive = runHelper('keepalive.mjs', ['--once']);
  let snapshot = null;
  if (snapshotDue) {
    snapshot = runHelper('snapshot.mjs', ['--once', '--summary']);
  }
  const record = {
    ts: utcNow(),
    local: now.toString(),
    targetQps: target,
    traffic,
    keepalive: { ok: keepalive.ok, status: keepalive.status, detail: keepalive.stderr || keepalive.stdout },
    snapshot: snapshot ? { ok: snapshot.ok, status: snapshot.status, detail: snapshot.stderr || snapshot.stdout } : null,
  };
  process.stdout.write(`${JSON.stringify(record)}\n`);
  if (!keepalive.ok || (snapshot && !snapshot.ok)) {
    process.stderr.write(`${utcNow()} WARN: keepalive/snapshot 未通过，详见上方记录\n`);
  }
  return record;
}

async function main() {
  let round = 0;
  do {
    round += 1;
    const snapshotDue = round % SNAPSHOT_EVERY === 0;
    await runOnce(snapshotDue);
    if (!LOOP) return;
    await new Promise((resolve) => setTimeout(resolve, INTERVAL * 1000));
  } while (LOOP);
}

main().catch((error) => {
  process.stderr.write(`${utcNow()} day-watch fatal: ${String(error.stack || error)}\n`);
  process.exit(1);
});
