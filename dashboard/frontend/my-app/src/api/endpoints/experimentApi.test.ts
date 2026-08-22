// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
    completeExperiment,
    createExperiment,
    failExperiment,
    fetchExperiment,
    fetchExperiments,
    startExperiment,
} from '@/api/endpoints/experimentApi'
import type { ExperimentDetailEnvelope } from '@/types/experiment.types'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const detailEnvelope = (): ExperimentDetailEnvelope => ({
    data: {
        segment: {
            segmentId: 'seg-1',
            tenant: 'tenant-a',
            name: '实验',
            status: 'running',
            createdAt: '2026-08-20T00:00:00.000Z',
            updatedAt: '2026-08-20T00:00:00.000Z',
        },
        events: [],
        metrics: [],
        traces: [],
    },
    meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
})

describe('experimentApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => { originalFetch = globalThis.fetch })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('fetchExperiments 无筛选时请求列表路径', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta: {} }))
        const result = await fetchExperiments()
        expect(result.data).toEqual([])
        expect(String(mockCalls()[0][0])).toContain('/experiments')
        expect(String(mockCalls()[0][0])).not.toContain('?')
    })

    it('fetchExperiments 带 status 过滤并编码', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({ data: [], meta: {} }))
        await fetchExperiments('running')
        expect(String(mockCalls()[0][0])).toContain('/experiments?status=running')
    })

    it('fetchExperiment 编码 segmentId', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope()))
        const result = await fetchExperiment('a/b c')
        expect(result.data.segment.segmentId).toBe('seg-1')
        expect(String(mockCalls()[0][0])).toContain('/experiments/a%2Fb%20c')
    })

    it('createExperiment POST tenant/name JSON', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope()))
        await createExperiment('tenant-a', '新实验')
        const [url, init] = mockCalls()[0]
        expect(String(url)).toContain('/experiments')
        expect(init?.method).toBe('POST')
        expect(JSON.parse(String(init?.body))).toEqual({ tenant: 'tenant-a', name: '新实验' })
    })

    it('start/complete/fail 走对应动作路径', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(detailEnvelope()))
        await startExperiment('seg-1')
        expect(String(mockCalls()[0][0])).toContain('/experiments/seg-1/start')
        await completeExperiment('seg-1')
        expect(String(mockCalls()[1][0])).toContain('/experiments/seg-1/complete')
        await failExperiment('seg-1', 'OOM')
        const [url, init] = mockCalls()[2]
        expect(String(url)).toContain('/experiments/seg-1/fail')
        expect(init?.method).toBe('POST')
        expect(JSON.parse(String(init?.body))).toEqual({ reason: 'OOM' })
    })
})