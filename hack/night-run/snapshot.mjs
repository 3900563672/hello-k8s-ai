#!/usr/bin/env node
// 夜间长时运行：指标快照采集
// 用法：
//   node hack/night-run/snapshot.mjs --once                 # 抓一次快照（默认）
//   node hack/night-run/snapshot.mjs --once --summary       # 抓取后输出人读摘要
//   node hack/night-run/snapshot.mjs --date 2026-08-17      # 指定运行日期目录（本地时区）
// 输出：快照写入 .runtime/night-run/<日期>/snapshots/<UTC时间>.json（不入库）；
//       摘要写 stdout（Agent 解析与人工核对用）。
// 前置：WSL 内 node >= 22；Backend 本地端口可达（keepalive.mjs 负责恢复）。
import { writeFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';

const args = process.argv.slice(2);
const get = (name, dflt) => {
  const i = args.indexOf(name);
  return i >= 0 && args[i + 1] ? args[i + 1] : dflt;
};
const BASE = get('--base-url', 'http://localhost:8080');
// 默认用本地时区日期（Asia/Shanghai），避免跨 UTC 日边界时归档到错误目录
const localDate = () => {
  const now = new Date();
  const shifted = new Date(now.getTime() + 8 * 3600 * 1000);
  return shifted.toISOString().slice(0, 10);
};
const RUN_DATE = get('--date', localDate());
const SUMMARY = args.includes('--summary');
const METRIC_IDS = ['simulator.errorRate', 'simulator.ttft', 'simulator.queue', 'simulator.qps', 'simulator.tickLatency'];
const SNAP_DIR = join('/root/hello-k8s-ai/.runtime/night-run', RUN_DATE, 'snapshots');

const utcNow = () => new Date().toISOString();

async function httpGet(path, timeoutMs = 30000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(`${BASE}${path}`, { signal: controller.signal });
    const body = await response.text();
    let json = null;
    try { json = JSON.parse(body); } catch { /* 保留原文 */ }
    return { ok: response.ok, status: response.status, json, body: body.slice(0, 300) };
  } catch (error) {
    return { ok: false, status: 0, json: null, body: String(error.message || error).slice(0, 300) };
  } finally {
    clearTimeout(timer);
  }
}

function latestValue(result) {
  if (!result?.json?.data) return null;
  const series = result.json.data.series || [];
  const values = series.flatMap((s) => s.values || s.points || []);
  if (!values.length) return null;
  const last = values[values.length - 1];
  // 支持 [t, v] 数组与 {time, value} 对象两种形态
  if (Array.isArray(last)) return { time: last[0], value: last[1] };
  return last;
}

