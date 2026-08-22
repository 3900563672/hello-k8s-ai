import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MonitorPage } from '@/components/features/monitor/MonitorPage'

vi.mock('@/components/features/monitor/MonitorWall', () => ({
    MonitorWall: () => <div data-testid="monitor-wall" />,
}))

const renderPage = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <MonitorPage />
        </QueryClientProvider>,
    )
}

describe('MonitorPage', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('Grafana 健康时展开后可看到监控面板 iframe', async () => {
        const user = userEvent.setup()
        globalThis.fetch = vi.fn(async () => new Response('ok', { status: 200 }))
        renderPage()
        await user.click(await screen.findByText('Grafana 综合视图'))
        expect(await screen.findByTitle('Grafana 监控面板')).toBeInTheDocument()
        expect(screen.queryByText(/Grafana 暂不可用/)).not.toBeInTheDocument()
        expect(screen.getByTestId('monitor-wall')).toBeInTheDocument()
    })

    it('Grafana 不可用时展示告警提示', async () => {
        globalThis.fetch = vi.fn(async () => new Response('not found', { status: 404 }))
        renderPage()
        expect(await screen.findByText(/Grafana 暂不可用/)).toBeInTheDocument()
    })

    it('点击刷新会重新渲染 iframe', async () => {
        const user = userEvent.setup()
        globalThis.fetch = vi.fn(async () => new Response('ok', { status: 200 }))
        renderPage()
        await user.click(await screen.findByText('Grafana 综合视图'))
        const iframe = await screen.findByTitle('Grafana 监控面板')
        const firstSrc = iframe.getAttribute('src')
        await user.click(screen.getByRole('button', { name: /刷新/ }))
        expect(screen.getByTitle('Grafana 监控面板').getAttribute('src')).toBe(firstSrc)
    })
})