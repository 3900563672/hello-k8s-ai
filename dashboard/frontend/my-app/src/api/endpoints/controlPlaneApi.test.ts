import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
    distributeConfiguration,
    fetchClusterSnapshot,
    updateSimulationRate,
} from '@/api/endpoints/controlPlaneApi'
import bootstrapFixture from '@/lib/mocks/fixtures/bootstrap.json'
import configurationFixture from '@/lib/mocks/fixtures/configuration.json'
import type { ClusterNode } from '@/types/control-plane.types'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })

const makeNode = (overrides: Partial<ClusterNode> = {}): ClusterNode => ({
    id: 'worker-1',
    name: 'worker-1',
    role: 'worker',
    status: 'running',
    ready: true,
    zone: 'zone-a',
    gpuCapacity: 2,
    ...overrides,
})

describe('controlPlaneApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('fetchClusterSnapshot 用 bootstrap 快照映射集群字段', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(bootstrapFixture))
        const cluster = await fetchClusterSnapshot()
        expect(cluster.name).toBe(bootstrapFixture.data.cluster.name)
        expect(cluster.connectionStatus).toBe('connected')
        expect(cluster.context).toBe(bootstrapFixture.data.cluster.context)
        expect(cluster.clockRate).toBe(bootstrapFixture.data.clock.rate)
        expect(cluster.clockAppliedRate).toBe(bootstrapFixture.data.clock.appliedRate)
        expect(cluster.clockSynchronizedInstances).toBe(bootstrapFixture.data.clock.synchronizedInstances)
        expect(cluster.clockTotalInstances).toBe(bootstrapFixture.data.clock.totalInstances)
        expect(cluster.simulationRateSupported).toBe(
            bootstrapFixture.data.clock.capabilities.canSetRate &&
            bootstrapFixture.data.clock.capabilities.simulatorAcceleration,
        )
        expect(cluster.controlPlane.role).toBe('control-plane')
        expect(cluster.workers.every((node) => node.role === 'worker')).toBe(true)
    })

    it('fetchClusterSnapshot 未同步时进入 connecting', async () => {
        const fixture = {
            ...bootstrapFixture,
            data: {
                ...bootstrapFixture.data,
                cluster: { ...bootstrapFixture.data.cluster, connected: true, cacheSynced: false },
            },
        }
        globalThis.fetch = vi.fn(async () => okResponse(fixture))
        const cluster = await fetchClusterSnapshot()
        expect(cluster.connectionStatus).toBe('connecting')
    })

    it('updateSimulationRate 提交 PATCH 并返回回执', async () => {
        const fetchMock = vi.fn<typeof globalThis.fetch>(async () => okResponse({
            data: { results: [{ resourceVersion: '18', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        globalThis.fetch = fetchMock
        const receipt = await updateSimulationRate(5, '17')
        expect(receipt).toEqual({ rate: 5, resourceVersion: '18', convergence: 'converged' })
        expect(String(fetchMock.mock.calls[0]?.[0])).toBe('/api/v1/clock/rate')
        expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('PATCH')
        expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ rate: 5, resourceVersion: '17', dryRun: false })
        expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get('Idempotency-Key')).toMatch(/^simulation-rate-/)
    })

    it('updateSimulationRate 无结果时抛错', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { results: [] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        await expect(updateSimulationRate(5, '17')).rejects.toThrow('Backend 未返回倍速更新结果')
    })

    it('distributeConfiguration 汇总配置数量与在线 Worker 数', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configurationFixture))
        const cluster = {
            id: 'cluster-1',
            workers: [makeNode(), makeNode({ id: 'worker-2', name: 'worker-2' })],
        }
        const receipt = await distributeConfiguration(cluster as Parameters<typeof distributeConfiguration>[0])
        const fixtureData = configurationFixture.data
        expect(receipt.acceptedNodes).toBe(2)
        expect(receipt.resources).toEqual({
            models: fixtureData.models.length,
            nodes: fixtureData.workerNodes.length,
            tenants: fixtureData.tenants.length,
            total: fixtureData.models.length + fixtureData.workerNodes.length + fixtureData.tenants.length,
            revision: configurationFixture.meta.sourceVersions.kubernetes,
        })
        expect(receipt.createdAt).toBe(configurationFixture.meta.servedAt)
    })
})
