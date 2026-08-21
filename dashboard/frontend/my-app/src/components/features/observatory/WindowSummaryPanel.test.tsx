import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WindowSummaryPanel } from '@/components/features/observatory/WindowSummaryPanel'

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const renderPanel = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <WindowSummaryPanel />
        </QueryClientProvider>,
    )
}

describe('WindowSummaryPanel', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('后端未接入（404）时展示未接入空态', async () => {
        globalThis.fetch = vi.fn(async () => json({ error: { code: 'NOT_READY', message: 'x' }, meta }, 404))
        renderPanel()
        expect(await screen.findByText('时间聚合未接入')).toBeInTheDocument()
    })

    it('无窗口时展示暂无窗口总结', async () => {
        globalThis.fetch = vi.fn(async () => json({ data: [], meta }))
        renderPanel()
        expect(await screen.findByText('暂无窗口总结')).toBeInTheDocument()
    })

    it('渲染 L3/L4 窗口、分数与 verdict', async () => {
        globalThis.fetch = vi.fn(async () => json({
            data: [
                {
                    windowId: 'w-1',
                    level: 'L3',
                    windowStart: '2026-08-20T00:00:00.000Z',
                    windowEnd: '2026-08-20T01:00:00.000Z',
                    scores: { overall: 85, verdict: '整体健康', reason: '本窗口集群稳定' },
                    findings: [],
                    createdAt: '2026-08-20T01:00:00.000Z',
                    updatedAt: '2026-08-20T01:00:00.000Z',
                },
                {
                    windowId: 'w-2',
                    level: 'L4',
                    windowStart: '2026-08-20T00:00:00.000Z',
                    windowEnd: '2026-08-20T23:59:00.000Z',
                    scores: { overall: 58, verdict: '存在风险', reason: '全天存在 3 次节点抖动' },
                    findings: [],
                    createdAt: '2026-08-20T23:59:00.000Z',
                    updatedAt: '2026-08-20T23:59:00.000Z',
                },
            ],
            meta,
        }))
        renderPanel()
        expect(await screen.findByText('窗口总结')).toBeInTheDocument()
        expect(screen.getByText('日总结')).toBeInTheDocument()
        expect(screen.getByText('85')).toBeInTheDocument()
        expect(screen.getByText('58')).toBeInTheDocument()
        expect(screen.getByText('整体健康')).toBeInTheDocument()
        expect(screen.getByText('本窗口集群稳定')).toBeInTheDocument()
    })
})