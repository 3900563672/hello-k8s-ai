import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AIOpsJobList } from '@/components/features/observatory/AIOpsJobList'

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const renderList = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <AIOpsJobList />
        </QueryClientProvider>,
    )
}

const makeJob = (overrides: Record<string, unknown> = {}) => ({
    jobId: 'job-1',
    segmentId: 'segment-very-long-id-000001',
    kind: 'window-analysis',
    status: 'done',
    attempts: 1,
    maxAttempts: 3,
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
    ...overrides,
})

describe('AIOpsJobList', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('无任务时展示空态说明', async () => {
        globalThis.fetch = vi.fn(async () => json({ data: [], meta }))
        renderList()
        expect(await screen.findByText(/暂无任务/)).toBeInTheDocument()
    })

    it('渲染任务状态徽章与进行中计数', async () => {
        globalThis.fetch = vi.fn(async () => json({
            data: [
                makeJob({ jobId: 'job-1', status: 'running', segmentId: 'seg-1', startedAt: '2026-08-20T00:00:00.000Z' }),
                makeJob({ jobId: 'job-2', status: 'failed', attempts: 2, lastError: '分析超时' }),
                makeJob({ jobId: 'job-3', status: 'done', segmentId: 'seg-3' }),
            ],
            meta,
        }))
        renderList()
        expect(await screen.findByText('执行中')).toBeInTheDocument()
        expect(screen.getByText('进行中 1')).toBeInTheDocument()
        expect(screen.getByText('失败')).toBeInTheDocument()
        expect(screen.getByText('第 2 次')).toBeInTheDocument()
        expect(screen.getByText('分析超时')).toBeInTheDocument()
        expect(screen.getByText('已完成')).toBeInTheDocument()
    })

    it('长 segmentId 会被缩短展示', async () => {
        globalThis.fetch = vi.fn(async () => json({
            data: [makeJob({ segmentId: '0123456789abcdefghijklmnopqrstuvwxyz' })],
            meta,
        }))
        renderList()
        expect(await screen.findByText(/^01234567/)).toBeInTheDocument()
        expect(screen.getByText(/…uvwxyz$/)).toBeInTheDocument()
    })
})