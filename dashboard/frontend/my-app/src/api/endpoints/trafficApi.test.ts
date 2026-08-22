// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setTenantTraffic, trafficApi } from '@/api/endpoints/trafficApi'

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

describe('trafficApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => { originalFetch = globalThis.fetch })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('getTenants 映射字段并使用 displayName 回退', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const tenants = await trafficApi.getTenants()
        expect(tenants).toEqual([
            { id: 'tenant-a', name: '租户A', priority: 'P1', requestedQPS: 100, allocatedQPS: 80, runtimePhase: 'running' },
            { id: 'tenant-b', name: 'tenant-b', priority: 'P2', requestedQPS: 20, allocatedQPS: 20, runtimePhase: undefined },
        ])
    })

    it('getTenants 带时间戳时追加 at 参数；时间零点不追加', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        await trafficApi.getTenants('2026-08-20T00:00:00.000Z')
        expect(String(mockCalls()[0][0])).toContain('/traffic?at=2026-08-20T00%3A00%3A00.000Z')
        await trafficApi.getTenants(new Date(0).toISOString())
        expect(String(mockCalls()[1][0])).toBe('/api/v1/traffic')
    })

    it('getTenantTraffic 命中返回序列，未命中返回 null', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const hit = await trafficApi.getTenantTraffic('tenant-a')
        expect(hit).toEqual({ tenantId: 'tenant-a', tenantName: '租户A', timeSeconds: [0], values: [100] })
        const miss = await trafficApi.getTenantTraffic('tenant-x')
        expect(miss).toBeNull()
    })

    it('getAllTenantsTraffic 输出全部租户序列', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const series = await trafficApi.getAllTenantsTraffic()
        expect(series).toHaveLength(2)
        expect(series[1]).toEqual({ tenantId: 'tenant-b', tenantName: 'tenant-b', timeSeconds: [0], values: [20] })
    })

    it('getTotalQPS 汇总全部请求 QPS', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(trafficBody()))
        const total = await trafficApi.getTotalQPS()
        expect(total).toEqual({ tenantId: 'total', tenantName: '全部租户', timeSeconds: [0], values: [120] })
    })

    it('setTenantTraffic PATCH 清洗 qps 并带幂等头', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { results: [{ resourceVersion: '18', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        const receipt = await setTenantTraffic('tenant-a', -5.6, '17')
        expect(receipt).toEqual({ tenantId: 'tenant-a', qps: 0, resourceVersion: '18', convergence: 'converged' })
        const [url, init] = mockCalls()[0]
        expect(String(url)).toBe('/api/v1/tenants/tenant-a/traffic')
        expect(init?.method).toBe('PATCH')
        expect(JSON.parse(String(init?.body))).toEqual({ qps: 0, resourceVersion: '17', dryRun: false })
        expect(new Headers(init?.headers).get('Idempotency-Key')).toMatch(/^tenant-traffic-/)
    })

    it('setTenantTraffic 无结果时抛错，无 resourceVersion 时回退入参', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { results: [] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        await expect(setTenantTraffic('tenant-a', 10, '17')).rejects.toThrow('Backend 未返回流量更新结果')

        globalThis.fetch = vi.fn(async () => okResponse({
            data: { results: [{ convergence: 'pending' }] },
            meta: { requestId: 'r2', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        const receipt = await setTenantTraffic('tenant-a', 10, '17')
        expect(receipt.resourceVersion).toBe('17')
        expect(receipt.qps).toBe(10)
    })
})