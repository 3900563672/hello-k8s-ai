import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AlertList } from '@/components/features/observatory/AlertList'

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const renderList = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <AlertList />
        </QueryClientProvider>,
    )
}

describe('AlertList', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('后端未接入（404）时展示未接入空态', async () => {
        globalThis.fetch = vi.fn(async () => json({ error: { code: 'NOT_READY', message: 'x' }, meta }, 404))
        renderList()
        expect(await screen.findByText('警戒能力未接入')).toBeInTheDocument()
        expect(screen.getByText(/后端 M3（时间聚合与警戒）未启用/)).toBeInTheDocument()
    })

    it('无告警时展示暂无警戒', async () => {
        globalThis.fetch = vi.fn(async () => json({ data: [], meta }))
        renderList()
        expect(await screen.findByText('暂无警戒')).toBeInTheDocument()
    })

    it('渲染告警规则、严重级别与解释', async () => {
        globalThis.fetch = vi.fn(async () => json({
            data: [
                {
                    alertId: 'a-1',
                    severity: 'critical',
                    rule: '节点 node-1 健康分低于阈值',
                    triggeredAt: '2026-08-20T00:00:00.000Z',
                    interpretation: '连续 3 个窗口低于 60 分',
                },
                {
                    alertId: 'a-2',
                    severity: 'info',
                    rule: '流量形状已应用',
                    triggeredAt: '2026-08-20T00:00:00.000Z',
                },
            ],
            meta,
        }))
        renderList()
        expect(await screen.findByText('节点 node-1 健康分低于阈值')).toBeInTheDocument()
        expect(screen.getByText('严重')).toBeInTheDocument()
        expect(screen.getByText('连续 3 个窗口低于 60 分')).toBeInTheDocument()
        expect(screen.getByText('流量形状已应用')).toBeInTheDocument()
        expect(screen.getByText('提示')).toBeInTheDocument()
    })
})