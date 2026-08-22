// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { createQueryClient, resetReplayStore, wrapperFor } from '@/test/queryUtils'
import {
    useAIOpsAlerts,
    useAIOpsAnalysisBySegment,
    useAIOpsAnalyses,
    useAIOpsJobs,
    useAIOpsLimits,
    useAIOpsQuota,
    useAIOpsWindows,
    useConfirmAIOpsCommand,
    useCreateAIOpsCommand,
    useStopAIOpsCommand,
} from '@/api/queries/aiopsQueries'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const notEnabled = () => okResponse({ error: { code: 'AI_OPS_DISABLED', message: 'AIOps 未启用' }, meta }, 404)

describe('aiops queries', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        resetReplayStore()
    })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('useAIOpsQuota / useAIOpsLimits 请求对应路径', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/aiops/quota')) return okResponse({ data: { used: 1, limit: 10 }, meta })
            if (url.includes('/aiops/limits')) return okResponse({ data: { maxQPS: 500 }, meta })
            return notEnabled()
        })
        const client = createQueryClient()
        const { result: quota } = renderHook(() => useAIOpsQuota(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(quota.current.isSuccess).toBe(true))
        expect(quota.current.data?.data).toEqual({ used: 1, limit: 10 })

        const { result: limits } = renderHook(() => useAIOpsLimits(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(limits.current.isSuccess).toBe(true))
        expect(limits.current.data?.data).toEqual({ maxQPS: 500 })
    })

    it('useAIOpsAnalyses 携带 status 过滤', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta }))
        const client = createQueryClient()
        const { result } = renderHook(() => useAIOpsAnalyses('completed'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(result.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/aiops/analyses?status=completed&limit=50')
        expect(client.getQueryData(['aiops', 'analyses', 'completed'])).toBeDefined()
    })

    it('useAIOpsAnalysisBySegment 无 segmentId 时禁用，有值时请求', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            data: {
                analysis: {
                    analysisId: 'a-1', segmentId: 'seg-1', status: 'completed',
                    l1Total: 1, l1Done: 1, attempts: 1,
                    createdAt: '2026-08-20T00:00:00.000Z', updatedAt: '2026-08-20T00:00:00.000Z',
                },
                entities: [],
            },
            meta,
        }))
        const client = createQueryClient()
        const { result: disabled } = renderHook(() => useAIOpsAnalysisBySegment(null), { wrapper: wrapperFor(client) })
        expect(disabled.current.isPending).toBe(true)
        expect(mockCalls()).toHaveLength(0)

        const { result: loaded } = renderHook(() => useAIOpsAnalysisBySegment('seg-1'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(loaded.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/aiops/analyses?segmentId=seg-1')
    })

    it('useAIOpsWindows / useAIOpsJobs / useAIOpsAlerts 各自请求', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta }))
        const client = createQueryClient()
        const { result: windows } = renderHook(() => useAIOpsWindows('L4'), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(windows.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/aiops/windows?level=L4&limit=20')

        const { result: jobs } = renderHook(() => useAIOpsJobs(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(jobs.current.isSuccess).toBe(true))
        expect(String(mockCalls()[1][0])).toContain('/aiops/jobs?limit=20')

        const { result: alerts } = renderHook(() => useAIOpsAlerts(), { wrapper: wrapperFor(client) })
        await waitFor(() => expect(alerts.current.isSuccess).toBe(true))
        expect(String(mockCalls()[2][0])).toContain('/aiops/alerts?limit=20')
    })

    it('create/confirm/stop mutation 调用对应接口', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: { commandId: 'cmd-1' }, meta }))
        const client = createQueryClient()
        const { result: create } = renderHook(() => useCreateAIOpsCommand(), { wrapper: wrapperFor(client) })
        create.current.mutate('把 QPS 提到 200')
        await waitFor(() => expect(create.current.isSuccess).toBe(true))
        expect(String(mockCalls()[0][0])).toContain('/aiops/commands')
        expect(JSON.parse(String(mockCalls()[0][1]?.body))).toEqual({ rawInput: '把 QPS 提到 200' })

        const { result: confirm } = renderHook(() => useConfirmAIOpsCommand(), { wrapper: wrapperFor(client) })
        confirm.current.mutate('cmd-1')
        await waitFor(() => expect(confirm.current.isSuccess).toBe(true))
        expect(String(mockCalls()[1][0])).toContain('/aiops/commands/cmd-1/confirm')

        const { result: stop } = renderHook(() => useStopAIOpsCommand(), { wrapper: wrapperFor(client) })
        stop.current.mutate('cmd-1')
        await waitFor(() => expect(stop.current.isSuccess).toBe(true))
        expect(String(mockCalls()[2][0])).toContain('/aiops/commands/cmd-1/stop')
    })
})