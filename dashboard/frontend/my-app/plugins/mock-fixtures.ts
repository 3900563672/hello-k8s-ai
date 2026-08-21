import type { Plugin } from 'vite'
import { readFileSync } from 'node:fs'
import path from 'node:path'

/**
 * dev:mock 数据层：vite --mode mock 时启用。
 * 拦截 /api/v1 GET 请求，从 src/lib/mocks/fixtures/ 返回真实录制快照，
 * 免后端浏览页面效果。写请求返回 405（只读模式）。
 */

const FIXTURE_DIR = path.resolve(import.meta.dirname, '../src/lib/mocks/fixtures')

const STATIC: Record<string, string> = {
    '/health/live': 'health-live.json',
    '/health/ready': 'health-ready.json',
    '/capabilities': 'capabilities.json',
    '/bootstrap': 'bootstrap.json',
    '/configuration': 'configuration.json',
    '/clock': 'clock.json',
    '/replay': 'replay.json',
    '/replay/frame': 'overview.json',
    '/resources': 'resources.json',
    '/events': 'events.json',
}

const METRIC_FIXTURES: Record<string, string> = {
    'simulator.ttft': 'metrics-simulator-ttft.json',
    'simulator.queue': 'metrics-simulator-queue.json',
    'simulator.qps': 'metrics-simulator-qps.json',
    'simulator.errorRate': 'metrics-simulator-error-rate.json',
    'simulator.tickLatency': 'metrics-simulator-tick-latency.json',
    'controller.errorRate': 'metrics-controller-error-rate.json',
}

const SNAPSHOT_AT = '2026-08-17T00:00:00Z'
const LATE_SNAPSHOT_AT = '2026-08-18T03:00:00Z'

function resolveFixture(pathname: string, params: URLSearchParams): string | null {
    if (STATIC[pathname]) return STATIC[pathname]

    const at = params.get('at')
    if (pathname === '/traffic') return at ? 'traffic-at-snapshot.json' : 'traffic.json'
    if (pathname === '/overview') {
        if (at === SNAPSHOT_AT) return 'overview-at-snapshot.json'
        if (at === LATE_SNAPSHOT_AT) return 'overview-at-late-snapshot.json'
        return 'overview.json'
    }
    if (pathname === '/segment') {
        const start = params.get('start')
        if (start === '2026-08-16T14:00:00Z') return 'segment-early.json'
        if (start === '2026-08-18T03:00:00Z') return 'segment-late.json'
        return 'segment.json'
    }
    if (pathname === '/experiments') {
        const status = params.get('status')
        if (status) return `experiments-status-${status}.json`
        return 'experiments.json'
    }
    if (pathname === '/traces') {
        const start = params.get('start')
        return start === '2026-08-18T03:00:00Z' ? 'traces-late-window.json' : 'traces.json'
    }
    if (pathname.startsWith('/traces/')) {
        const traceId = pathname.slice('/traces/'.length)
        if (/^[0-9a-f]{32}$/.test(traceId)) return `trace-${traceId}.json`
    }
    if (pathname === '/metrics') {
        const metricId = params.get('metricId')
        if (metricId && METRIC_FIXTURES[metricId]) return METRIC_FIXTURES[metricId]
    }
    return null
}

const cache = new Map<string, string | null>()

function loadFixture(name: string): string | null {
    if (cache.has(name)) return cache.get(name) ?? null
    const file = path.join(FIXTURE_DIR, name)
    try {
        const body = readFileSync(file, 'utf-8')
        cache.set(name, body)
        return body
    } catch {
        cache.set(name, null)
        return null
    }
}

export function mockFixturesPlugin(): Plugin {
    return {
        name: 'mock-fixtures',
        configureServer(server) {
            server.middlewares.use('/api/v1', (req, res) => {
                const url = new URL(req.url ?? '/', 'http://localhost')
                const pathname = url.pathname

                if (pathname === '/stream') {
                    res.statusCode = 204
                    res.end()
                    return
                }
                if (req.method !== 'GET') {
                    res.statusCode = 405
                    res.setHeader('Content-Type', 'application/json')
                    res.end(JSON.stringify({
                        error: {
                            code: 'MOCK_READ_ONLY',
                            message: 'dev:mock 只读模式，不支持写操作。',
                        },
                    }))
                    return
                }

                const fixture = resolveFixture(pathname, url.searchParams)
                let body = fixture ? loadFixture(fixture) : null
                if (body === null) {
                    res.statusCode = 404
                    res.setHeader('Content-Type', 'application/json')
                    res.end(JSON.stringify({
                        error: {
                            code: 'MOCK_NOT_FOUND',
                            message: `dev:mock 无匹配 fixture：${pathname}`,
                        },
                    }))
                    return
                }
                res.setHeader('Content-Type', 'application/json')
                res.setHeader('Connection', 'close')
                res.end(body)
            })
        },
    }
}
