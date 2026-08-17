#!/usr/bin/env node
// 长时运行（long-run）：按时间表轮换流量档位 + 健康检查 + 快照采集 + 扩容/节点采集（运行期 0 Token）
// 用法：
//   node hack/night-run/day-watch.mjs --once                                  # 单轮试跑
//   node hack/night-run/day-watch.mjs --loop --interval 900                    # 常驻（默认 15 分钟一轮）
//   node hack/night-run/day-watch.mjs --until 18:00                            # 跑到本地 18:00 自动停止并恢复基线流量
//   node hack/night-run/day-watch.mjs --hours 4                                # 运行 4 小时后停止
//   node hack/night-run/day-watch.mjs --baseline-qps 200 --peak-qps 350 --peak-minutes 15 --cycle-minutes 60
// 剧本：每个 cycle-minutes 周期内，前 (cycle - peak) 分钟跑基线档，最后 peak 分钟跑压测档。
//       周期相位从进程启动时刻起算，避免"启动即峰值"。
// 输出：stdout 为摘要 JSON 行；每轮完整记录（含 keepalive/snapshot/kubectl 全量）落盘
//       .runtime/longrun/<日期>/rounds/；结束时生成 summary.md 并恢复 final-qps。
import { spawnSync } from 'node:child_process';
import { writeFileSync, mkdirSync, readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import http from 'node:http';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
// WSL 内脚本专用端口 18080：Windows 侧 dllhost 占用 localhost:8080 导致 WSL 内访问不稳（见 KNOWN_PITFALLS）
const BASE = get('--base-url', 'http://localhost:18080');
const INTERVAL = Number(get('--interval', '900'));
const LOOP = args().includes('--loop');
const TENANT = get('--tenant', 'tenant-core');
const BASELINE_QPS = Number(get('--baseline-qps', '35'));
const PEAK_QPS = Number(get('--peak-qps', '50'));
const PEAK_MINUTES = Number(get('--peak-minutes', '15'));
const CYCLE_MINUTES = Number(get('--cycle-minutes', '60'));
const SNAPSHOT_EVERY = Number(get('--snapshot-every', '2')); // 每 N 轮抓一次快照
const UNTIL = get('--until', '');                            // 本地时区 HH:MM，到点自动停止
const HOURS = Number(get('--hours', '0'));                   // 运行 N 小时后停止（--until 优先）
const FINAL_QPS = Number(get('--final-qps', String(BASELINE_QPS))); // 结束时恢复的流量档位

const START_MS = Date.now();
const utcNow = () => new Date().toISOString();
let idemCounter = 0;

// 本地日期（Asia/Shanghai 固定 +8h，与 snapshot.mjs 一致），用于产物目录
function localDate() {
  const shifted = new Date(Date.now() + 8 * 3600 * 1000);
  return shifted.toISOString().slice(0, 10);
}
const RUN_DIR = path.join(REPO, '.runtime/longrun', localDate());
const ROUNDS_DIR = path.join(RUN_DIR, 'rounds');
const SNAPSHOT_DIR = path.join(RUN_DIR, 'snapshots');

function args() { return process.argv.slice(2); }
function get(name, fallback) {
  const index = args().indexOf(name);
  return index >= 0 ? args()[index + 1] : fallback;
}
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// port-forward 隧道对 keep-alive 长连接不友好（已知坑）：不用 undici/fetch 连接池，
// 每次请求新建 TCP 连接（agent:false），网络层错误自动重试（最多 3 次，间隔 1s）。
function httpJson(method, urlPath, body, attempts = 3) {
  const run = () => new Promise((resolve) => {
    const url = new URL(BASE + urlPath);
    const payload = body === undefined ? null : JSON.stringify(body);
    const headers = { 'Content-Type': 'application/json', Connection: 'close' };
    if (method === 'PATCH') {
      headers['Idempotency-Key'] = `day-watch-${Date.now()}-${idemCounter++}`;
    }
    const request = http.request({
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method,
      headers,
      timeout: 15000,
      agent: false,
    }, (response) => {
      let text = '';
      response.setEncoding('utf8');
      response.on('data', (chunk) => { text += chunk; });
      response.on('end', () => {
        let json = null;
        try { json = JSON.parse(text); } catch { /* 非 JSON 响应保留原文 */ }
        resolve({ ok: response.statusCode >= 200 && response.statusCode < 300, status: response.statusCode, json, body: text.slice(0, 300) });
      });
    });
    request.on('error', (error) => resolve({ ok: false, status: 0, json: null, body: String(error.message || error) }));
    request.on('timeout', () => request.destroy(new Error('request timeout')));
    if (payload) request.write(payload);
    request.end();
  });
  return (async () => {
    let last;
    for (let attempt = 1; attempt <= attempts; attempt += 1) {
      last = await run();
      if (last.ok || last.status !== 0) return { ...last, attempt };
      await sleep(1000);
    }
    return { ...last, attempt: attempts };
  })();
}

// 周期相位从进程启动时刻起算：启动后先跑基线，再跑峰值，循环。
function targetQps() {
  const elapsedMinutes = (Date.now() - START_MS) / 60000;
  const position = elapsedMinutes % CYCLE_MINUTES;
  return position >= CYCLE_MINUTES - PEAK_MINUTES ? PEAK_QPS : BASELINE_QPS;
}

function shouldStop() {
  if (UNTIL) {
    const [hours, minutes] = UNTIL.split(':').map(Number);
    if (Number.isFinite(hours) && Number.isFinite(minutes)) {
      const now = new Date();
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes, 0);
      return now >= end;
    }
  }
  if (HOURS > 0) {
    return Date.now() - START_MS >= HOURS * 3600 * 1000;
  }
  return false;
}

