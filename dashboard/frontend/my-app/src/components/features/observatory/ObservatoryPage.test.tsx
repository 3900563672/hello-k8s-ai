import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ObservatoryPage } from '@/components/features/observatory/ObservatoryPage'
import { useTimeStore } from '@/stores/timeSlice'
import type { OverviewData } from '@/types/trace.types'

class IntersectionObserverStub {
    root = null
    rootMargin = ''
    thresholds = []
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
        return []
    }
}
vi.stubGlobal('IntersectionObserver', IntersectionObserverStub)

const refetch = vi.fn()
const overview: OverviewData = {
    availability: 'available',
    asOf: '2026-08-21T00:00:00.000Z',
    clock: {
        serverTime: 'x',
        actualTime: 'x',
        logicalTime: 'x',
        rate: 1,
        state: 'running',
        authoritative: true,
        capabilities: { simulatorAcceleration: true },
    },
    configuration: { tenants: [], models: [], workerNodes: [], simulatorInstances: [] } as never,
    traffic: { asOf: 'x', tenants: [] },
    workloads: {
        nodes: [{ ref: { apiVersion: 'v1', kind: 'Node', name: 'node-a' }, ready: true } as never],
        pods: [
            { ref: { apiVersion: 'v1', kind: 'Pod', name: 'pod-001' }, ready: true, nodeName: 'node-a' },
            { ref: { apiVersion: 'v1', kind: 'Pod', name: 'pod-002' }, ready: false, phase: 'CrashLoopBackOff', nodeName: 'node-a' },
        ] as never[],
        deployments: [],
        services: [],
        leases: [],
        events: [],
    },
    metrics: {},
    traces: [],
    freshness: {},
}

vi.mock('@/api/queries/traceQueries', () => ({
    useOverview: () => ({ data: { data: overview }, isFetching: false, refetch }),
    useTraceDetail: () => ({ data: { data: { spans: [] } } }),
}))

vi.mock('@/api/queries/aiopsQueries', () => ({
    useAIOpsAnalyses: () => ({ data: { data: [] } }),
    useAIOpsAnalysisBySegment: () => ({ data: undefined }),
}))

vi.mock('@/components/features/trace/SegmentPanel', () => ({
    SegmentPanel: () => <div data-testid="segment-panel" />,
}))
vi.mock('@/components/features/trace/ExperimentPanel', () => ({
    ExperimentPanel: () => <div data-testid="experiment-panel" />,
}))
vi.mock('@/components/features/monitor/MonitorWall', () => ({
    MonitorWall: () => <div data-testid="monitor-wall" />,
}))
vi.mock('@/components/features/observatory/AiInsightPanel', () => ({
    AiInsightPanel: () => <div data-testid="ai-insight-panel" />,
}))
vi.mock('@/components/features/observatory/AlertList', () => ({
    AlertList: () => <div data-testid="alert-list" />,
}))
vi.mock('@/components/features/observatory/CommandInput', () => ({
    CommandInput: () => <div data-testid="command-input" />,
}))
vi.mock('@/components/features/observatory/WindowSummaryPanel', () => ({
    WindowSummaryPanel: () => <div data-testid="window-summary-panel" />,
}))

describe('ObservatoryPage', () => {
    beforeEach(() => {
        useTimeStore.setState({ mode: 'latest', timestamp: new Date(0).toISOString(), selectedSnapshotId: null, revision: 0, snapshots: [] })
        refetch.mockClear()
    })

    it('渲染标题、统计徽标与六大分区', () => {
        render(<ObservatoryPage />)
        expect(screen.getByText('状态总览')).toBeInTheDocument()
        expect(screen.getByText('1/2')).toBeInTheDocument()
        expect(screen.getAllByText('集群拓扑').length).toBeGreaterThan(0)
        expect(screen.getAllByText('实时指标').length).toBeGreaterThan(0)
        expect(screen.getAllByText('Grafana').length).toBeGreaterThan(0)
        expect(screen.getAllByText('调度与切面').length).toBeGreaterThan(0)
        expect(screen.getAllByText('AI 洞察').length).toBeGreaterThan(0)
        expect(screen.getAllByText('调用链').length).toBeGreaterThan(0)
        expect(screen.getByTitle('Grafana 总览')).toBeInTheDocument()
    })

    it('子面板按预期挂载', async () => {
        const user = userEvent.setup()
        render(<ObservatoryPage />)
        await user.click(screen.getByRole('button', { name: /时间段切面分析/ }))
        await user.click(screen.getByRole('button', { name: /切面实验/ }))
        expect(screen.getByTestId('segment-panel')).toBeInTheDocument()
        expect(screen.getByTestId('experiment-panel')).toBeInTheDocument()
        expect(screen.getByTestId('monitor-wall')).toBeInTheDocument()
        expect(screen.getByTestId('ai-insight-panel')).toBeInTheDocument()
        expect(screen.getByTestId('alert-list')).toBeInTheDocument()
        expect(screen.getByTestId('command-input')).toBeInTheDocument()
    })

    it('点击刷新触发 refetch', async () => {
        const user = userEvent.setup()
        render(<ObservatoryPage />)
        await user.click(screen.getByRole('button', { name: /刷新/ }))
        expect(refetch).toHaveBeenCalled()
    })

    it('侧边导航提供分区入口', () => {
        render(<ObservatoryPage />)
        expect(screen.getByRole('button', { name: /拓扑/ })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /实时指标/ })).toBeInTheDocument()
    })
})