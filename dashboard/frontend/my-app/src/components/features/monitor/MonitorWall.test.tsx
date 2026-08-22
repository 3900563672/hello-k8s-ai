import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MonitorWall } from '@/components/features/monitor/MonitorWall'
import overviewFixture from '@/lib/mocks/fixtures/overview.json'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echarts-mock" />,
}))

const renderWall = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, retryDelay: 0 } } })
    return render(
        <QueryClientProvider client={client}>
            <MonitorWall />
        </QueryClientProvider>,
    )
}

describe('MonitorWall', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('加载中显示占位，成功后渲染五张指标卡', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            if (String(input).includes('/overview')) {
                return new Response(JSON.stringify(overviewFixture), {
                    status: 200,
                    headers: { 'Content-Type': 'application/json' },
                })
            }
            return new Response('{}', { status: 404 })
        })
        renderWall()
        expect(screen.getByText('正在读取指标…')).toBeInTheDocument()
        expect(await screen.findByText('TTFT 首字延迟')).toBeInTheDocument()
        expect(screen.getByText('QPS 吞吐')).toBeInTheDocument()
        expect(screen.getByText('队列积压')).toBeInTheDocument()
        expect(screen.getByText('Tick 延迟')).toBeInTheDocument()
        expect(screen.getByText('错误率')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /全部5/ })).toBeInTheDocument()
    })

    it('查询失败时展示可重试的错误态', async () => {
        globalThis.fetch = vi.fn(async () => {
            throw new Error('Backend 不可达')
        })
        renderWall()
        expect(await screen.findByText('指标读取失败')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /重试/ })).toBeInTheDocument()
    })
})