function runHelper(script, extraArgs) {
  const result = spawnSync('node', [path.join(REPO, 'hack/night-run', script), ...extraArgs], {
    cwd: REPO,
    encoding: 'utf8',
    timeout: 240000, // snapshot 抓 6 个指标 + 4 个接口，给足超时
  });
  return {
    ok: result.status === 0,
    status: result.status,
    stdout: (result.stdout || '').trim(),
    stderr: (result.stderr || '').trim(),
    error: result.error ? String(result.error.message || result.error).slice(0, 200) : undefined,
  };
}

async function adjustTraffic(target) {
  const current = await httpJson('GET', '/api/v1/traffic');
  if (!current.ok) {
    return { patched: false, reason: `traffic read failed: ${current.status} (${current.body || current.attempt})`, currentQps: null };
  }
  const tenant = current.json?.data?.tenants?.find((t) => t.tenant?.name === TENANT);
  const currentQps = tenant?.allocatedQPS ?? tenant?.requestedQPS;
  if (currentQps === target) {
    return { patched: false, reason: 'already-at-target', currentQps };
  }
  const failures = [];
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const result = await httpJson('PATCH', `/api/v1/tenants/${TENANT}/traffic`, { qps: target });
    if (result.ok) return { patched: true, reason: `patched to ${target}qps (attempt ${attempt})`, currentQps };
    failures.push(`${result.status}/${result.body || result.attempt}`);
    await sleep(2000);
  }
  return { patched: false, reason: `patch failed after 3 attempts: ${failures.join('; ')}`, currentQps };
}

