#!/usr/bin/env node
// 长时运行（long-run）：按时间表轮换流量档位 + 健康检查 + 快照采集 + 扩容/节点采集（运行期 0 Token）
// 用法：
//   node hack/night-run/day-watch.mjs --once                                  # 单轮试跑
//   node hack/night-run/day-watch.mjs --loop --interval 900                    # 常驻（默认 15 分钟一轮）
//   node hack/night-run/day-watch.mjs --until 18:00                            # 跑到本地 18:00 自动停止并恢复基线流量
//   node hack/night-run/day-watch.mjs --hours 4                                # 运行 4 小时后停止
//   node hack/night-run/day-watch.mjs --baseline-qps 200 --peak-qps 350 --peak-minutes 15 --cycle-minutes 60
//   node hack/night-run/day-watch.mjs --run-dir .runtime/longrun-test/2026-08-17   # 覆盖产物目录（测试隔离用）
//   node hack/night-run/day-watch.mjs --resummarize .runtime/longrun/2026-08-17    # 仅重生成 summary.md（需 meta.json）
// 剧本：每个 cycle-minutes 周期内，前 (cycle - peak) 分钟跑基线档，最后 peak 分钟跑压测档。
//       周期相位从进程启动时刻起算，避免"启动即峰值"。
// 输出：stdout 为摘要 JSON 行；每轮完整记录（含 keepalive/snapshot/kubectl 全量）落盘
//       .runtime/longrun/<日期>/rounds/；峰值中点在 metric-samples/ 追加一次指标采样；
//       启动时写 meta.json（startIso/endIso），summary 只统计本次 run 的轮次；结束时生成 summary.md 并恢复 final-qps。
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
const STRICT_PHASE = args().includes('--strict-phase'); // #48：相位错位时拒绝启动（默认只告警）
const RESUMMARIZE = get('--resummarize', '');                // 只重生成指定目录的 summary.md

const START_MS = Date.now();
const START_ISO = new Date().toISOString();
const utcNow = () => new Date().toISOString();
let idemCounter = 0;

// 本地日期（Asia/Shanghai 固定 +8h，与 snapshot.mjs 一致），用于产物目录
function localDate() {
  const shifted = new Date(Date.now() + 8 * 3600 * 1000);
  return shifted.toISOString().slice(0, 10);
}
const RUN_DIR_OVERRIDE = get('--run-dir', '');
let RUN_DIR = RUN_DIR_OVERRIDE ? path.resolve(REPO, RUN_DIR_OVERRIDE) : path.join(REPO, '.runtime/longrun', localDate());
let ROUNDS_DIR = path.join(RUN_DIR, 'rounds');
let SNAPSHOT_DIR = path.join(RUN_DIR, 'snapshots');
let METRIC_DIR = path.join(RUN_DIR, 'metric-samples');
let META_PATH = path.join(RUN_DIR, 'meta.json');
// --resummarize 时把产物目录重定向到目标 run（buildSummary 前调用）
function redirectRunDir(dir) {
  RUN_DIR = dir;
  ROUNDS_DIR = path.join(RUN_DIR, 'rounds');
  SNAPSHOT_DIR = path.join(RUN_DIR, 'snapshots');
  METRIC_DIR = path.join(RUN_DIR, 'metric-samples');
  META_PATH = path.join(RUN_DIR, 'meta.json');
}

// 与 snapshot.mjs 同源的指标 ID：每轮轻量采样一次，峰值中点再补一次（见 scheduleMidPeakSample）
const METRIC_IDS = ['controller.errorRate', 'simulator.errorRate', 'simulator.ttft', 'simulator.queue', 'simulator.qps', 'simulator.tickLatency'];

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

// #48 相位校验：轮次间隔（分钟）与周期/峰值窗口错位时，峰值永远不会生效。
// 轮次落在 k*interval mod cycle；集合 = gcd(interval, cycle) 的倍数，检查是否命中 [cycle-peak, cycle)。
export function phaseHitsPeak(intervalMin, cycleMin, peakMin) {
  if (intervalMin <= 0 || cycleMin <= 0 || peakMin <= 0 || peakMin >= cycleMin) return false;
  const gcd = (a, b) => (b === 0 ? a : gcd(b, a % b));
  const step = gcd(intervalMin, cycleMin);
  for (let t = 0; t < cycleMin; t += step) {
    if (t >= cycleMin - peakMin) return true;
  }
  return false;
}

