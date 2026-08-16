#!/usr/bin/env node
// 夜间长时运行：健康检查 + 断线自恢复
// 用法：
//   node hack/night-run/keepalive.mjs --once                 # 单次检查，全绿退出码 0
//   node hack/night-run/keepalive.mjs --loop --interval 900  # 常驻循环（默认 900s）
//   node hack/night-run/keepalive.mjs --base-url http://localhost:8080
// 前置：WSL 内 node >= 22；集群已启动（bash setup.sh 或已部署）；本地端口转发已建立。
// 输出：stdout 为 JSON 行（Agent 解析用），失败项同时写 stderr；一切时间戳为 UTC。
import { execFileSync } from 'node:child_process';

const args = process.argv.slice(2);
const get = (name, dflt) => {
  const i = args.indexOf(name);
  return i >= 0 && args[i + 1] ? args[i + 1] : dflt;
};
const BASE = get('--base-url', 'http://localhost:8080');
const INTERVAL = Number(get('--interval', '900'));
const LOOP = args.includes('--loop');
const NAMESPACE = process.env.NAMESPACE || 'hello-k8s-ai-system';
const REPO = '/root/hello-k8s-ai';

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const utcNow = () => new Date().toISOString();

async function httpGet(path, timeoutMs = 8000) {
  // port-forward 偶发连接复用失败：网络层错误自动重试（最多 3 次，间隔 500ms）
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const response = await fetch(`${BASE}${path}`, { signal: controller.signal });
      const body = await response.text();
      let json = null;
      try { json = JSON.parse(body); } catch { /* 非 JSON 响应保留原文 */ }
      return { status: response.status, ok: response.ok, json, body: body.slice(0, 500) };
    } catch (error) {
      if (attempt === 3) {
        return { status: 0, ok: false, json: null, body: String(error.message || error).slice(0, 500) };
      }
      await sleep(500);
    } finally {
      clearTimeout(timer);
    }
  }
  return { status: 0, ok: false, json: null, body: 'unreachable' };
}

function runKubectl(extraArgs) {
  try {
    const out = execFileSync('kubectl', extraArgs, {
      encoding: 'utf8', timeout: 15000,
      env: { ...process.env, NAMESPACE },
    });
    return { ok: true, out: out.trim() };
  } catch (error) {
    return { ok: false, error: String(error.stderr || error.message).slice(0, 300) };
  }
}

function podSummary() {
  const result = runKubectl(['get', 'pods', '-n', NAMESPACE, '-o', 'json']);
  if (!result.ok) return { ok: false, error: result.error };
  try {
    const items = JSON.parse(result.out).items || [];
    const simulator = items.filter((pod) => pod.metadata?.labels?.['app.kubernetes.io/name'] === 'simulator'
      || pod.metadata?.name?.startsWith('simulator-'));
    const running = simulator.filter((pod) => pod.status?.phase === 'Running');
    const ready = simulator.filter((pod) =>
      (pod.status?.containerStatuses || []).every((cs) => cs.ready));
    const leader = simulator.find((pod) => (pod.metadata?.annotations || {})['lease.leader'] === 'true')
      || simulator.find((pod) => (pod.metadata?.annotations || {})['simulator.leader'] === 'true');
    return {
      ok: true,
      simulatorPods: simulator.length,
      runningPods: running.length,
      readyPods: ready.length,
      leader: leader?.metadata?.name || (items.find((pod) => pod.metadata?.name?.startsWith('simulator-'))?.metadata?.name ?? null),
      totalPods: items.length,
    };
  } catch (error) {
    return { ok: false, error: String(error).slice(0, 300) };
  }
}

