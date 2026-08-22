// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { createQueryClient, resetReplayStore, wrapperFor } from '@/test/queryUtils'
import {
    useAllTenantsTraffic,
    useSetTenantTraffic,
    useTenantTraffic,
    useTenants,
    useTotalQPS,
} from '@/api/queries/trafficQueries'
import { useTimeStore } from '@/stores/timeSlice'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const trafficBody = () => ({
    data: {
        asOf: '2026-08-20T00:00:00.000Z',
        tenants: [
            { tenant: { name: 'tenant-a' }, displayName: '租户A', priority: 'P1', requestedQPS: 100, allocatedQPS: 80, runtimePhase: 'running' },
            { tenant: { name: 'tenant-b' }, displayName: '', priority: 'P2', requestedQPS: 20, allocatedQPS: 20 },
        ],
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('traffic queries', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
    })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('useTenants latest 不带 at，historical 带 at 且缓存键用快照', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const client = createQueryClient()
        const { result } = renderHook(() => useTenants(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(result.current.data).toHaveLength(2)
        expect(String(mockCalls()[0][0])).toBe('/api/v1/traffic')
        expect(client.getQueryData(['traffic', 'tenants', 'latest'])).toBeDefined()

        useTimeStore.setState({
            timestamp: '2026-08-20T00:00:00.000Z',
            selectedSnapshotId: 'snap-1',
            mode: 'historical',
            revision: 1,
        })
        const { result: historical } = renderHook(() => useTenants(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(historical.current.isSuccess).toBe(true))
        expect(String(mockCalls()[1][0])).toContain('/traffic?at=2026-08-20T00%3A00%3A00.000Z')
        expect(client.getQueryData(['traffic', 'tenants', 'snap-1'])).toBeDefined()
    })

    it('useTenantTraffic 无 tenantId 时禁用', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const client = createQueryClient()
        const { result } = renderHook(() => useTenantTraffic(null), { wrapper: wrapperFor(client) })
        expect(result.current.isPending).toBe(true)
        expect(mockCalls()).toHaveLength(0)
    })

    it('useTenantTraffic 命中时返回序列', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const client = createQueryClient()
        const { result } = renderHook(() => useTenantTraffic('tenant-a'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(result.current.data).toEqual({ tenantId: 'tenant-a', tenantName: '租户A', timeSeconds: [0], values: [100] })
    })

    it('useAllTenantsTraffic / useTotalQPS 使用聚合接口', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const client = createQueryClient()
        const { result: all } = renderHook(() => useAllTenantsTraffic(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(all.current.isSuccess).toBe(true))
        expect(all.current.data).toHaveLength(2)

        const { result: total } = renderHook(() => useTotalQPS(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(total.current.isSuccess).toBe(true))
        expect(total.current.data?.values).toEqual([120])
    })

    it('useSetTenantTraffic 提交 PATCH 并失效全部流量查询', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { results: [{ resourceVersion: '18', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        const client = createQueryClient()
        const { result } = renderHook(() => useSetTenantTraffic(), { wrapper: wrapperFor(client) })
        result.current.mutate({ tenantId: 'tenant-a', qps: 150, resourceVersion: '17' })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toBe('/api/v1/tenants/tenant-a/traffic')
        expect(JSON.parse(String(mockCalls()[0][1]?.body))).toEqual({ qps: 150, resourceVersion: '17', dryRun: false })
    })
})