async function collect() {
  const ts = utcNow();
  const snapshot = { ts, collectedAt: ts };

  // 1) 时钟/配置/租户/实例（overview）
  const overview = await httpGet('/api/v1/overview');
  if (overview.ok) {
    const data = overview.json?.data || {};
    const configuration = data.configuration || {};
    const tenants = (data.traffic?.tenants || []);
    const instances = tenants.flatMap((t) => (t.instances || []).map((i) => ({
      instance: i.name,
      tenant: i.tenant,
      model: i.model,
      desiredReplicas: i.desiredReplicas,
      availableReplicas: i.availableReplicas,
      assignedQPS: i.assignedQPS,
      effectiveScore: i.effectiveScore,
      phase: i.phase,
    })));
    snapshot.overview = {
      clockState: data.clock?.state,
      rate: data.clock?.appliedRate ?? data.clock?.rate,
      logicalTime: data.clock?.logicalTime,
      tenantCount: (configuration.tenants || []).length,
      modelCount: (configuration.models || []).length,
      nodeCount: (configuration.workerNodes || []).length,
      instances,
      leaseCount: (data.workloads?.leases || []).length,
    };
  } else {
    snapshot.overviewError = { status: overview.status, body: overview.body };
  }

  // 2) 指标（errorRate / ttft / queue / qps / tickLatency）——并行抓取避免串行超时
  snapshot.metrics = {};
  const metricResults = await Promise.all(
    METRIC_IDS.map(async (metricId) => {
      const result = await httpGet(`/api/v1/metrics?metricId=${metricId}&step=300s`);
      return { metricId, result };
    }),
  );
  for (const { metricId, result } of metricResults) {
    snapshot.metrics[metricId] = latestValue(result);
    if (!result.ok) snapshot.metrics[`${metricId}Error`] = { status: result.status, body: result.body };
  }

  // 3) 流量档位
  const traffic = await httpGet('/api/v1/traffic');
  if (traffic.ok) {
    snapshot.traffic = (traffic.json?.data?.tenants || []).map((t) => ({
      tenant: t.tenant?.name,
      displayName: t.displayName,
      requestedQPS: t.requestedQPS,
      allocatedQPS: t.allocatedQPS,
      runtimePhase: t.runtimePhase,
      readyReplicaCount: t.readyReplicaCount,
      avgTTFT: t.performance?.avgTTFT?.value,
    }));
  } else {
    snapshot.trafficError = { status: traffic.status, body: traffic.body };
  }

  // 4) 资源与 Pod（resources）
  const resources = await httpGet('/api/v1/resources');
  if (resources.ok) {
    const states = resources.json?.data?.states || [];
    const byKind = {};
    for (const s of states) byKind[s.kind] = (byKind[s.kind] || 0) + 1;
    snapshot.resources = {
      count: resources.json?.data?.count ?? states.length,
      byKind,
      deployments: states.filter((s) => s.kind === 'Deployment').map((s) => `${s.name}:${s.payload?.status?.readyReplicas ?? '?'}/${s.payload?.status?.replicas ?? '?'}`),
      podNames: states.filter((s) => s.kind === 'Pod').map((s) => s.name),
    };
  } else {
    snapshot.resourcesError = { status: resources.status, body: resources.body };
  }

  // 5) 数据库与缓存状态（ready 探针明细）
  const ready = await httpGet('/api/v1/health/ready');
  if (ready.ok) {
    const checks = ready.json?.data?.checks || {};
    snapshot.health = {
      database: checks.database,
      kubernetesCache: checks.kubernetesCache,
      prometheus: checks.prometheus,
      jaeger: checks.jaeger,
    };
  } else {
    snapshot.healthError = { status: ready.status, body: ready.body };
  }

  return snapshot;
}

function summarize(snapshot) {
  const m = snapshot.metrics || {};
  const fmt = (v) => {
    if (v == null) return '-';
    if (typeof v === 'object') return `${v.value ?? v.v ?? JSON.stringify(v)}`;
    return `${v}`;
  };
  const instanceSummary = (snapshot.overview?.instances || [])
    .map((i) => `${i.instance}:${i.availableReplicas}/${i.desiredReplicas}@${i.assignedQPS}qps score=${i.effectiveScore}`)
    .join('; ');
  return [
    `snapshot ${snapshot.collectedAt}`,
    `  clock=${snapshot.overview?.clockState} rate=${snapshot.overview?.rate}`,
    `  tenants=${snapshot.overview?.tenantCount} models=${snapshot.overview?.modelCount} nodes=${snapshot.overview?.nodeCount} leases=${snapshot.overview?.leaseCount}`,
    `  errorRate=${fmt(m['simulator.errorRate'])} ttft=${fmt(m['simulator.ttft'])} queue=${fmt(m['simulator.queue'])} qps=${fmt(m['simulator.qps'])} tickLatency=${fmt(m['simulator.tickLatency'])}`,
    `  instances: ${instanceSummary || '-'}`,
    `  traffic=${JSON.stringify(snapshot.traffic?.map((t) => `${t.tenant}=${t.allocatedQPS}qps(${t.runtimePhase})`) || snapshot.trafficError || [])}`,
    `  resources=${snapshot.resources?.count ?? '-'} (${JSON.stringify(snapshot.resources?.byKind ?? {})})`,
    `  db=${JSON.stringify(snapshot.health?.database ?? snapshot.healthError ?? {})}`,
  ].join('\n');
}

async function main() {
  const snapshot = await collect();
  mkdirSync(SNAP_DIR, { recursive: true });
  const file = join(SNAP_DIR, `${snapshot.collectedAt.replace(/[:.]/g, '-')}.json`);
  writeFileSync(file, `${JSON.stringify(snapshot, null, 2)}\n`, 'utf8');

  process.stdout.write(`${JSON.stringify({ file, collectedAt: snapshot.collectedAt, ok: true })}\n`);
  if (SUMMARY) {
    process.stdout.write(summarize(snapshot));
    process.stdout.write('\n');
  }
}

main().catch((error) => {
  process.stderr.write(`${utcNow()} snapshot 致命错误: ${String(error).slice(0, 500)}\n`);
  process.stderr.write(`${utcNow()} stack: ${String(error.stack || "").slice(0, 800)}\n`);
  process.exit(2);
});