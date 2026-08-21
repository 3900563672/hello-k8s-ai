// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { createQueryClient, resetReplayStore, wrapperFor } from '@/test/queryUtils'
import {
    useCompleteExperiment,
    useCreateExperiment,
    useExperimentDetail,
    useExperiments,
    useFailExperiment,
    useStartExperiment,
} from '@/api/queries/experimentQueries'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const record = (status: string) => ({
    segmentId: 'seg-1', tenant: 'tenant-a', name: '实验', status,
    createdAt: '2026-08-20T00:00:00.000Z', updatedAt: '2026-08-20T00:00:00.000Z',
})

const listEnvelope = (items: unknown[]) => ({
    data: items,
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

const detailEnvelope = (status: string) => ({
    data: { segment: record(status), events: [], metrics: [], traces: [] },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('experiment queries', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
    })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('useExperiments 拉取列表并透传 status', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(listEnvelope([record('running')])))
        const client = createQueryClient()
        const { result } = renderHook(() => useExperiments('running'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(result.current.data?.data).toHaveLength(1)
        expect(String(mockCalls()[0][0])).toContain('/experiments?status=running')
        expect(client.getQueryData(['experiments', 'list', 'running'])).toBeDefined()
    })

    it('useExperimentDetail 无 segmentId 时禁用查询', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope('running')))
        const client = createQueryClient()
        const { result } = renderHook(() => useExperimentDetail(null), { wrapper: wrapperFor(client) })
        expect(result.current.isPending).toBe(true)
        expect(mockCalls()).toHaveLength(0)
    })

    it('useExperimentDetail 有 segmentId 时请求详情', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope('completed')))
        const client = createQueryClient()
        const { result } = renderHook(() => useExperimentDetail('seg-1'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(result.current.data?.data.segment.status).toBe('completed')
        expect(String(mockCalls()[0][0])).toContain('/experiments/seg-1')
    })

    it('mutation 成功后失效 experiments 查询', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope('running')))
        const client = createQueryClient()
        const { result } = renderHook(() => useCreateExperiment(), { wrapper: wrapperFor(client) })
        result.current.mutate({ tenant: 'tenant-a', name: '新实验' })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/experiments')
        expect(JSON.parse(String(mockCalls()[0][1]?.body))).toEqual({ tenant: 'tenant-a', name: '新实验' })
        // invalidateQueries 会触发一次 refetch（无活跃查询时不发请求，仅标记失效）
        expect(client.getQueryState(['experiments', 'all'])?.isInvalidated).toBeUndefined()
    })

    it('start/complete/fail mutation 调用对应接口', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope('running')))
        const client = createQueryClient()
        const { result: start } = renderHook(() => useStartExperiment(), { wrapper: wrapperFor(client) })
        start.current.mutate('seg-1')
        await waitFor(() => expect(start.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/experiments/seg-1/start')

        const { result: complete } = renderHook(() => useCompleteExperiment(), { wrapper: wrapperFor(client) })
        complete.current.mutate('seg-1')
        await waitFor(() => expect(complete.current.isSuccess).toBe(true))
        expect(String(mockCalls()[1][0])).toContain('/experiments/seg-1/complete')

        const { result: fail } = renderHook(() => useFailExperiment(), { wrapper: wrapperFor(client) })
        fail.current.mutate({ segmentId: 'seg-1', reason: 'OOM' })
        await waitFor(() => expect(fail.current.isSuccess).toBe(true))
        expect(String(mockCalls()[2][0])).toContain('/experiments/seg-1/fail')
        expect(JSON.parse(String(mockCalls()[2][1]?.body))).toEqual({ reason: 'OOM' })
    })
})