import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AiChatWidget } from '@/components/features/aiops/AiChatWidget'

const meta = {
    requestId: 'r1',
    servedAt: '2026-08-20T00:00:00.000Z',
    partial: false,
    warnings: [],
    sourceVersions: {},
}

const notEnabled = (message: string) => new Response(JSON.stringify({
    error: { code: 'AI_OPS_DISABLED', message },
    meta,
}), { status: 404, headers: { 'Content-Type': 'application/json' } })

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
        // 用户消息保留在会话中
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
})
