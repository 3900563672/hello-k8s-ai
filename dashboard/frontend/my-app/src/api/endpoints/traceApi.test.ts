// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchOverview, fetchSegment, fetchTrace } from '@/api/endpoints/traceApi'
import type { OverviewEnvelope } from '@/types/trace.types'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const overviewEnvelope = (): OverviewEnvelope => ({
    data: {
        asOf: '2026-08-20T00:00:00.000Z',
        availability: 'available',
        clock: {
            serverTime: '2026-08-20T00:00:00.000Z',
            actualTime: '2026-08-20T00:00:00.000Z',
            logicalTime: '2026-08-20T00:00:00.000Z',
            rate: 1,
            state: 'running',
            authoritative: true,
            capabilities: { simulatorAcceleration: true },
        },
        configuration: {
            asOf: '2026-08-20T00:00:00.000Z',
            availability: 'available',
            models: [],
            workerNodes: [],
            tenants: [],
            policies: { tenantModel: [], tenantNode: [], modelNode: [] },
            orchestrators: [],
            simulationClocks: [],
            simulatorInstances: [],
            tenantPerformance: [],
            tenantRuntimes: [],
        },
        metrics: {},
        workloads: { nodes: [], pods: [], deployments: [], services: [], leases: [], events: [] },
        traffic: { asOf: '2026-08-20T00:00:00.000Z', tenants: [] },
        traces: [],
        freshness: {},
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

const baseQuery = { effectiveAt: '2026-08-20T00:00:00.000Z', snapshotId: null, mode: 'latest' as const, revision: 0 }

describe('traceApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => { originalFetch = globalThis.fetch })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('fetchOverview latest 模式不带 at 参数', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(overviewEnvelope()))
        await fetchOverview(baseQuery)
        expect(String(mockCalls()[0][0])).toBe('/api/v1/overview')
    })

    it('fetchOverview historical 模式带 at 与筛选参数', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(overviewEnvelope()))
        await fetchOverview({
            ...baseQuery,
            mode: 'historical',
            snapshotId: 'snap-1',
            tenantId: 'tenant-a',
            modelId: 'model-b',
            instanceId: 'inst c',
        })
        const url = String(mockCalls()[0][0])
        expect(url).toContain('/overview?')
        expect(url).toContain('at=2026-08-20T00%3A00%3A00.000Z')
        expect(url).toContain('tenant=tenant-a')
        expect(url).toContain('model=model-b')
        expect(url).toContain('instance=inst+c')
    })

    it('fetchSegment 组装时间窗与筛选', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta: {} }))
        await fetchSegment({
            start: '2026-08-20T00:00:00.000Z',
            end: '2026-08-20T01:00:00.000Z',
            tenantId: 'tenant-a',
        })
        const url = String(mockCalls()[0][0])
        expect(url).toContain('/segment?')
        expect(url).toContain('start=2026-08-20T00%3A00%3A00.000Z')
        expect(url).toContain('end=2026-08-20T01%3A00%3A00.000Z')
        expect(url).toContain('tenant=tenant-a')
    })

    it('fetchTrace 编码 traceId', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: {}, meta: {} }))
        await fetchTrace('trace/1')
        expect(String(mockCalls()[0][0])).toContain('/traces/trace%2F1')
    })
})