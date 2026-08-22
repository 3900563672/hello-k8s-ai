import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AiInsightPanel } from '@/components/features/observatory/AiInsightPanel'

const meta = {
    requestId: 'r1',
    servedAt: '2026-08-20T00:00:00.000Z',
    partial: false,
    warnings: [],
    sourceVersions: {},
}

const notEnabled = () => new Response(JSON.stringify({
    error: { code: 'AI_OPS_DISABLED', message: 'AIOps 未启用：请配置 AIOPS_ENABLED=true 并设置 API Key' },
    meta,
}), { status: 404, headers: { 'Content-Type': 'application/json' } })

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
})

const renderPanel = () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
        <QueryClientProvider client={client}>
            <AiInsightPanel />
        </QueryClientProvider>,
    )
}

describe('AiInsightPanel', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('AIOps 未启用（404）时展示引导空态', async () => {
        globalThis.fetch = vi.fn(async () => notEnabled())
        renderPanel()
        expect(await screen.findByText('AIOps 分析未启用')).toBeInTheDocument()
        expect(screen.getByText(/AIOPS_ENABLED=true/)).toBeInTheDocument()
    })

    it('无分析记录时展示空列表与详情占位', async () => {
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/aiops/analyses')) return json({ data: [], meta })
            if (url.includes('/aiops/jobs')) return json({ data: [], meta })
            if (url.includes('/aiops/limits')) return notEnabled()
            if (url.includes('/aiops/quota')) return notEnabled()
            return notEnabled()
        })
        renderPanel()
        expect(await screen.findByText('暂无分析记录')).toBeInTheDocument()
        expect(screen.getByText('选择左侧分析查看 AI 洞察')).toBeInTheDocument()
        expect(screen.getByText('最近分析（15s 轮询）')).toBeInTheDocument()
    })

    it('渲染已完成分析的状态徽章、总分与实体总结', async () => {
        const analysis = {
            analysisId: 'analysis-1',
            segmentId: 'segment-very-long-id-000001',
            status: 'completed',
            l1Total: 1,
            l1Done: 1,
            scores: {
                goal: 90,
                stability: 80,
                efficiency: 70,
                anomaly: 95,
                overall: 85,
                verdict: '整体健康，关注效率维度',
                reason: '目标达成良好，效率略低于阈值。',
            },
            attempts: 2,
            createdAt: '2026-08-20T00:00:00.000Z',
            updatedAt: '2026-08-20T00:01:00.000Z',
        }
        const entity = {
            summaryId: 'entity-1',
            analysisId: 'analysis-1',
            entityKind: 'Node',
            entityName: 'node-a',
            classification: 'healthy',
            phenomenon: '节点负载平稳',
            issueFlag: false,
            conclusion: '无需处理',
            createdAt: '2026-08-20T00:00:00.000Z',
        }
        globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
            const url = String(input)
            if (url.includes('/aiops/analyses?segmentId=')) {
                return json({ data: { analysis, entities: [entity] }, meta })
            }
            if (url.includes('/aiops/analyses')) return json({ data: [analysis], meta })
            if (url.includes('/aiops/jobs')) return json({ data: [], meta })
            return notEnabled()
        })
        const user = userEvent.setup()
        renderPanel()
        const card = await screen.findByText(/segment-…/)
        await user.click(card)
        expect(await screen.findByText('总分')).toBeInTheDocument()
        expect(screen.getByText('85')).toBeInTheDocument()
        expect(screen.getByText('目标达成')).toBeInTheDocument()
        expect(screen.getByText('实体级总结（1）')).toBeInTheDocument()
        expect(screen.getByText(/Node \/ node-a/)).toBeInTheDocument()
        expect(screen.getByText('优质')).toBeInTheDocument()
        expect(screen.getByText(/已试 2 次/)).toBeInTheDocument()
    })
})
