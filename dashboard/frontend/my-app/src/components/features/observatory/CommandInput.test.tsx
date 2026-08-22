import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

const baseFetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
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

describe('CommandInput', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        globalThis.fetch = baseFetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
        baseFetch.mockClear()
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
        const calls = baseFetch.mock.calls.filter(
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

    it('输入框回车触发解析', async () => {
        renderInput()
        const input = await screen.findByPlaceholderText(/例如：给 preset-tenant-001/)
        fireEvent.change(input, { target: { value: '回车触发' } })
        fireEvent.keyDown(input, { key: 'Enter' })
        expect(await screen.findByText('已解析')).toBeInTheDocument()
    })

    it('展示可执行范围：波形中文映射/时长不限/随时可停止', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) {
                return json({ data: { ...limits, trafficShapes: ['tidal', 'spike', 'ramp'], unlimitedDuration: true, supportsStop: true }, meta })
            }
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            return json({ data: [], meta })
        })
        renderInput()
        expect(await screen.findByText(/峰值 QPS ≤ 500/)).toBeInTheDocument()
        expect(screen.getByText(/潮汐 \/ 脉冲 \/ 斜坡/)).toBeInTheDocument()
        expect(screen.getByText('时长不限')).toBeInTheDocument()
        expect(screen.getByText('随时可停止')).toBeInTheDocument()
    })

    it('配额禁用时不展示配额条', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: { ...quota, enabled: false }, meta })
            return json({ data: [], meta })
        })
        renderInput()
        await waitFor(() => expect(screen.queryByText(/今日配额/)).not.toBeInTheDocument())
    })

    it('解析结果展示生效值（钳制/默认）与波形预览', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: {
                    ...parsedCommand,
                    parsed: { traffic: { qps: 800 }, rate: 2, sceneType: '潮汐', sceneTimeAnchor: '2026-08-20T00:00:00Z' },
                    applied: {
                        values: [
                            { field: 'peakQps', requested: 800, effective: 500, reason: 'clamped-to-max' },
                            { field: 'rate', requested: null, effective: 1, reason: 'ok' },
                            { field: 'durationMinutes', requested: 30, effective: 30, reason: 'defaulted' },
                        ],
                        curve: [
                            { x: 0, y: 0 },
                            { x: 60, y: 200 },
                        ],
                        wallClockSeconds: 3600,
                    },
                }, meta })
            }
            return json({ data: [], meta })
        })
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '将流量提升到 800 QPS')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('场景：')).toBeInTheDocument()
        expect(screen.getByText('潮汐')).toBeInTheDocument()
        const resultSection = screen.getByText('生效参数：').parentElement?.parentElement
        expect(resultSection).not.toBeNull()
        const result = resultSection as HTMLElement
        expect(within(result).getByText(/峰值 QPS/)).toBeInTheDocument()
        expect(within(result).getByText('800')).toBeInTheDocument()
        expect(within(result).getByText('500')).toBeInTheDocument()
        expect(within(result).getByText('（超上限，已钳制）')).toBeInTheDocument()
        expect(within(result).getByText('（未指定，用默认）')).toBeInTheDocument()
        expect(within(result).getByText(/倍速/)).toBeInTheDocument()
        expect(result.querySelector('svg polyline')).toBeInTheDocument()
    })

    it('执行中展示进度/当前 QPS/停止，停止后进入已停止', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: {
                    ...parsedCommand,
                    status: 'executing',
                    parsed: { traffic: { qps: 200 }, rate: 2 },
                    applied: {
                        values: [],
                        curve: [
                            { x: 0, y: 0 },
                            { x: 60, y: 200 },
                        ],
                        wallClockSeconds: 3600,
                    },
                }, meta })
            }
            if (url.endsWith('/aiops/commands/cmd-1/stop') && init?.method === 'POST') {
                return json({ data: { ...parsedCommand, status: 'stopped' }, meta })
            }
            return json({ data: [], meta })
        })
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '执行一个实验')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('执行中')).toBeInTheDocument()
        expect(screen.getByText(/模拟进度 100%/)).toBeInTheDocument()
        expect(screen.getByText(/墙钟剩余/)).toBeInTheDocument()
        expect(screen.getAllByText(/200 QPS/).length).toBeGreaterThanOrEqual(2)
        await user.click(screen.getByRole('button', { name: /停止/ }))
        expect(await screen.findByText('已停止')).toBeInTheDocument()
    })

    it('完成后展示执行步骤（成功/失败图标与明细）', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: {
                    ...parsedCommand,
                    status: 'done',
                    steps: [
                        { step: 'set-traffic', status: 'done', detail: '写入 500 QPS' },
                        { step: 'start-experiment', status: 'failed', detail: '资源不足' },
                    ],
                }, meta })
            }
            return json({ data: [], meta })
        })
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '执行步骤实验')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('set-traffic')).toBeInTheDocument()
        expect(screen.getByText('写入 500 QPS')).toBeInTheDocument()
        expect(screen.getByText('start-experiment')).toBeInTheDocument()
        expect(screen.getByText('资源不足')).toBeInTheDocument()
        expect(screen.getByText('已完成')).toBeInTheDocument()
    })

    it('失败命令展示 errorText', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: { ...parsedCommand, status: 'failed', errorText: '执行超时：模拟器无响应' }, meta })
            }
            return json({ data: [], meta })
        })
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '失败实验')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('执行超时：模拟器无响应')).toBeInTheDocument()
        expect(screen.getByText('失败')).toBeInTheDocument()
    })

    it('长 commandId 展示短 id', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input)
            if (url.endsWith('/aiops/limits')) return json({ data: limits, meta })
            if (url.endsWith('/aiops/quota')) return json({ data: quota, meta })
            if (url.endsWith('/aiops/commands') && init?.method === 'POST') {
                return json({ data: { ...parsedCommand, commandId: 'cmd-abcdefgh-1234567890-xyz' }, meta })
            }
            return json({ data: [], meta })
        })
        const user = userEvent.setup()
        renderInput()
        await user.type(screen.getByPlaceholderText(/例如：给 preset-tenant-001/), '短 id 实验')
        await user.click(screen.getByRole('button', { name: /解析/ }))
        expect(await screen.findByText('cmd-abcd…90-xyz')).toBeInTheDocument()
    })
})