// 周期相位从进程启动时刻起算：启动后先跑基线，再跑峰值，循环。
function targetQps() {
  const elapsedMinutes = (Date.now() - START_MS) / 60000;
  const position = elapsedMinutes % CYCLE_MINUTES;
  return position >= CYCLE_MINUTES - PEAK_MINUTES ? PEAK_QPS : BASELINE_QPS;
}

// 到截止时间的剩余毫秒；没有截止时间时返回 Infinity（不会停止）。
function msUntilStop() {
  if (UNTIL) {
    const [hours, minutes] = UNTIL.split(':').map(Number);
    if (Number.isFinite(hours) && Number.isFinite(minutes)) {
      const now = new Date();
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes, 0);
      return Math.max(0, end - now);
    }
  }
  if (HOURS > 0) return Math.max(0, START_MS + HOURS * 3600 * 1000 - Date.now());
  return Infinity;
}
function shouldStop() { return msUntilStop() === 0; }

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

// 每轮轻量指标采样：取 6 个指标的最新值（与快照同源，粒度 = 轮次间隔）。
async function fetchMetrics() {
  const out = {};
  let failed = 0;
  for (const metricId of METRIC_IDS) {
    const result = await httpJson('GET', `/api/v1/metrics?metricId=${metricId}&step=300s`);
    if (!result.ok) { failed += 1; out[metricId] = { error: `${result.status}/${result.body || result.attempt}` }; continue; }
    const series = result.json?.data?.series || [];
    const values = series.flatMap((s) => s.points || []).map((p) => Number(p.value)).filter((v) => Number.isFinite(v));
    out[metricId] = values.length ? values[values.length - 1] : null;
  }
  out._failed = failed;
  return out;
}

