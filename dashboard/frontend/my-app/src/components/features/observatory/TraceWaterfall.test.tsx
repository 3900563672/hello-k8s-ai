import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TraceWaterfall } from '@/components/features/observatory/TraceWaterfall'
import type { TraceSpan, TraceSummary } from '@/types/trace.types'

const traceSpans: TraceSpan[] = [
    {
        spanId: 'span-root',
        service: 'gateway',
        operation: 'GET /api/tenants',
        startTime: '2026-08-20T00:00:00.000Z',
        durationMs: 120,
        status: 'ok',
        attributes: {},
        events: [],
    },
    {
        spanId: 'span-child',
        parentSpanId: 'span-root',
        service: 'scheduler',
        operation: 'Reconcile Tenant',
        startTime: '2026-08-20T00:00:00.010Z',
        durationMs: 80,
        status: 'error',
        attributes: {},
        events: [],
    },
]

const otherSpans: TraceSpan[] = [
    {
        spanId: 'span-2',
        service: 'scheduler',
        operation: 'Reconcile Node',
        startTime: '2026-08-20T00:00:01.000Z',
        durationMs: 45,
        status: 'ok',
        attributes: {},
        events: [],
    },
]

const traces: TraceSummary[] = [
    {
        traceId: 'trace-1',
        rootService: 'gateway',
        rootOperation: 'GET /api/tenants',
        durationMs: 120,
        startTime: '2026-08-20T00:00:00.000Z',
        spanCount: 2,
        errorSpanCount: 1,
        entities: {},
    },
    {
        traceId: 'trace-2',
        rootService: 'scheduler',
        rootOperation: 'Reconcile Node',
        durationMs: 45,
        startTime: '2026-08-20T00:00:01.000Z',
        spanCount: 1,
        errorSpanCount: 0,
        entities: {},
    },
]

vi.mock('@/api/queries/traceQueries', () => ({
    useTraceDetail: (traceId: string) => ({
        data: { data: { spans: traceId === 'trace-1' ? traceSpans : otherSpans } },
        isLoading: false,
        isError: false,
    }),
}))

describe('TraceWaterfall', () => {
    it('无 trace 时展示空态', () => {
        render(<TraceWaterfall traces={[]} />)
        expect(screen.getByText('无匹配 Trace')).toBeInTheDocument()
    })

    it('渲染 trace 列表并展示首个 trace 的 span 瀑布', async () => {
        render(<TraceWaterfall traces={traces} />)
        expect(await screen.findByText('Reconcile Tenant')).toBeInTheDocument()
        expect(screen.getAllByText('GET /api/tenants').length).toBeGreaterThanOrEqual(2)
        expect(screen.getByText('gateway')).toBeInTheDocument()
        expect(screen.getAllByText('120.0ms').length).toBeGreaterThanOrEqual(2)
    })

    it('搜索过滤 trace 列表（右侧详情不受影响）', async () => {
        const user = userEvent.setup()
        render(<TraceWaterfall traces={traces} />)
        await user.type(screen.getByPlaceholderText('搜索 Trace ID / 操作 / 服务'), 'Node')
        expect(screen.getByRole('button', { name: /trace-2/ })).toBeInTheDocument()
        expect(screen.queryByRole('button', { name: /trace-1/ })).not.toBeInTheDocument()
        expect(screen.getAllByText('Reconcile Node').length).toBeGreaterThanOrEqual(1)
    })

    it('点击 trace 切换选中并展示对应 span 详情', async () => {
        const user = userEvent.setup()
        render(<TraceWaterfall traces={traces} />)
        await user.click(screen.getByRole('button', { name: /trace-2/ }))
        expect((await screen.findAllByText('Reconcile Node')).length).toBeGreaterThanOrEqual(2)
    })
})