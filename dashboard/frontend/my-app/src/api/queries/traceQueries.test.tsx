// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { createQueryClient, resetReplayStore, wrapperFor } from '@/test/queryUtils'
import { useOverview, useSegment, useTraceDetail } from '@/api/queries/traceQueries'
import { useTimeStore } from '@/stores/timeSlice'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const overviewEnvelope = () => ({
    data: {
        asOf: '2026-08-20T00:00:00.000Z',
        availability: 'available' as const,
        metrics: {},
        workloads: { nodes: [], pods: [], events: [] },
        traffic: { asOf: '2026-08-20T00:00:00.000Z', tenants: [] },
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('trace queries', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
    })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('useOverview latest 模式请求 /overview 且不带 at', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(overviewEnvelope()))
        const client = createQueryClient()
        const replay = useTimeStore.getState()
        const { result } = renderHook(
            () => useOverview({ effectiveAt: replay.timestamp, snapshotId: replay.selectedSnapshotId, mode: replay.mode, revision: replay.revision }),
            { wrapper: wrapperFor(client) },
        )
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toBe('/api/v1/overview')
    })

    it('useOverview historical 模式带 at 且缓存键含快照', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(overviewEnvelope()))
        useTimeStore.setState({
            timestamp: '2026-08-20T00:00:00.000Z',
            selectedSnapshotId: 'snap-1',
            mode: 'historical',
            revision: 1,
        })
        const client = createQueryClient()
        const replay = useTimeStore.getState()
        const { result } = renderHook(
            () => useOverview({ effectiveAt: replay.timestamp, snapshotId: replay.selectedSnapshotId, mode: replay.mode, revision: replay.revision }),
            { wrapper: wrapperFor(client) },
        )
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/overview?at=2026-08-20T00%3A00%3A00.000Z')
        expect(client.getQueryData(['trace', 'overview', 'historical', 'snap-1', null, null, null])).toBeDefined()
    })

    it('useSegment 无查询条件时禁用', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta: {} }))
        const client = createQueryClient()
        const { result } = renderHook(() => useSegment(null), { wrapper: wrapperFor(client) })
        expect(result.current.isPending).toBe(true)
        expect(mockCalls()).toHaveLength(0)
    })

    it('useSegment 组装时间窗请求', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta: {} }))
        const client = createQueryClient()
        const { result } = renderHook(
            () => useSegment({ start: '2026-08-20T00:00:00.000Z', end: '2026-08-20T01:00:00.000Z', tenantId: 'tenant-a' }),
            { wrapper: wrapperFor(client) },
        )
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/segment?')
        expect(String(mockCalls()[0][0])).toContain('tenant=tenant-a')
    })

    it('useTraceDetail null 禁用，有 traceId 时请求', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: {}, meta: {} }))
        const client = createQueryClient()
        const { result: disabled } = renderHook(() => useTraceDetail(null), { wrapper: wrapperFor(client) })
        expect(disabled.current.isPending).toBe(true)
        expect(mockCalls()).toHaveLength(0)

        const { result: loaded } = renderHook(() => useTraceDetail('trace-1'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(loaded.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/traces/trace-1')
    })
})