// 峰值中点补一次指标采样：15 分钟轮次粒度会错过峰值中段的强度，
// 在进入峰值的下一轮 sleep 期间用 setTimeout 触发（PG resource_events 5s 序列仍是权威证据链）。
const midPeakSamples = [];
let prevTarget = null;
function scheduleMidPeakSample(round) {
  const delay = Math.round((PEAK_MINUTES / 2) * 60000);
  setTimeout(async () => {
    const metrics = await fetchMetrics();
    const sample = { kind: 'mid-peak', round, ts: utcNow(), metrics };
    midPeakSamples.push(sample);
    try {
      mkdirSync(METRIC_DIR, { recursive: true });
      writeFileSync(path.join(METRIC_DIR, `mid-peak-round-${String(round).padStart(3, '0')}-${sample.ts.replace(/[:.]/g, '-')}.json`), `${JSON.stringify(sample, null, 2)}\n`, 'utf8');
    } catch (error) {
      process.stderr.write(`${utcNow()} mid-peak 采样落盘失败: ${String(error).slice(0, 200)}\n`);
    }
  }, delay);
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
  const metrics = await fetchMetrics();
  // 峰值相位切换（基线 → 峰值）时，预约峰值中点补采样（只对常驻模式生效）
  const peakJustStarted = LOOP && target === PEAK_QPS && prevTarget === BASELINE_QPS;
  prevTarget = target;
  if (peakJustStarted) scheduleMidPeakSample(round);
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
    metrics,
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
    metrics: metrics && metrics['simulator.qps'] != null
      ? { qps: metrics['simulator.qps'], queue: metrics['simulator.queue'], ttft: metrics['simulator.ttft'], errorRate: metrics['simulator.errorRate'] }
      : null,
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

// 结束时：恢复流量 + 记录 endIso + 生成 summary.md
async function finish() {
  process.stderr.write(`${utcNow()} 到达结束时间，恢复流量到 ${FINAL_QPS}qps 并生成 summary\n`);
  const restore = await adjustTraffic(FINAL_QPS);
  const summary = buildSummary(restore, '');
  try {
    mkdirSync(RUN_DIR, { recursive: true });
    writeFileSync(path.join(RUN_DIR, 'summary.md'), summary, 'utf8');
    const meta = readMeta();
    writeFileSync(META_PATH, `${JSON.stringify({ ...meta, endIso: utcNow() }, null, 2)}\n`, 'utf8');
  } catch (error) {
    process.stderr.write(`${utcNow()} summary 落盘失败: ${String(error).slice(0, 200)}\n`);
  }
  process.stdout.write(`${summary}\n`);
}

function readMeta() {
  try {
    return JSON.parse(readFileSync(META_PATH, 'utf8')) || {};
  } catch {
    return {};
  }
}

function buildSummary(restore, resummarizeDir) {
  const meta = resummarizeDir ? (() => {
    try { return JSON.parse(readFileSync(path.join(resummarizeDir, 'meta.json'), 'utf8')) || {}; } catch { return {}; }
  })() : readMeta();
  const paramLine = meta.args ? `- 参数：${meta.args.join(' ')}` : `- 参数：baseline=${BASELINE_QPS}qps peak=${PEAK_QPS}qps cycle=${CYCLE_MINUTES}min peak=${PEAK_MINUTES}min interval=${INTERVAL}s`;
  const restoreState = restore
    ? (restore.patched ? '已恢复' : (restore.reason === 'already-at-target' ? '已恢复（已在目标）' : '未恢复（' + (restore.reason || '?') + '）'))
    : '未提供（--resummarize 只重生成）';
  const endLine = meta.endIso ? `- 结束时间（UTC）：${meta.endIso}` : `- 结束时间（UTC）：${utcNow()}`;
  const lines = [`# 长时运行汇总（${localDate()}）`, '', endLine, paramLine, `- 恢复流量：${restoreState}`, ''];
  // 只统计 [startIso, endIso] 窗口内的轮次与快照，避免同一日期目录里多轮 run 互相污染
  const inWindow = (ts) => {
    if (!meta.startIso) return true; // 无 meta 时退回全量（老数据）
    if (ts < meta.startIso) return false;
    if (meta.endIso && ts > meta.endIso) return false;
    return true;
  };
  let rounds = [];
  try {
    const files = readdirSync(ROUNDS_DIR).filter((f) => f.startsWith('round-') && f.endsWith('.json')).sort();
    for (const file of files) {
      try {
        const round = JSON.parse(readFileSync(path.join(ROUNDS_DIR, file), 'utf8'));
        if (inWindow(round.ts)) rounds.push(round);
      } catch { /* skip */ }
    }
  } catch { /* 无 rounds 目录 */ }
  const failedKeepalive = rounds.filter((r) => !r.keepalive?.ok);
  const failedSnapshot = rounds.filter((r) => r.snapshot && !r.snapshot.ok);
  lines.push(`## 轮次统计`, `- 总轮数：${rounds.length}`, `- keepalive 失败：${failedKeepalive.length}（${failedKeepalive.map((r) => `#${r.round}`).join(', ') || '-'}）`, `- snapshot 失败：${failedSnapshot.length}（${failedSnapshot.map((r) => `#${r.round}`).join(', ') || '-'}）`, '');

  // 扩容事件：相邻轮 lastScaling 变化（仅本次 run 窗口内）
  const events = [];
  let lastScaling = '';
  for (const round of rounds) {
    const scaling = round.kube?.scaling;
    const key = scaling ? `${scaling.action} ${scaling.oldReplicas}->${scaling.newReplicas} @${scaling.time}` : '';
    const inEventWindow = scaling && inWindow(scaling.time);
    if (key && inEventWindow && key !== lastScaling) {
      events.push(`- ${round.ts} #${round.round} ${key}`);
      lastScaling = key;
    }
  }
  lines.push(`## 扩缩容事件`, events.length ? events.join('\n') : '- 无（未发生扩缩容）', '');

  // 副本与节点趋势：首/末轮（窗口内）
  const first = rounds[0];
  const last = rounds[rounds.length - 1];
  if (first && last) {
    const fmtInst = (r) => (r.kube?.instances || []).map((i) => `${i.name}=${i.availableReplicas ?? '?'}/${i.desiredReplicas ?? '?'}`).join('; ');
    const fmtNode = (r) => (r.kube?.workernodes || []).map((n) => `${n.name}:${n.usedConcurrency ?? '?'}/${n.specConcurrency ?? '?'}`).join('; ');
    lines.push(`## 趋势（首/末）`, `- 副本 首：${fmtInst(first) || '-'}`, `- 副本 末：${fmtInst(last) || '-'}`, `- 节点 首：${fmtNode(first) || '-'}`, `- 节点 末：${fmtNode(last) || '-'}`, '');
  }

  // 轮内指标（每轮 1 个点 + 峰值中点采样）：粒度 = 轮次间隔，峰值中段有补点
  const roundMetricPoints = (metricId) => {
    const values = [];
    const seen = new Set();
    const pushPoint = (ts, value, midPeak) => {
      if (seen.has(ts)) return; // 内存与落盘双路径去重
      seen.add(ts);
      values.push({ ts, value, midPeak: midPeak || false });
    };
    for (const round of rounds) {
      const v = round.metrics?.[metricId];
      if (v != null && Number.isFinite(Number(v))) pushPoint(round.ts, Number(v), false);
    }
    for (const sample of midPeakSamples) {
      const v = sample.metrics?.[metricId];
      if (v != null && Number.isFinite(Number(v))) pushPoint(sample.ts, Number(v), true);
    }
    try {
      const files = readdirSync(METRIC_DIR).filter((f) => f.endsWith('.json')).sort();
      for (const file of files) {
        const sample = JSON.parse(readFileSync(path.join(METRIC_DIR, file), 'utf8'));
        if (!inWindow(sample.ts)) continue;
        const v = sample.metrics?.[metricId];
        if (v != null && Number.isFinite(Number(v))) pushPoint(sample.ts, Number(v), true);
      }
    } catch { /* 无 metric-samples 目录 */ }
    return values;
  };
  const rangeOf = (points) => {
    const values = points.map((p) => p.value);
    if (!values.length) return '-';
    const midPeakCount = points.filter((p) => p.midPeak).length;
    return `min=${Math.min(...values)} max=${Math.max(...values)} last=${values[values.length - 1]}（${points.length} 点，其中峰值中采样 ${midPeakCount}）`;
  };
  const roundQps = roundMetricPoints('simulator.qps');
  if (roundQps.length || midPeakSamples.length) {
    lines.push(`## 轮内指标（每轮 + 峰值中采样）`, `- simulator.qps：${rangeOf(roundQps)}`, `- simulator.queue：${rangeOf(roundMetricPoints('simulator.queue'))}`, `- simulator.ttft：${rangeOf(roundMetricPoints('simulator.ttft'))}`, `- simulator.errorRate：${rangeOf(roundMetricPoints('simulator.errorRate'))}`, `- controller.errorRate：${rangeOf(roundMetricPoints('controller.errorRate'))}`, `- 说明：轮次粒度可能错过峰值中段的真实强度；精确序列以 PG resource_events（5s）为准。`, '');
  }

  // 快照指标（窗口内快照）
  const snapshots = [];
  try {
    const dir = path.join(RUN_DIR, 'snapshots');
    const files = readdirSync(dir).filter((f) => f.endsWith('.json')).sort();
    for (const file of files) {
      try {
        const snap = JSON.parse(readFileSync(path.join(dir, file), 'utf8'));
        if (inWindow(snap.ts)) snapshots.push(snap);
      } catch { /* skip */ }
    }
  } catch { /* 无快照 */ }
  if (snapshots.length) {
    const range = (metric) => {
      const values = snapshots.map((s) => s.metrics?.[metric]?.value).filter((v) => v != null && Number.isFinite(Number(v))).map(Number);
      if (!values.length) return '-';
      return `min=${Math.min(...values)} max=${Math.max(...values)} last=${values[values.length - 1]}`;
    };
    lines.push(`## 快照指标（${snapshots.length} 个）`, `- controller.errorRate：${range('controller.errorRate')}`, `- simulator.errorRate：${range('simulator.errorRate')}`, `- simulator.ttft：${range('simulator.ttft')}`, `- simulator.queue：${range('simulator.queue')}`, `- simulator.qps：${range('simulator.qps')}`, `- 说明：快照每 ${SNAPSHOT_EVERY} 轮一次，粒度更粗，峰值强度以「轮内指标」与 PG resource_events 为准。`, '');
  }

  lines.push(`## 结论与后续`, `- 完整数据：rounds/ 与 snapshots/ 目录（.runtime/longrun/${localDate()}/）`, `- 失败轮次的 keepalive/snapshot 明细见对应 round JSON 文件`, ``);
  return lines.join('\n');
}

async function main() {
  // 只重生成 summary：读现有 meta.json + rounds + snapshots，不动流量
  if (RESUMMARIZE) {
    if (!metaExists(RESUMMARIZE)) {
      process.stderr.write(`${utcNow()} --resummarize 需要目录下有 meta.json（含 startIso/endIso）\n`);
      process.exit(1);
    }
    redirectRunDir(RESUMMARIZE);
    const summary = buildSummary(null, RESUMMARIZE);
    writeFileSync(path.join(RESUMMARIZE, 'summary.md'), summary, 'utf8');
    process.stdout.write(`${summary}\n`);
    return;
  }
  const preflightChecks = await preflight();
  for (const check of preflightChecks) {
    process.stderr.write(`${utcNow()} preflight ${check.name}: ${check.ok ? 'ok' : 'WARN ' + (check.detail || '')}\n`);
  }
  // #48：轮次相位校验——峰值剧本必须能命中峰值窗口，否则整窗静默平峰。
  if (PEAK_QPS !== BASELINE_QPS && !phaseHitsPeak(INTERVAL / 60, CYCLE_MINUTES, PEAK_MINUTES)) {
    const phaseMsg = `轮次间隔(${INTERVAL}s) 与 cycle=${CYCLE_MINUTES}min/peak=${PEAK_MINUTES}min 相位错位，峰值不会生效（#48）。`
      + `建议 --interval 600（每 10 分钟一轮，可命中 30 分钟周期的 10 分钟峰值窗口）`
      + `或保持 --interval 900 + --cycle-minutes 60 --peak-minutes 15。`;
    process.stderr.write(`${utcNow()} WARN: ${phaseMsg}\n`);
    if (STRICT_PHASE) {
      process.stderr.write(`${utcNow()} --strict-phase 已设置，拒绝启动（#48）。\n`);
      process.exit(1);
    }
  }

  // 启动元数据：summary 只统计本次 run（[startIso, endIso] 窗口）
  try {
    mkdirSync(RUN_DIR, { recursive: true });
    writeFileSync(META_PATH, `${JSON.stringify({ runDir: RUN_DIR, startIso: START_ISO, pid: process.pid, args: args(), startedAt: utcNow() }, null, 2)}\n`, 'utf8');
  } catch (error) {
    process.stderr.write(`${utcNow()} meta.json 写入失败: ${String(error).slice(0, 200)}\n`);
  }
  let round = 0;
  do {
    if (round > 0 && shouldStop()) break;
    round += 1;
    const roundStart = Date.now();
    const snapshotDue = round % SNAPSHOT_EVERY === 0;
    await runOnce(round, snapshotDue, preflightChecks);
    if (!LOOP) return;
    if (shouldStop()) break;
    // 精确轮次间隔：按本轮实际耗时补足，且 sleep 不超过截止时间（避免 --until 超时多跑一轮）
    const elapsed = Date.now() - roundStart;
    const remaining = msUntilStop();
    if (remaining === 0) break;
    const wait = Math.max(5000, Math.min(INTERVAL * 1000 - elapsed, remaining));
    await sleep(wait);
  } while (LOOP);
  if (LOOP && shouldStop()) {
    await finish();
  }
}

function metaExists(dir) {
  try { return readFileSync(path.join(dir, 'meta.json'), 'utf8').length > 0; } catch { return false; }
}

if (process.argv[1] && import.meta.url === new URL(process.argv[1], 'file:').href) {
  main().catch((error) => {
    process.stderr.write(`${utcNow()} day-watch fatal: ${String(error.stack || error)}\n`);
    process.exit(1);
  });
}
