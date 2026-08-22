// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { createQueryClient, resetReplayStore, wrapperFor } from '@/test/queryUtils'
import {
    useCreateModel,
    useDeleteModel,
    useDeleteModels,
    useModels,
    useNodes,
    useOrchestrators,
    usePolicies,
    useTenants,
} from '@/api/queries/configQueries'
import { useTimeStore } from '@/stores/timeSlice'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const resource = (kind: string, name: string, spec: Record<string, unknown>) => ({
    ref: { apiVersion: 'v1', kind, name, uid: `uid-${name}` },
    metadata: { generation: 1, resourceVersion: '7' },
    spec,
    status: { phase: 'Ready' },
    conditions: [],
    derived: {},
})

const configuration = () => ({
    data: {
        asOf: '2026-08-20T00:00:00.000Z',
        availability: 'available' as const,
        models: [resource('Model', 'model-a', { displayName: '模型A', gpuUnits: 2, maxConcurrency: 4, absoluteScore: 80, coldStartMs: 100, performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 } })],
        workerNodes: [resource('WorkerNode', 'node-a', { displayName: '节点A', gpu: 8, maxConcurrency: 16 })],
        tenants: [resource('Tenant', 'tenant-a', { displayName: '租户A', priority: 'P1', qps: 100, ttftThresholdMs: 2000, queueThreshold: 50, ttftScaleDownThresholdMs: 1000, queueScaleDownThreshold: 20 })],
        orchestrators: [resource('Orchestrator', 'orch-a', { tenantRef: { name: 'tenant-a' }, scaleUpCooldownSeconds: 30, scaleDownCooldownSeconds: 30, allowScaleToZero: true, minReplicas: 1, maxReplicas: 3, maxScaleUpBatch: 1 })],
        policies: { tenantModel: [resource('TenantModelPolicy', 'p-tm', { tenantRef: { name: 'tenant-a' }, modelRef: { name: 'model-a' }, effect: 'Allow' })], tenantNode: [], modelNode: [] },
        simulationClocks: [],
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('config queries', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
    })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('useModels latest 请求 /configuration 且缓存键为 latest', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const client = createQueryClient()
        const { result } = renderHook(() => useModels(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(result.current.data).toHaveLength(1)
        expect(result.current.data?.[0]).toMatchObject({ name: 'model-a', displayName: '模型A' })
        expect(String(mockCalls()[0][0])).toBe('/api/v1/configuration')
        expect(client.getQueryData(['config', 'models', 'latest'])).toBeDefined()
    })

    it('useModels historical 带 at 且缓存键用快照', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        useTimeStore.setState({
            timestamp: '2026-08-20T00:00:00.000Z',
            selectedSnapshotId: 'snap-1',
            mode: 'historical',
            revision: 1,
        })
        const client = createQueryClient()
        const { result } = renderHook(() => useModels(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/configuration?at=2026-08-20T00%3A00%3A00.000Z')
        expect(client.getQueryData(['config', 'models', 'snap-1'])).toBeDefined()
    })

    it('useNodes/useTenants/useOrchestrators/usePolicies 各自解析对应集合', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const client = createQueryClient()
        const { result: nodes } = renderHook(() => useNodes(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(nodes.current.isSuccess).toBe(true))
        expect(nodes.current.data?.[0].name).toBe('node-a')

        const { result: tenants } = renderHook(() => useTenants(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(tenants.current.isSuccess).toBe(true))
        expect(tenants.current.data?.[0].name).toBe('tenant-a')

        const { result: orchestrators } = renderHook(() => useOrchestrators(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(orchestrators.current.isSuccess).toBe(true))
        expect(orchestrators.current.data?.[0].name).toBe('orch-a')

        const { result: policies } = renderHook(() => usePolicies(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(policies.current.isSuccess).toBe(true))
        expect(policies.current.data).toHaveLength(1)
        expect(policies.current.data?.[0].kind).toBe('tenantModel')
    })

    it('useCreateModel mutation 提交 :apply', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: { operationId: 'op-1', acceptedAt: '2026-08-20T00:00:00.000Z', state: 'applied', results: [{ ref: { kind: 'Model', name: 'model-a' }, action: 'create', convergence: 'converged' }] },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        const client = createQueryClient()
        const { result } = renderHook(() => useCreateModel(), { wrapper: wrapperFor(client) })
        result.current.mutate({
            name: 'model-a', displayName: '模型A', gpuUnits: 2, maxConcurrency: 4,
            absoluteScore: 80, coldStartMs: 100,
            performance: { prefillBaseMs: 10, prefillPerTokenUs: 5, decodePerTokenMs: 3 },
        })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toBe('/api/v1/configuration:apply')
    })

    it('useDeleteModel/useDeleteModels mutation 删除资源', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(configuration()))
        const client = createQueryClient()
        const { result: single } = renderHook(() => useDeleteModel(), { wrapper: wrapperFor(client) })
        single.current.mutate('model-a')
        await waitFor(() => expect(single.current.isSuccess).toBe(true))
        expect(String(mockCalls()[1][0])).toBe('/api/v1/configuration/Model/model-a')
        expect(mockCalls()[1][1]?.method).toBe('DELETE')

        const { result: batch } = renderHook(() => useDeleteModels(), { wrapper: wrapperFor(client) })
        batch.current.mutate(['model-a'])
        await waitFor(() => expect(batch.current.isSuccess).toBe(true))
        expect(mockCalls()).toHaveLength(4) // 2x getConfiguration + 2x DELETE
    })
})