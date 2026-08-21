import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CommandInput } from '@/components/features/observatory/CommandInput'

const meta = { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} }

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const limits = {
    maxTrafficQPS: 500,
    maxSimulationRate: 20,
    trafficShapes: ['tide', 'pulse', 'spike'],
    defaultTidalPeriodMinutes: 60,
    defaultPeakQPS: 100,
    defaultRate: 1,
    trafficRequiresTenant: true,
    unlimitedDuration: false,
    supportsStop: true,
}

const quota = { enabled: true, callsUsed: 3, callsMax: 100, tokensUsed: 1000, tokensMax: 10000 }

const parsedCommand = {
    commandId: 'cmd-1',
    rawInput: '将流量提升到 200 QPS，持续 30 分钟',
    parsed: { traffic: { qps: 200 }, durationMinutes: 30 },
    status: 'parsed',
    steps: [],
    createdAt: '2026-08-20T00:00:00.000Z',
    updatedAt: '2026-08-20T00:00:00.000Z',
}

const renderInput = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <CommandInput />
        </QueryClientProvider>,
    )
}

describe('CommandInput', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: parsedCommand, meta })
            }
            if (url.endsWith('/aiops/commands/cmd-1/confirm') && init?.method === 'POST') {
                return json({ data: { ...parsedCommand, status: 'confirmed' }, meta })
            }
            return json({ data: [], meta })
        })
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('展示配额与输入框', async () => {
        renderInput()
        expect(await screen.findByText(/3\/100/)).toBeInTheDocument()
        expect(screen.getByPlaceholderText(/例如：给 preset-tenant-001/)).toBeInTheDocument()
    })

    it('输入指令并解析，展示意图摘要', async () => {
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '将流量提升到 200 QPS，持续 30 分钟')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('已解析')).toBeInTheDocument()
        expect(screen.getByText(/200 QPS/)).toBeInTheDocument()
    })

    it('空输入不触发解析请求', async () => {
        const user = userEvent.setup()
        renderInput()
        await user.click(screen.getByRole('button', { name: /解析/ }))
        const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls.filter(
            ([, init]) => init?.method === 'POST',
        )
        expect(calls).toHaveLength(0)
    })

    it('解析失败展示错误信息', async () => {
        const user = userEvent.setup()
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ error: { code: 'PARSE_FAILED', message: '无法理解指令' }, meta }, 400)
            }
            return json({ data: [], meta })
        })
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '随便写点什么')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('无法理解指令')).toBeInTheDocument()
    })

    it('解析后确认命令进入确认状态', async () => {
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '将流量提升到 200 QPS')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        await user.click(await screen.findByRole('button', { name: /确认/ }))
        expect(await screen.findByText('已确认')).toBeInTheDocument()
    })
})