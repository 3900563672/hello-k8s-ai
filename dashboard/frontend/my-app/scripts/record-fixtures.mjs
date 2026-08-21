#!/usr/bin/env node
/**
 * 从运行中的 Dashboard Backend 录制真实 API 快照到 src/lib/mocks/fixtures/。
 *
 * 用法：
 *   DASHBOARD_URL=http://localhost:8080/api/v1 node scripts/record-fixtures.mjs
 *
 * 输出：每个路由一个 JSON 文件（原样保存 Backend 响应，含 envelope/meta），
 *       外加 manifest.json 记录录制时间、来源与每项状态。
 * 说明：/stream 为 SSE 长连接，不录制；动态详情（实验、Trace）由列表自动发现。
 */

import { mkdir, readdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'

const BASE = (process.env.DASHBOARD_URL || 'http://localhost:8080/api/v1').replace(/\/$/, '')
const OUT_DIR = path.resolve(import.meta.dirname, '../src/lib/mocks/fixtures')

const SEGMENT_START = '2026-08-17T00:00:00Z'
const SEGMENT_END = '2026-08-17T01:00:00Z'
const SEGMENT_EARLY_START = '2026-08-16T14:00:00Z'
const SEGMENT_EARLY_END = '2026-08-16T15:00:00Z'
const SEGMENT_LATE_START = '2026-08-18T03:00:00Z'
const SEGMENT_LATE_END = '2026-08-18T04:00:00Z'
const METRIC_START = '2026-08-17T00:00:00Z'
const METRIC_END = '2026-08-17T01:00:00Z'

const STATIC_ROUTES = [
  ['health-live', '/health/live'],
  ['health-ready', '/health/ready'],
  ['capabilities', '/capabilities'],
  ['bootstrap', '/bootstrap'],
  ['configuration', '/configuration'],
  ['traffic', '/traffic'],
  ['traffic-at-snapshot', `/traffic?at=${encodeURIComponent(SEGMENT_START)}`],
  ['clock', '/clock'],
  ['events', '/events?limit=200'],
  ['resources', '/resources?limit=100'],
  ['replay', '/replay?limit=1000'],
  ['overview', '/overview'],
  ['overview-at-snapshot', `/overview?at=${encodeURIComponent(SEGMENT_START)}`],
  ['overview-at-late-snapshot', `/overview?at=${encodeURIComponent(SEGMENT_LATE_START)}`],
  ['segment', `/segment?start=${SEGMENT_START}&end=${SEGMENT_END}`],
  ['segment-early', `/segment?start=${SEGMENT_EARLY_START}&end=${SEGMENT_EARLY_END}`],
  ['segment-late', `/segment?start=${SEGMENT_LATE_START}&end=${SEGMENT_LATE_END}`],
  ['experiments', '/experiments'],
  ['experiments-status-pending', '/experiments?status=pending'],
  ['experiments-status-running', '/experiments?status=running'],
  ['experiments-status-completed', '/experiments?status=completed'],
  ['experiments-status-failed', '/experiments?status=failed'],
  ['traces', '/traces?limit=20'],
  ['traces-late-window', `/traces?start=${SEGMENT_LATE_START}&end=${SEGMENT_LATE_END}&limit=20`],
]

const METRIC_ROUTES = [
  ['metrics-simulator-ttft', `/metrics?metricId=simulator.ttft&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
  ['metrics-simulator-queue', `/metrics?metricId=simulator.queue&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
  ['metrics-simulator-qps', `/metrics?metricId=simulator.qps&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
  ['metrics-simulator-error-rate', `/metrics?metricId=simulator.errorRate&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
  ['metrics-simulator-tick-latency', `/metrics?metricId=simulator.tickLatency&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
  ['metrics-controller-error-rate', `/metrics?metricId=controller.errorRate&start=${METRIC_START}&end=${METRIC_END}&step=60s`],
]

const safeName = (value) => value.replace(/[^a-zA-Z0-9._-]/g, '-')

async function fetchJson(routePath, signal) {
  const response = await fetch(`${BASE}${routePath}`, {
    headers: { Accept: 'application/json' },
    signal,
  })
  const text = await response.text()
  let body = null
  try {
    body = JSON.parse(text)
  } catch {
    body = text
  }
  return { status: response.status, body }
}

async function collectDynamic(recordings, manifest, signal) {
  const detailRoutes = []
  for (const entry of recordings) {
    if (entry.status !== 200) continue
    const isTraceList = entry.name.startsWith('traces')
    const envelope = entry.body
    const payload = envelope?.data
    const data = isTraceList ? payload?.items : payload
    if (Array.isArray(data)) {
      for (const item of data) {
        const id = isTraceList ? (item?.traceId || item?.traceID || item?.id) : (item?.id || item?.segmentId || item?.name)
        if (!id) continue
        detailRoutes.push({ id, isTrace: isTraceList })
      }
    }
  }

  const result = []
  for (const { id, isTrace } of detailRoutes) {
    const routePath = isTrace
      ? `/traces/${encodeURIComponent(id)}`
      : `/experiments/${encodeURIComponent(id)}`
    const name = isTrace ? `trace-${safeName(id)}` : `experiment-${safeName(id)}`
    const recorded = await fetchJson(routePath, signal)
    manifest.push({ name, path: routePath, status: recorded.status, size: JSON.stringify(recorded.body).length })
    await writeFile(path.join(OUT_DIR, `${name}.json`), JSON.stringify(recorded.body, null, 2))
    result.push({ name, routePath, ...recorded })
  }
  return result
}

async function main() {
  await mkdir(OUT_DIR, { recursive: true })
  for (const file of await readdir(OUT_DIR)) {
    if (file.endsWith('.json') && file !== 'manifest.json') {
      await rm(path.join(OUT_DIR, file))
    }
  }

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 20_000)
  const manifest = []

  const routes = [...STATIC_ROUTES, ...METRIC_ROUTES]
  const recordings = []
  for (const [name, routePath] of routes) {
    const recorded = await fetchJson(routePath, controller.signal)
    const size = typeof recorded.body === 'string'
      ? recorded.body.length
      : JSON.stringify(recorded.body).length
    manifest.push({ name, path: routePath, status: recorded.status, size })
    await writeFile(path.join(OUT_DIR, `${name}.json`), JSON.stringify(recorded.body, null, 2))
    recordings.push({ name, routePath, ...recorded })
  }

  const dynamic = await collectDynamic(recordings, manifest, controller.signal)
  clearTimeout(timeout)

  await writeFile(path.join(OUT_DIR, 'manifest.json'), JSON.stringify({
    recordedAt: new Date().toISOString(),
    baseUrl: BASE,
    note: '真实录制快照，供 dev:mock 数据层使用；/stream(SSE) 不录制；动态详情由列表自动发现。',
    items: manifest,
    dynamicDetails: dynamic.map(({ name, routePath, status }) => ({ name, path: routePath, status })),
  }, null, 2))

  const failed = manifest.filter((item) => item.status !== 200)
  console.log(`recorded ${manifest.length + dynamic.length} fixtures -> ${OUT_DIR}`)
  if (failed.length > 0) {
    console.log(`non-200 (${failed.length}):`)
    for (const item of failed) console.log(`  ${item.status} ${item.path}`)
  }
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
