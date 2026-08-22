import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TraceWorkbench } from '@/components/features/trace/TraceWorkbench'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'
import type { SegmentOverviewData } from '@/types/trace.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echart-mock" />,
}))

const snapshots: Snapshot[] = [
    {
        id: 'snap-a',
        timestamp: '2026-08-21T10:00:00.000Z',
        weight: 1,
        type: 'config',
        trigger: 'event',
        domain: 'configuration',
        severity: 'critical',
        title: '模型配额变更',
        summary: 'maxConcurrency 从 8 调整到 16',
        source: 'config-apply',
        impact: { tenants: 1, nodes: 2, models: 1, changes: 2 },
        tags: ['model'],
    },
    {
        id: 'snap-b',
        timestamp: '2026-08-21T10:05:00.000Z',
        weight: 1,
        type: 'event',
        trigger: 'time',
        domain: 'scheduler',
        severity: 'normal',
        title: '调度器扩容',
        summary: '副本从 2 扩到 4',
        source: 'orchestrator',
        impact: { tenants: 1, nodes: 2, models: 1, changes: 1 },
        tags: [],
    },
]

const segmentData: SegmentOverviewData = {
    availability: 'available',
    start: '2026-08-21T09:35:00.000Z',
    end: '2026-08-21T10:05:00.000Z',
    startSnapshot: {
        capturedAt: '2026-08-21T09:35:00.000Z',
        workloads: { nodes: [{ ready: true } as never], pods: [{ ready: true }] as never[], deployments: [{ readyReplicas: 1, desiredReplicas: 1 }] as never[], services: [], leases: [], events: [] },
        traffic: { asOf: 'x', tenants: [] },
        configuration: { tenants: [], models: [], workerNodes: [], simulatorInstances: [] } as never,
    },
    endSnapshot: {
        capturedAt: '2026-08-21T10:05:00.000Z',
        workloads: { nodes: [{ ready: true } as never], pods: [{ ready: true }, { ready: true }] as never[], deployments: [{ readyReplicas: 2, desiredReplicas: 2 }] as never[], services: [], leases: [], events: [] },
        traffic: { asOf: 'x', tenants: [] },
        configuration: { tenants: [], models: [], workerNodes: [], simulatorInstances: [] } as never,
    },
    metrics: {},
    traces: [],
    freshness: { prometheus: { state: 'ready', observedAt: 'x' } },
}

const segmentQuery = vi.fn((_query: unknown) => ({ data: { data: segmentData, meta: { warnings: [] } } }))

vi.mock('@/api/queries/traceQueries', () => ({
    useSegment: (query: unknown) => segmentQuery(query),
}))

describe('TraceWorkbench', () => {
    beforeEach(() => {
        useTimeStore.setState({ snapshots, mode: 'latest', selectedSnapshotId: null, revision: 0 })
        segmentQuery.mockClear()
    })

    it('渲染切面列表、计数与筛选区', () => {
        render(<TraceWorkbench />)
        expect(screen.getByText('切面时间线')).toBeInTheDocument()
        expect(screen.getByText('2 / 2')).toBeInTheDocument()
        expect(screen.getByText('模型配额变更')).toBeInTheDocument()
        expect(screen.getByText('调度器扩容')).toBeInTheDocument()
        expect(screen.getByText('严重度')).toBeInTheDocument()
        expect(screen.getAllByText('配置决策').length).toBeGreaterThan(0)
    })

    it('搜索过滤切面列表', async () => {
        const user = userEvent.setup()
        render(<TraceWorkbench />)
        await user.type(screen.getByPlaceholderText('搜索标题 / 摘要…'), '扩容')
        expect(screen.queryByText('模型配额变更')).not.toBeInTheDocument()
        expect(screen.getByText('调度器扩容')).toBeInTheDocument()
        expect(screen.getByText('1 / 2')).toBeInTheDocument()
    })

    it('按严重度筛选并清除', async () => {
        const user = userEvent.setup()
        render(<TraceWorkbench />)
        await user.click(screen.getByText('严重'))
        expect(screen.queryByText('调度器扩容')).not.toBeInTheDocument()
        await user.click(screen.getByText('清除'))
        expect(screen.getByText('调度器扩容')).toBeInTheDocument()
    })

    it('点击切面选中并触发段查询与详情统计', async () => {
        const user = userEvent.setup()
        render(<TraceWorkbench />)
        await user.click(screen.getByText('模型配额变更'))
        expect(useTimeStore.getState().selectedSnapshotId).toBe('snap-a')
        expect(await screen.findByText('数据源新鲜度')).toBeInTheDocument()
        expect(screen.getByText('prometheus')).toBeInTheDocument()
        expect(segmentQuery).toHaveBeenCalled()
    })

    it('选中切面后切到决策序列 tab 展示窗口内配置决策', async () => {
        const user = userEvent.setup()
        render(<TraceWorkbench />)
        await user.click(screen.getByText('模型配额变更'))
        await user.click(await screen.findByRole('button', { name: /决策序列/ }))
        expect(screen.getAllByText('模型配额变更').length).toBeGreaterThan(0)
        expect(screen.getByText('maxConcurrency 从 8 调整到 16')).toBeInTheDocument()
    })
})