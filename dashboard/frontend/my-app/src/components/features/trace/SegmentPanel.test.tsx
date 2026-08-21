import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SegmentPanel } from '@/components/features/trace/SegmentPanel'
import { useTimeStore } from '@/stores/timeSlice'
import type { Snapshot } from '@/types/time.types'
import type { SegmentOverviewData } from '@/types/trace.types'

vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echart-mock" />,
}))

const snapshots: Snapshot[] = [
    {
        id: 'snap-1',
        timestamp: '2026-08-21T00:00:00.000Z',
        weight: 1,
        type: 'event',
        trigger: 'time',
        domain: 'scheduler',
        severity: 'normal',
        title: '起点快照',
        summary: '扩缩容开始',
        source: 'orchestrator',
        impact: { tenants: 1, nodes: 2, models: 1, changes: 1 },
        tags: [],
    },
    {
        id: 'snap-2',
        timestamp: '2026-08-21T00:10:00.000Z',
        weight: 1,
        type: 'event',
        trigger: 'time',
        domain: 'scheduler',
        severity: 'attention',
        title: '终点快照',
        summary: '扩缩容结束',
        source: 'orchestrator',
        impact: { tenants: 1, nodes: 2, models: 1, changes: 2 },
        tags: [],
    },
]

const baseSegment: SegmentOverviewData = {
    availability: 'available',
    start: '2026-08-21T00:00:00.000Z',
    end: '2026-08-21T00:10:00.000Z',
    startSnapshot: {
        capturedAt: '2026-08-21T00:00:00.000Z',
        workloads: {
            nodes: [{ ready: true } as never],
            pods: [{ ready: true }, { ready: false }] as never[],
            deployments: [{ readyReplicas: 1, desiredReplicas: 1 }] as never[],
            services: [],
            leases: [],
            events: [],
        },
        traffic: { asOf: 'x', tenants: [{ allocatedQPS: 50 }] as never[] },
        configuration: {
            tenants: [{ name: 't' }],
            models: [{ name: 'm' }],
            workerNodes: [{ name: 'n' }],
            simulatorInstances: [{ spec: { replicas: 2 } }],
        } as never,
    },
    endSnapshot: {
        capturedAt: '2026-08-21T00:10:00.000Z',
        workloads: {
            nodes: [{ ready: true } as never],
            pods: [{ ready: true }, { ready: true }] as never[],
            deployments: [{ readyReplicas: 2, desiredReplicas: 2 }] as never[],
            services: [],
            leases: [],
            events: [],
        },
        traffic: { asOf: 'x', tenants: [{ allocatedQPS: 120 }] as never[] },
        configuration: {
            tenants: [{ name: 't' }],
            models: [{ name: 'm' }],
            workerNodes: [{ name: 'n' }],
            simulatorInstances: [{ spec: { replicas: 4 } }],
        } as never,
    },
    metrics: {
        'simulator.qps': {
            metricId: 'simulator.qps',
            unit: 'qps',
            start: 'x',
            end: 'y',
            stepSeconds: 60,
            resultType: 'matrix',
            warnings: [],
            queriedAt: 'x',
            series: [{ labels: {}, points: [{ time: '2026-08-21T00:01:00.000Z', value: 10 }] }],
        },
    },
    traces: [],
    freshness: {},
}

const segmentQuery = vi.fn((_query: unknown) => ({
    data: {
        data: baseSegment,
        meta: { warnings: ['部分指标缺失'] },
    },
}))

vi.mock('@/api/queries/traceQueries', () => ({
    useSegment: (query: unknown) => segmentQuery(query),
}))

describe('SegmentPanel', () => {
    beforeEach(() => {
        useTimeStore.setState({ snapshots, mode: 'latest', selectedSnapshotId: null, revision: 0 })
        segmentQuery.mockClear()
    })

    it('选择起点终点后分析并展示前后状态对比', async () => {
        const user = userEvent.setup()
        render(<SegmentPanel />)
        const selects = screen.getAllByRole('combobox')
        await user.selectOptions(selects[0], 'snap-1')
        await user.selectOptions(selects[1], 'snap-2')
        await user.click(screen.getByRole('button', { name: /分析/ }))
        expect(segmentQuery).toHaveBeenCalledWith({ start: '2026-08-21T00:00:00.000Z', end: '2026-08-21T00:10:00.000Z' })
        expect(await screen.findByText('起点状态')).toBeInTheDocument()
        expect(screen.getByText('终点状态')).toBeInTheDocument()
        expect(screen.getByText('1/2')).toBeInTheDocument()
        expect(screen.getByText('2/2')).toBeInTheDocument()
        expect(screen.getByText('120')).toBeInTheDocument()
        expect(screen.getByText('部分指标缺失')).toBeInTheDocument()
        expect(screen.getAllByTestId('echart-mock').length).toBeGreaterThan(0)
    })

    it('起点下拉排除最后一个快照，避免终点不晚于起点', () => {
        render(<SegmentPanel />)
        const startSelect = screen.getAllByRole('combobox')[0]
        const options = Array.from(startSelect.querySelectorAll('option')).map((option) => option.value)
        expect(options).toContain('snap-1')
        expect(options).not.toContain('snap-2')
    })

    it('无 trace 时展示空态', async () => {
        const user = userEvent.setup()
        render(<SegmentPanel />)
        const selects = screen.getAllByRole('combobox')
        await user.selectOptions(selects[0], 'snap-1')
        await user.selectOptions(selects[1], 'snap-2')
        await user.click(screen.getByRole('button', { name: /分析/ }))
        expect(await screen.findByText('该时间段内没有 Trace')).toBeInTheDocument()
    })
})