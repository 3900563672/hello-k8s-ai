// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiRequestError } from '@/api/client'
import {
    confirmAIOpsCommand,
    createAIOpsCommand,
    fetchAIOpsAlerts,
    fetchAIOpsAnalysis,
    fetchAIOpsAnalysisBySegment,
    fetchAIOpsAnalyses,
    fetchAIOpsChatMessages,
    fetchAIOpsCommand,
    fetchAIOpsJobs,
    fetchAIOpsLimits,
    fetchAIOpsQuota,
    fetchAIOpsSettings,
    fetchAIOpsTemplates,
    fetchAIOpsWindows,
    stopAIOpsCommand,
    streamAIOpsChat,
    updateAIOpsSettings,
} from '@/api/endpoints/aiopsApi'

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const emptyEnvelope = () => ({ data: {}, meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} } })

const sseResponse = (chunks: string[]): Response => {
    const stream = new ReadableStream<Uint8Array>({
        start(controller) {
            for (const chunk of chunks) controller.enqueue(new TextEncoder().encode(chunk))
            controller.close()
        },
    })
    return new Response(stream, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

describe('aiopsApi', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => { originalFetch = globalThis.fetch })
    afterEach(() => { globalThis.fetch = originalFetch })

    const mockCalls = () => (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls

    it('fetchAIOpsAnalyses 带 status 与 limit', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await fetchAIOpsAnalyses('completed')
        expect(String(mockCalls()[0][0])).toContain('/aiops/analyses?status=completed&limit=50')
        await fetchAIOpsAnalyses(undefined, 10)
        expect(String(mockCalls()[1][0])).toContain('/aiops/analyses?limit=10')
    })

    it('fetchAIOpsAnalysis / BySegment / Templates / Limits / Quota 路径正确', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await fetchAIOpsAnalysis('a/1')
        expect(String(mockCalls()[0][0])).toContain('/aiops/analyses/a%2F1')
        await fetchAIOpsAnalysisBySegment('seg 1')
        expect(String(mockCalls()[1][0])).toContain('/aiops/analyses?segmentId=seg%201')
        await fetchAIOpsTemplates()
        expect(String(mockCalls()[2][0])).toContain('/aiops/templates')
        await fetchAIOpsLimits()
        expect(String(mockCalls()[3][0])).toContain('/aiops/limits')
        await fetchAIOpsQuota()
        expect(String(mockCalls()[4][0])).toContain('/aiops/quota')
    })

    it('fetchAIOpsWindows / Alerts 携带 level 与 limit', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await fetchAIOpsWindows('L4', 5)
        expect(String(mockCalls()[0][0])).toContain('/aiops/windows?level=L4&limit=5')
        await fetchAIOpsAlerts(30)
        expect(String(mockCalls()[1][0])).toContain('/aiops/alerts?limit=30')
    })

    it('createAIOpsCommand POST rawInput；confirm/stop/fetch 路径编码', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await createAIOpsCommand('把 QPS 提到 200')
        const [url, init] = mockCalls()[0]
        expect(String(url)).toContain('/aiops/commands')
        expect(init?.method).toBe('POST')
        expect(JSON.parse(String(init?.body))).toEqual({ rawInput: '把 QPS 提到 200' })

        await confirmAIOpsCommand('cmd/1')
        expect(String(mockCalls()[1][0])).toContain('/aiops/commands/cmd%2F1/confirm')
        expect(mockCalls()[1][1]?.method).toBe('POST')

        await fetchAIOpsCommand('cmd/1')
        expect(String(mockCalls()[2][0])).toContain('/aiops/commands/cmd%2F1')
        expect(mockCalls()[2][1]?.method).toBeUndefined()

        await stopAIOpsCommand('cmd/1')
        expect(String(mockCalls()[3][0])).toContain('/aiops/commands/cmd%2F1/stop')
        expect(mockCalls()[3][1]?.method).toBe('POST')
    })

    it('fetchAIOpsJobs / ChatMessages 携带筛选参数', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await fetchAIOpsJobs('running', 5)
        expect(String(mockCalls()[0][0])).toContain('/aiops/jobs?status=running&limit=5')
        await fetchAIOpsChatMessages('sess 1', 100)
        expect(String(mockCalls()[1][0])).toContain('/aiops/chat/messages?sessionId=sess+1&limit=100')
    })

    it('fetchAIOpsSettings / updateAIOpsSettings POST 掩码配置', async () => {
        globalThis.fetch = vi.fn(async () => okResponse(emptyEnvelope()))
        await fetchAIOpsSettings()
        expect(String(mockCalls()[0][0])).toContain('/aiops/settings')
        expect(mockCalls()[0][1]?.method).toBeUndefined()
        await updateAIOpsSettings({ apiKey: 'sk-x', enabled: true })
        expect(mockCalls()[1][1]?.method).toBe('POST')
        expect(JSON.parse(String(mockCalls()[1][1]?.body))).toEqual({ apiKey: 'sk-x', enabled: true })
    })

    it('streamAIOpsChat 分发 lifecycle/tool/text 事件', async () => {
        globalThis.fetch = vi.fn(async () => sseResponse([
            'data: {"type":"lifecycle","phase":"start"}\n\n',
            'data: {"type":"tool","name":"set-traffic","phase":"start"}\n\n',
            'data: {"type":"text","delta":"正在执行"}\n\n',
            'data: {"type":"tool","name":"set-traffic","phase":"end"}\n\n',
            'data: {"type":"lifecycle","phase":"end","durationMs":120}\n\n',
            'data: [DONE]\n\n',
        ]))
        const events: string[] = []
        await streamAIOpsChat('hi', 's1', {
            onLifecycle: (phase, _error, durationMs) => events.push(`lifecycle:${phase}:${durationMs ?? ''}`),
            onTool: (name, phase) => events.push(`tool:${name}:${phase}`),
            onText: (delta) => events.push(`text:${delta}`),
        })
        expect(events).toEqual([
            'lifecycle:start:',
            'tool:set-traffic:start',
            'text:正在执行',
            'tool:set-traffic:end',
            'lifecycle:end:120',
        ])
        const [url, init] = mockCalls()[0]
        expect(String(url)).toContain('/aiops/chat')
        expect(init?.method).toBe('POST')
        expect(JSON.parse(String(init?.body))).toEqual({ message: 'hi', sessionId: 's1' })
        expect(String(init?.headers?.['Idempotency-Key'] ?? '')).toMatch(/^chat-/)
        // 每次调用 key 不同（Date.now 前缀），避免幂等占位命中
    })

    it('streamAIOpsChat 忽略坏行与非法 JSON，跨 chunk 拼接', async () => {
        globalThis.fetch = vi.fn(async () => sseResponse([
            'event: ping\ndata: not-json\n\n',
            'data: {"type":"tex',
            't","delta":"拼接"}\n\n',
        ]))
        const texts: string[] = []
        await streamAIOpsChat('hi', 's1', { onText: (delta) => texts.push(delta) })
        expect(texts).toEqual(['拼接'])
    })

    it('streamAIOpsChat 非 2xx 抛 ApiRequestError（解析 problem）', async () => {
        globalThis.fetch = vi.fn(async () => okResponse({
            error: { code: 'AI_OPS_DISABLED', message: '未启用' },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z' },
        }, 404))
        const error = await streamAIOpsChat('hi', 's1', {}).catch((caught: unknown) => caught)
        expect(error).toBeInstanceOf(ApiRequestError)
        if (error instanceof ApiRequestError) {
            expect(error.status).toBe(404)
            expect(error.message).toBe('未启用')
        }
    })

    it('streamAIOpsChat 无响应体时抛浏览器不支持', async () => {
        globalThis.fetch = vi.fn(async () => new Response(null, { status: 200 }))
        await expect(streamAIOpsChat('hi', 's1', {})).rejects.toThrow('浏览器不支持流式响应')
    })
})