// 每轮轻量集群采集：节点用量、Orchestrator 最近扩缩、实例副本（信息收集增强）
function kubeSnapshot() {
  const result = { workernodes: [], scaling: null, instances: [] };
  const run = (args) => spawnSync('kubectl', args, { encoding: 'utf8', timeout: 15000, maxBuffer: 32 * 1024 * 1024 });
  const wn = run(['get', 'workernodes', '-o', 'json']);
  if (wn.status === 0) {
    try {
      result.workernodes = (JSON.parse(wn.stdout).items || []).map((item) => ({
        name: item.metadata.name,
        specConcurrency: item.spec?.maxConcurrency,
        specGPU: item.spec?.gpu,
        usedConcurrency: item.status?.usedConcurrency,
        usedGPU: item.status?.usedGPU,
      }));
    } catch (error) { result.workernodeError = String(error).slice(0, 200); }
  } else { result.workernodeError = String(wn.stderr || wn.status).slice(0, 200); }

  const orc = run(['get', 'orchestrators', '-o', 'json']);
  if (orc.status === 0) {
    try {
      const item = (JSON.parse(orc.stdout).items || [])[0];
      result.scaling = item?.status?.lastScaling ? {
        action: item.status.lastScaling.action,
        oldReplicas: item.status.lastScaling.oldReplicas,
        newReplicas: item.status.lastScaling.newReplicas,
        time: item.status.lastScaling.time,
        ready: (item.status?.conditions || []).find((c) => c.type === 'Ready')?.status,
      } : null;
    } catch (error) { result.orchestratorError = String(error).slice(0, 200); }
  } else { result.orchestratorError = String(orc.stderr || orc.status).slice(0, 200); }

  const inst = run(['get', 'simulatorinstances', '-o', 'json']);
  if (inst.status === 0) {
    try {
      result.instances = (JSON.parse(inst.stdout).items || []).map((item) => ({
        name: item.metadata.name,
        desiredReplicas: item.spec?.replicas,
        availableReplicas: item.status?.availableReplicas,
        phase: item.status?.phase,
      }));
    } catch (error) { result.instancesError = String(error).slice(0, 200); }
  } else { result.instancesError = String(inst.stderr || inst.status).slice(0, 200); }
  return result;
}

async function preflight() {
  const checks = [];
  let backendOk = false;
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const result = await httpJson('GET', '/api/v1/health/live');
    if (result.ok) { backendOk = true; break; }
    await sleep(3000);
  }
  checks.push({ name: 'backend-18080', ok: backendOk, detail: backendOk ? 'reachable' : 'unreachable after 3 attempts' });
  const guard = spawnSync('bash', [path.join(REPO, 'hack/night-run/sleep-guard.sh'), 'status'], {
    encoding: 'utf8', timeout: 30000,
  });
  const guardOut = (guard.stdout || '').trim().split('\n').pop() || String(guard.stderr || '').slice(0, 200);
  checks.push({ name: 'sleep-guard', ok: guardOut.includes('guard=on'), detail: guardOut });
  return checks;
}