async function checkOnce() {
  const checks = [];

  // 1) 健康探针
  const live = await httpGet('/api/v1/health/live');
  checks.push({ name: 'health/live', ok: live.ok, status: live.status, detail: live.json?.data?.status || live.body });
  const ready = await httpGet('/api/v1/health/ready');
  checks.push({
    name: 'health/ready', ok: ready.ok, status: ready.status,
    detail: ready.ok ? `db=${ready.json?.data?.checks?.database?.available} cache=${ready.json?.data?.checks?.kubernetesCache?.ready}` : ready.body,
  });

  // 2) 流量档位
  const traffic = await httpGet('/api/v1/traffic');
  if (traffic.ok) {
    const tenants = (traffic.json?.data?.tenants || []).map((t) => `${t.tenant?.name || '?'}:${t.allocatedQPS ?? t.requestedQPS ?? '?'}qps`).join(', ');
    checks.push({ name: 'traffic', ok: true, status: traffic.status, detail: tenants || '(no tenants)' });
  } else {
    checks.push({ name: 'traffic', ok: false, status: traffic.status, detail: traffic.body });
  }

  // 3) 时钟与模拟器状态
  const overview = await httpGet('/api/v1/overview');
  if (overview.ok) {
    const data = overview.json?.data || {};
    checks.push({
      name: 'overview', ok: true, status: overview.status,
      detail: `clock=${data.clock?.state} rate=${data.clock?.appliedRate ?? data.clock?.rate} tenants=${(data.configuration?.tenants || []).length}`,
    });
  } else {
    checks.push({ name: 'overview', ok: false, status: overview.status, detail: overview.body });
  }

  // 4) Reconcile 错误比例（Prometheus 经 Dashboard 代理）
  const metrics = await httpGet('/api/v1/metrics?metricId=controller.errorRate&step=300s');
  if (metrics.ok) {
    const series = metrics.json?.data?.series || [];
    const last = series.map((s) => {
      const values = s.values || s.points || [];
      return values.length ? values[values.length - 1] : null;
    }).filter(Boolean).slice(0, 3);
    checks.push({ name: 'simulator.errorRate', ok: true, status: metrics.status, detail: JSON.stringify(last) });
  } else {
    checks.push({ name: 'simulator.errorRate', ok: false, status: metrics.status, detail: metrics.body });
  }

  // 5) Pod 状态
  const pods = podSummary();
  if (pods.ok) {
    checks.push({ name: 'pods', ok: pods.readyPods >= 1, status: 0, detail: JSON.stringify(pods) });
  } else {
    checks.push({ name: 'pods', ok: false, status: 0, detail: pods.error });
  }

  const failed = checks.filter((c) => !c.ok);
  const record = {
    ts: utcNow(),
    ok: failed.length === 0,
    checks,
  };
  process.stdout.write(`${JSON.stringify(record)}\n`);
  if (failed.length > 0) {
    process.stderr.write(`${utcNow()} FAIL: ${failed.map((c) => c.name).join(', ')}\n`);
  }
  return failed.length === 0;
}

async function restorePortForward() {
  process.stderr.write(`${utcNow()} 端口转发疑似断开，尝试 make cluster-open 恢复...\n`);
  try {
    execFileSync('make', ['cluster-open'], { cwd: REPO, encoding: 'utf8', timeout: 60000, stdio: 'pipe' });
    process.stderr.write(`${utcNow()} make cluster-open 执行完成\n`);
    return true;
  } catch (error) {
    process.stderr.write(`${utcNow()} make cluster-open 失败: ${String(error.stderr || error.message).slice(0, 300)}\n`);
    return false;
  }
}

async function main() {
  const health = await httpGet('/api/v1/health/live', 5000);
  if (!health.ok && !LOOP) {
    process.stderr.write(`${utcNow()} Backend 不可达（${BASE}），先恢复端口转发再检查\n`);
    await restorePortForward();
  }

  let allOk = await checkOnce();
  // 仅当健康探针不可达（端口转发断了）时才恢复并重试；指标类失败不触发恢复
  if (!allOk) {
    const live = await httpGet('/api/v1/health/live', 5000);
    if (!live.ok) {
      await restorePortForward();
      await sleep(3000);
      allOk = await checkOnce();
    }
  }

  if (!LOOP) {
    process.exit(allOk ? 0 : 1);
  }

  process.stderr.write(`${utcNow()} 进入常驻循环，每 ${INTERVAL}s 检查一次（Ctrl-C 退出）\n`);
  // eslint-disable-next-line no-constant-condition
  while (true) {
    await sleep(INTERVAL * 1000);
    try {
      await checkOnce();
    } catch (error) {
      process.stderr.write(`${utcNow()} 检查异常: ${String(error).slice(0, 300)}\n`);
    }
  }
}

main().catch((error) => {
  process.stderr.write(`${utcNow()} keepalive 致命错误: ${String(error).slice(0, 500)}\n`);
  process.exit(2);
});