import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AiChatWidget } from '@/components/features/aiops/AiChatWidget'

const meta = {
    requestId: 'r1',
    servedAt: '2026-08-20T00:00:00.000Z',
    partial: false,
    warnings: [],
    sourceVersions: {},
}

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const notEnabled = (message: string) => new Response(JSON.stringify({
    error: { code: 'AI_OPS_DISABLED', message },
    meta,
}), { status: 404, headers: { 'Content-Type': 'application/json' } })

const sseResponse = (chunks: string[]): Response => {
    const stream = new ReadableStream<Uint8Array>({
        start(controller) {
            const encoder = new TextEncoder()
            for (const chunk of chunks) controller.enqueue(encoder.encode(chunk))
            controller.close()
        },
    })
    return new Response(stream, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

const settings = { keyConfigured: true, model: 'gpt-4o-mini', baseUrl: 'https://api.openai.com/v1', enabled: true }

const mockFetch = (handler: (url: string, init?: RequestInit) => Response) => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
        handler(String(input), init))
}

describe('AiChatWidget', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        localStorage.clear()
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('点击气泡打开对话面板', async () => {
        const user = userEvent.setup()
        mockFetch(() => notEnabled('AIOps 未启用'))
        render(<AiChatWidget />)
        expect(screen.getByRole('button', { name: '打开 AI 助手' })).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        expect(screen.getByText('AI 助手')).toBeInTheDocument()
        expect(screen.getByPlaceholderText('输入问题，Enter 发送')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: '发送' })).toBeDisabled()
    })

    it('未启用时发送问题展示友好文案（#124 降级预案）', async () => {
        const user = userEvent.setup()
        mockFetch((url, init) => {
            if (url.includes('/aiops/chat') && init?.method === 'POST') {
                return notEnabled('AIOps 未启用：请先配置')
            }
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        const input = screen.getByPlaceholderText('输入问题，Enter 发送')
        await user.type(input, '当前集群什么情况？')
        await user.click(screen.getByRole('button', { name: '发送' }))
        expect(await screen.findByText(
            'AIOps 未启用：请在设置面板打开开关，并确认已配置 API Key。',
        )).toBeInTheDocument()
        expect(screen.getByText('当前集群什么情况？')).toBeInTheDocument()
    })

    it('设置视图 404 时保留后端校验文案', async () => {
        const user = userEvent.setup()
        mockFetch((url) => {
            if (url.includes('/aiops/settings')) {
                return notEnabled('读取配置失败：AIOps 未启用')
            }
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        await user.click(screen.getByRole('button', { name: 'AI 助手设置' }))
        expect(await screen.findByText('读取配置失败：AIOps 未启用')).toBeInTheDocument()
        expect(screen.getByText('AI 助手设置')).toBeInTheDocument()
    })

    it('成功对话：用户消息与流式助手回答渲染，发送按钮恢复可用', async () => {
        const user = userEvent.setup()
        mockFetch((url, init) => {
            if (url.includes('/aiops/chat/messages')) return json({ data: [], meta })
            if (url.includes('/aiops/chat') && init?.method === 'POST') {
                return sseResponse([
                    'data: {"type":"lifecycle","phase":"start"}\n\n',
                    'data: {"type":"tool","name":"set-traffic","phase":"start"}\n\n',
                    'data: {"type":"text","delta":"正在为你"}\n\n',
                    'data: {"type":"text","delta":"执行流量调整"}\n\n',
                    'data: {"type":"tool","name":"set-traffic","phase":"end"}\n\n',
                    'data: {"type":"lifecycle","phase":"end","durationMs":120}\n\n',
                    'data: [DONE]\n\n',
                ])
            }
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        const input = screen.getByPlaceholderText('输入问题，Enter 发送')
        await user.type(input, '把流量调高')
        await user.click(screen.getByRole('button', { name: '发送' }))
        expect(await screen.findByText('把流量调高')).toBeInTheDocument()
        expect(await screen.findByText('正在为你执行流量调整')).toBeInTheDocument()
        // 发送后输入框被清空，按钮回到禁用；重新输入后恢复可用（busy 已结束）
        await user.type(screen.getByPlaceholderText('输入问题，Enter 发送'), 'x')
        await waitFor(() => expect(screen.getByRole('button', { name: '发送' })).toBeEnabled())
    })

    it('设置视图加载并保存：POST 只含非空字段，保存成功提示', async () => {
        const user = userEvent.setup()
        const postCalls: Array<{ url: string; init?: RequestInit }> = []
        mockFetch((url, init) => {
            if (url.includes('/aiops/chat/messages')) return json({ data: [], meta })
            if (url.includes('/aiops/settings') && init?.method === 'POST') {
                postCalls.push({ url, init })
                return json({ data: settings, meta })
            }
            if (url.includes('/aiops/settings')) return json({ data: settings, meta })
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        await user.click(screen.getByRole('button', { name: 'AI 助手设置' }))
        expect(await screen.findByDisplayValue('gpt-4o-mini')).toBeInTheDocument()
        expect(screen.getByText('已配置')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /^保存$/ }))
        expect(await screen.findByText('已保存 ✓')).toBeInTheDocument()
        expect(postCalls).toHaveLength(1)
        const body = JSON.parse(postCalls[0].init!.body as string) as Record<string, unknown>
        expect(body.model).toBe('gpt-4o-mini')
        expect(body.enabled).toBe(true)
        expect(body.apiKey).toBeUndefined()
    })

    it('设置保存失败展示后端校验文案', async () => {
        const user = userEvent.setup()
        mockFetch((url, init) => {
            if (url.includes('/aiops/chat/messages')) return json({ data: [], meta })
            if (url.includes('/aiops/settings') && init?.method === 'POST') {
                return new Response(JSON.stringify({
                    error: { code: 'SAVE_FAILED', message: '服务端保存失败：模型不可用' },
                    meta,
                }), { status: 400, headers: { 'Content-Type': 'application/json' } })
            }
            if (url.includes('/aiops/settings')) return json({ data: settings, meta })
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        await user.click(screen.getByRole('button', { name: 'AI 助手设置' }))
        await screen.findByDisplayValue('gpt-4o-mini')
        await user.click(screen.getByRole('button', { name: /^保存$/ }))
        expect(await screen.findByText('服务端保存失败：模型不可用')).toBeInTheDocument()
    })

    it('打开面板拉取服务端历史消息', async () => {
        const user = userEvent.setup()
        mockFetch((url) => {
            if (url.includes('/aiops/chat/messages')) {
                return json({ data: [{ role: 'assistant', content: '历史回答内容' }], meta })
            }
            return notEnabled('AIOps 未启用')
        })
        render(<AiChatWidget />)
        await user.click(screen.getByRole('button', { name: '打开 AI 助手' }))
        expect(await screen.findByText('历史回答内容')).toBeInTheDocument()
    })
})