async function runOnce(round, snapshotDue, preflightChecks) {
  const roundStart = utcNow();
  const target = targetQps();
  const traffic = await adjustTraffic(target);
  const keepalive = runHelper('keepalive.mjs', ['--once', '--base-url', BASE]);
  let snapshot = null;
  if (snapshotDue) {
    snapshot = runHelper('snapshot.mjs', ['--once', '--summary', '--base-url', BASE, '--out-dir', SNAPSHOT_DIR]);
  }
  const kube = kubeSnapshot();
  const record = {
    round,
    ts: roundStart,
    targetQps: target,
    traffic,
    keepalive: {
      ok: keepalive.ok,
      status: keepalive.status,
      stdout: keepalive.stdout,
      stderr: keepalive.stderr,
      error: keepalive.error,
    },
    snapshot: snapshot ? { ok: snapshot.ok, status: snapshot.status, stdout: snapshot.stdout, stderr: snapshot.stderr, error: snapshot.error } : null,
    kube,
  };
  // 完整记录落盘，供结束时生成 summary 与事后分析
  try {
    mkdirSync(ROUNDS_DIR, { recursive: true });
    writeFileSync(path.join(ROUNDS_DIR, `round-${String(round).padStart(3, '0')}-${roundStart.replace(/[:.]/g, '-')}.json`), `${JSON.stringify(record, null, 2)}\n`, 'utf8');
  } catch (error) {
    process.stderr.write(`${utcNow()} rounds 落盘失败: ${String(error).slice(0, 200)}\n`);
  }

  // stdout 摘要行（向后兼容，字段精简）
  const brief = {
    ts: roundStart,
    round,
    targetQps: target,
    traffic,
    keepalive: { ok: keepalive.ok, status: keepalive.status, detail: (keepalive.stderr || keepalive.stdout || keepalive.error || '').slice(0, 400) },
    snapshot: snapshot ? { ok: snapshot.ok, status: snapshot.status, detail: (snapshot.stderr || snapshot.stdout || snapshot.error || '').slice(0, 400) } : null,
    scaling: kube.scaling,
    replicas: kube.instances.map((i) => `${i.name}=${i.availableReplicas ?? '?'}/${i.desiredReplicas ?? '?'}`).join('; ') || null,
    nodes: kube.workernodes.map((n) => `${n.name}:${n.usedConcurrency ?? '?'}/${n.specConcurrency ?? '?'}`).join('; ') || null,
  };
  process.stdout.write(`${JSON.stringify(brief)}\n`);
  if (preflightChecks && preflightChecks.some((c) => !c.ok)) {
    process.stderr.write(`${utcNow()} preflight: ${preflightChecks.map((c) => `${c.name}=${c.ok ? 'ok' : 'WARN(' + (c.detail || '') + ')'}`).join(' ')}\n`);
  }
  if (!keepalive.ok || (snapshot && !snapshot.ok)) {
    process.stderr.write(`${utcNow()} WARN: keepalive/snapshot 未通过，详见 rounds 完整记录\n`);
  }
  return record;
}

// 结束时：恢复流量 + 生成 summary.md
async function finish() {
  process.stderr.write(`${utcNow()} 到达结束时间，恢复流量到 ${FINAL_QPS}qps 并生成 summary\n`);
  const restore = await adjustTraffic(FINAL_QPS);
  const summary = buildSummary(restore);
  try {
    mkdirSync(RUN_DIR, { recursive: true });
    writeFileSync(path.join(RUN_DIR, 'summary.md'), summary, 'utf8');
  } catch (error) {
    process.stderr.write(`${utcNow()} summary 落盘失败: ${String(error).slice(0, 200)}\n`);
  }
  process.stdout.write(`${summary}\n`);
}

function buildSummary(restore) {
  const lines = [`# 长时运行汇总（${localDate()}）`, '', `- 结束时间（UTC）：${utcNow()}`, `- 参数：baseline=${BASELINE_QPS}qps peak=${PEAK_QPS}qps cycle=${CYCLE_MINUTES}min peak=${PEAK_MINUTES}min interval=${INTERVAL}s`, `- 恢复流量：${restore.patched ? '已恢复' : '未恢复（' + (restore.reason || '?') + '）'}`, ''];
  let rounds = [];
  try {
    const files = readdirSync(ROUNDS_DIR).filter((f) => f.startsWith('round-') && f.endsWith('.json')).sort();
    for (const file of files) {
      try { rounds.push(JSON.parse(readFileSync(path.join(ROUNDS_DIR, file), 'utf8'))); } catch { /* skip */ }
    }
  } catch { /* 无 rounds 目录 */ }
  const failedKeepalive = rounds.filter((r) => !r.keepalive?.ok);
  const failedSnapshot = rounds.filter((r) => r.snapshot && !r.snapshot.ok);
  lines.push(`## 轮次统计`, `- 总轮数：${rounds.length}`, `- keepalive 失败：${failedKeepalive.length}（${failedKeepalive.map((r) => `#${r.round}`).join(', ') || '-'}）`, `- snapshot 失败：${failedSnapshot.length}（${failedSnapshot.map((r) => `#${r.round}`).join(', ') || '-'}）`, '');

  // 扩容事件：相邻轮 lastScaling 变化
  const events = [];
  let lastScaling = '';
  for (const round of rounds) {
    const scaling = round.kube?.scaling;
    const key = scaling ? `${scaling.action} ${scaling.oldReplicas}->${scaling.newReplicas} @${scaling.time}` : '';
    if (key && key !== lastScaling) {
      events.push(`- ${round.ts} #${round.round} ${key}`);
      lastScaling = key;
    }
  }
  lines.push(`## 扩缩容事件`, events.length ? events.join('\n') : '- 无（未发生扩缩容）', '');

  // 副本与节点趋势：首/末轮
  const first = rounds[0];
  const last = rounds[rounds.length - 1];
  if (first && last) {
    const fmtInst = (r) => (r.kube?.instances || []).map((i) => `${i.name}=${i.availableReplicas ?? '?'}/${i.desiredReplicas ?? '?'}`).join('; ');
    const fmtNode = (r) => (r.kube?.workernodes || []).map((n) => `${n.name}:${n.usedConcurrency ?? '?'}/${n.specConcurrency ?? '?'}`).join('; ');
    lines.push(`## 趋势（首/末）`, `- 副本 首：${fmtInst(first) || '-'}`, `- 副本 末：${fmtInst(last) || '-'}`, `- 节点 首：${fmtNode(first) || '-'}`, `- 节点 末：${fmtNode(last) || '-'}`, '');
  }

  // 快照指标范围
  const snapshots = [];
  try {
    const dir = path.join(RUN_DIR, 'snapshots');
    const files = readdirSync(dir).filter((f) => f.endsWith('.json')).sort();
    for (const file of files) {
      try { snapshots.push(JSON.parse(readFileSync(path.join(dir, file), 'utf8'))); } catch { /* skip */ }
    }
  } catch { /* 无快照 */ }
  if (snapshots.length) {
    const range = (metric) => {
      const values = snapshots.map((s) => s.metrics?.[metric]?.value).filter((v) => v != null && Number.isFinite(Number(v))).map(Number);
      if (!values.length) return '-';
      return `min=${Math.min(...values)} max=${Math.max(...values)} last=${values[values.length - 1]}`;
    };
    lines.push(`## 快照指标（${snapshots.length} 个）`, `- controller.errorRate：${range('controller.errorRate')}`, `- simulator.errorRate：${range('simulator.errorRate')}`, `- simulator.ttft：${range('simulator.ttft')}`, `- simulator.queue：${range('simulator.queue')}`, `- simulator.qps：${range('simulator.qps')}`, '');
  }

  lines.push(`## 结论与后续`, `- 完整数据：rounds/ 与 snapshots/ 目录（.runtime/longrun/${localDate()}/）`, `- 失败轮次的 keepalive/snapshot 明细见对应 round JSON 文件`, ``);
  return lines.join('\n');
}

async function main() {
  const preflightChecks = await preflight();
  for (const check of preflightChecks) {
    process.stderr.write(`${utcNow()} preflight ${check.name}: ${check.ok ? 'ok' : 'WARN ' + (check.detail || '')}\n`);
  }
  let round = 0;
  do {
    round += 1;
    const roundStart = Date.now();
    const snapshotDue = round % SNAPSHOT_EVERY === 0;
    await runOnce(round, snapshotDue, preflightChecks);
    if (!LOOP) return;
    if (shouldStop()) break;
    // 精确轮次间隔：按本轮实际耗时补足，避免"sleep 一轮 + 执行耗时"导致整体漂移
    const elapsed = Date.now() - roundStart;
    const wait = Math.max(5000, INTERVAL * 1000 - elapsed);
    await sleep(wait);
  } while (LOOP);
  if (LOOP && shouldStop()) {
    await finish();
  }
}

main().catch((error) => {
  process.stderr.write(`${utcNow()} day-watch fatal: ${String(error.stack || error)}\n`);
  process.exit(1);
});
