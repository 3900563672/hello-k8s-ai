import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DataOverviewPage } from '@/components/features/trace/DataOverviewPage'

// CI runner 上该大页面用例渲染较慢（v8 coverage 下实测可到 5s+），文件级放宽超时避免 flake
vi.setConfig({ testTimeout: 15000 })
import { useTimeStore } from '@/stores/timeSlice'
import overviewFixture from '@/lib/mocks/fixtures/overview.json'
import type { BackendResource, KubernetesCondition, ModelSpec, NodeSpec, OrchestratorSpec, TenantSpec } from '@/types/config.types'
import type {
    BackendDeployment,
    BackendEvent,
    BackendLease,
    BackendNode,
    BackendPod,
    OverviewData,
    TenantTraffic,
    TraceDetail,
} from '@/types/trace.types'

const h = vi.hoisted(() => {
    const refetch = vi.fn().mockResolvedValue(undefined)
    const overviewState = {
        data: undefined as { data: OverviewData; meta: { warnings: string[] } } | undefined,
        isPending: false,
        isError: false,
        isFetching: false,
        error: undefined as { message: string } | undefined,
        refetch,
    }
    const detailState = {
        isPending: false,
        isError: false,
        error: undefined as { message: string } | undefined,
        data: undefined as { data: TraceDetail } | undefined,
    }
    return { refetch, overviewState, detailState }
})

vi.mock('@/api/queries/traceQueries', () => ({
    useOverview: () => h.overviewState,
    useTraceDetail: () => h.detailState,
}))

vi.mock('@/components/features/trace/SegmentPanel', () => ({
    SegmentPanel: () => <div data-testid="segment-panel" />,
}))
vi.mock('@/components/features/trace/ExperimentPanel', () => ({
    ExperimentPanel: () => <div data-testid="experiment-panel" />,
}))
vi.mock('@/components/features/trace/TraceWorkbench', () => ({
    TraceWorkbench: () => <div data-testid="trace-workbench" />,
}))
vi.mock('echarts-for-react', () => ({
    default: () => <div data-testid="echarts-mock" />,
}))

const base = overviewFixture.data as unknown as OverviewData

function loadOverview(data: OverviewData, warnings: string[] = []) {
    h.overviewState.data = { data, meta: { warnings } }
    h.overviewState.isPending = false
    h.overviewState.isError = false
    h.overviewState.error = undefined
}

const condition = (type: string, status: string): KubernetesCondition => ({ type, status })

function makeResource<TSpec extends Record<string, unknown> = Record<string, unknown>>(
    name: string,
    kind: string,
    extra: Partial<BackendResource<TSpec>> = {},
    spec: TSpec = {} as TSpec,
): BackendResource<TSpec> {
    return {
        ref: { apiVersion: 'hello-k8s-ai.io/v1', kind, name },
        metadata: { generation: 1, resourceVersion: '1' },
        spec,
        status: { phase: 'Running' },
        conditions: [condition('Ready', 'True')],
        ...extra,
    }
}

function makeRichOverview(): OverviewData {
    const rich = structuredClone(base)
    rich.configuration = {
        asOf: base.asOf,
        availability: 'available',
        tenants: [makeResource<TenantSpec & Record<string, unknown>>('tenant-a', 'Tenant', {}, { displayName: '租户A', priority: 'P1', qps: 30, ttftThresholdMs: 300, queueThreshold: 50, ttftScaleDownThresholdMs: 200, queueScaleDownThreshold: 10 })],
        models: [makeResource<ModelSpec & Record<string, unknown>>('model-a', 'Model', {}, { displayName: 'model-a', gpuUnits: 1, maxConcurrency: 64, absoluteScore: 100, coldStartMs: 100, performance: { prefillBaseMs: 10, prefillPerTokenUs: 1, decodePerTokenMs: 5 } })],
        workerNodes: [makeResource<NodeSpec & Record<string, unknown>>('node-a', 'WorkerNode', {}, { displayName: '节点A', gpu: 1, maxConcurrency: 64 })],
        simulatorInstances: [makeResource('sim-1', 'SimulatorInstance', { spec: { modelRef: { name: 'model-a' }, tenantRef: { name: 'tenant-a' } } })],
        policies: {
            tenantModel: [makeResource('tm-policy', 'TenantModelPolicy')],
            tenantNode: [],
            modelNode: [],
        },
        orchestrators: [makeResource<OrchestratorSpec & Record<string, unknown>>('orch-1', 'Orchestrator', {}, {
            tenantRef: { name: 'tenant-a' },
            scaleUpCooldownSeconds: 60,
            scaleDownCooldownSeconds: 120,
            allowScaleToZero: false,
            minReplicas: 1,
            maxReplicas: 10,
            maxScaleUpBatch: 4,
        })],
        simulationClocks: [makeResource('clock-1', 'SimulationClock')],
        tenantPerformance: [makeResource('perf-1', 'TenantPerformance')],
        tenantRuntimes: [makeResource('rt-1', 'TenantRuntime')],
    }
    const trafficTenant: TenantTraffic = {
        tenant: { apiVersion: 'hello-k8s-ai.io/v1', kind: 'Tenant', name: 'tenant-a' },
        displayName: '租户A',
        priority: 'P1',
        requestedQPS: 30,
        allocatedQPS: 25,
        allocationBalanced: false,
        performance: { sampleCount: 5, freshness: 'fresh' },
        runtimePhase: 'Running',
        readyReplicaCount: 2,
        instances: [
            {
                name: 'sim-1',
                model: 'model-a',
                assignedQPS: 25,
                availableReplicas: 2,
                desiredReplicas: 2,
                freshness: 'fresh',
                pods: [{
                    ref: { apiVersion: 'v1', kind: 'Pod', namespace: 'default', name: 'pod-1' },
                    phase: 'Running',
                    ready: true,
                    nodeName: 'node-a',
                    conditions: [condition('Ready', 'True')],
                    containers: [{ name: 'sim', ready: true, restartCount: 0, state: 'running' }],
                }],
            },
        ],
    }
    rich.traffic = { asOf: base.asOf, tenants: [trafficTenant] }
    const pod = (name: string, ready: boolean): BackendPod => ({
        ref: { apiVersion: 'v1', kind: 'Pod', namespace: 'default', name },
        phase: ready ? 'Running' : 'Pending',
        ready,
        nodeName: 'node-a',
        conditions: [condition('Ready', ready ? 'True' : 'False')],
        containers: [{ name: 'sim', ready, restartCount: 0, state: ready ? 'running' : 'waiting' }],
        simulatorInstance: 'sim-1',
        tenant: 'tenant-a',
        model: 'model-a',
    })
    const deployment: BackendDeployment = {
        ref: { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'default', name: 'sim-1' },
        desiredReplicas: 2,
        updatedReplicas: 2,
        readyReplicas: 2,
        availableReplicas: 2,
        unavailableReplicas: 0,
        conditions: [condition('Available', 'True')],
        simulatorInstance: 'sim-1',
        tenant: 'tenant-a',
        model: 'model-a',
    }
    const node: BackendNode = {
        ref: { apiVersion: 'v1', kind: 'Node', name: 'node-a' },
        role: 'worker',
        ready: true,
        phase: 'Running',
        schedulable: true,
        zone: 'zone-a',
        conditions: [condition('Ready', 'True')],
        observedAt: base.asOf,
    }
    const service = { ref: { apiVersion: 'v1', kind: 'Service', namespace: 'default', name: 'svc-1' }, type: 'ClusterIP', clusterIP: '10.96.0.10' }
    const lease: BackendLease = {
        ref: { apiVersion: 'coordination.k8s.io/v1', kind: 'Lease', namespace: 'default', name: 'lease-1' },
        holderIdentity: 'sim-1',
        renewTime: base.asOf,
        leaseDurationSeconds: 15,
        fresh: true,
    }
    const event: BackendEvent = {
        id: 'evt-1',
        eventTime: base.asOf,
        type: 'Warning',
        reason: 'BackOff',
        message: 'Back-off restarting failed container',
        count: 3,
        regarding: { apiVersion: 'v1', kind: 'Pod', namespace: 'default', name: 'pod-1' },
        reportingController: 'kubelet',
    }
    rich.workloads = {
        nodes: [node],
        pods: [pod('pod-1', true), pod('pod-2', false)],
        deployments: [deployment],
        services: [service],
        leases: [lease],
        events: [event],
    }
    return rich
}

function makeEmptyOverview(): OverviewData {
    const empty = structuredClone(base)
    empty.configuration = {
        asOf: base.asOf,
        availability: 'available',
        tenants: [],
        models: [],
        workerNodes: [],
        simulatorInstances: [],
        policies: { tenantModel: [], tenantNode: [], modelNode: [] },
        orchestrators: [],
        simulationClocks: [],
        tenantPerformance: [],
        tenantRuntimes: [],
    }
    empty.traffic = { asOf: base.asOf, tenants: [] }
    empty.workloads = { nodes: [], pods: [], deployments: [], services: [], leases: [], events: [] }
    empty.traces = []
    return empty
}

describe('DataOverviewPage', () => {
    beforeEach(() => {
        useTimeStore.setState({ mode: 'latest', timestamp: new Date(0).toISOString(), selectedSnapshotId: null, revision: 0, snapshots: [] })
        h.refetch.mockClear()
        h.overviewState.data = undefined
        h.overviewState.isPending = false
        h.overviewState.isError = false
        h.overviewState.isFetching = false
        h.overviewState.error = undefined
        h.detailState.isPending = false
        h.detailState.isError = false
        h.detailState.error = undefined
        h.detailState.data = undefined
    })

    afterEach(() => cleanup())

    it('加载态：显示聚合提示', () => {
        h.overviewState.isPending = true
        render(<DataOverviewPage />)
        expect(screen.getByText(/正在聚合 Kubernetes、Prometheus 与 Jaeger 数据/)).toBeInTheDocument()
    })

    it('错误态：显示错误信息，重试触发 refetch', async () => {
        const user = userEvent.setup({ delay: null })
        h.overviewState.isError = true
        h.overviewState.error = { message: 'backend unreachable' }
        render(<DataOverviewPage />)
        expect(screen.getByText('Overview API 请求失败')).toBeInTheDocument()
        expect(screen.getByText('backend unreachable')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: '重试' }))
        expect(h.refetch).toHaveBeenCalledTimes(1)
    })

    it('latest 模式：header、Informer Live Cache 徽标与空态表格', () => {
        loadOverview(makeEmptyOverview())
        render(<DataOverviewPage />)
        expect(screen.getByText('Kubernetes 真实状态回显')).toBeInTheDocument()
        expect(screen.getByText('Informer Live Cache')).toBeInTheDocument()
        expect(screen.queryByText('PostgreSQL Snapshot')).not.toBeInTheDocument()
        expect(screen.getByText('Kubernetes 中没有 Tenant Traffic 读模型')).toBeInTheDocument()
        expect(screen.getAllByText('Informer cache 中没有该类资源').length).toBe(3)
        expect(screen.getByText('Informer cache 中没有工作负载')).toBeInTheDocument()
        expect(screen.getByText('Informer cache 中没有基础设施资源')).toBeInTheDocument()
        expect(screen.getByText('Kubernetes API 当前没有 Event')).toBeInTheDocument()
        expect(screen.getByText('当前窗口没有 Trace，或 Jaeger Provider 不可用')).toBeInTheDocument()
    })

    it('meta.warnings 渲染为 Notice', () => {
        loadOverview(base, ['Prometheus 保留窗口不足', 'Jaeger 无持久化'])
        render(<DataOverviewPage />)
        expect(screen.getByText('Prometheus 保留窗口不足')).toBeInTheDocument()
        expect(screen.getByText('Jaeger 无持久化')).toBeInTheDocument()
    })

    it('指标卡：有数据渲染数值与 Sparkline，缺失 metric 显示占位符', () => {
        loadOverview(base)
        render(<DataOverviewPage />)
        // 卡片标签 + 趋势图标签各出现一次
        expect(screen.getAllByText('TTFT').length).toBe(2)
        expect(screen.getAllByText('Time Scale').length).toBe(2)
        // 5 个有数据 metric → 5 个 Sparkline；Time Scale 无 metric → 1 个 No samples 占位
        expect(screen.getAllByText('No samples').length).toBe(1)
        // 6 个趋势图 mock（5 有数据 + 1 无数据）
        expect(screen.getAllByTestId('echarts-mock').length).toBe(6)
    })

    it('历史模式：显示 PostgreSQL Snapshot 徽标，回到最新状态调用 store action', async () => {
        const user = userEvent.setup({ delay: null })
        const returnSpy = vi.spyOn(useTimeStore.getState(), 'returnToLatest').mockImplementation(() => undefined)
        useTimeStore.setState({ mode: 'historical', timestamp: '2026-08-20T00:00:00.000Z' })
        loadOverview(base)
        render(<DataOverviewPage />)
        expect(screen.getByText('PostgreSQL Snapshot')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /回到最新状态/ }))
        expect(returnSpy).toHaveBeenCalledTimes(1)
        returnSpy.mockRestore()
    })

    it('刷新按钮：点击调用 refetch，isFetching 时禁用', async () => {
        const user = userEvent.setup({ delay: null })
        loadOverview(base)
        render(<DataOverviewPage />)
        const refresh = screen.getByRole('button', { name: /刷新/ })
        await user.click(refresh)
        expect(h.refetch).toHaveBeenCalledTimes(1)
        cleanup()
        h.overviewState.isFetching = true
        render(<DataOverviewPage />)
        expect(screen.getByRole('button', { name: /刷新/ })).toBeDisabled()
    })

    it('CollapsibleSection：默认展开/收起行为与切换', async () => {
        const user = userEvent.setup({ delay: null })
        loadOverview(base)
        render(<DataOverviewPage />)
        // 集群状态 defaultOpen：内部内容可见
        expect(screen.getByText('Kubernetes 中没有 Tenant Traffic 读模型')).toBeInTheDocument()
        expect(screen.queryByTestId('segment-panel')).not.toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /时间段切面分析/ }))
        expect(screen.getByTestId('segment-panel')).toBeInTheDocument()
        expect(screen.queryByTestId('experiment-panel')).not.toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /实验管理/ }))
        expect(screen.getByTestId('experiment-panel')).toBeInTheDocument()
        await user.click(screen.getByRole('button', { name: /时间段切面分析/ }))
        expect(screen.queryByTestId('segment-panel')).not.toBeInTheDocument()
}, 15000)

    it('富数据：状态卡计数、Traffic 表、资源表、工作负载、基础设施与事件', () => {
        loadOverview(makeRichOverview())
        render(<DataOverviewPage />)
        expect(screen.getByText('1/2')).toBeInTheDocument() // Pod Ready
        expect(screen.getByText('1/1')).toBeInTheDocument() // Deployment Ready
        // Traffic 表
        expect(screen.getByText('租户A')).toBeInTheDocument()
        expect(screen.getByText('30')).toBeInTheDocument()
        expect(screen.getByText('25')).toBeInTheDocument()
        expect(screen.getByText('sim-1 · 2/2')).toBeInTheDocument()
        // 资源表
        expect(screen.getByText('Tenant / Model / WorkerNode')).toBeInTheDocument()
        expect(screen.getByText('tenant-a')).toBeInTheDocument()
        expect(screen.getByText('model-a')).toBeInTheDocument()
        expect(screen.getAllByText('SimulatorInstance').length).toBeGreaterThanOrEqual(2) // 表标题 + 资源 kind
        expect(screen.getByText('Clock / Policy / Orchestrator / Performance / Runtime')).toBeInTheDocument()
        expect(screen.getByText('tm-policy')).toBeInTheDocument()
        expect(screen.getByText('orch-1')).toBeInTheDocument()
        expect(screen.getByText('clock-1')).toBeInTheDocument()
        expect(screen.getByText('perf-1')).toBeInTheDocument()
        expect(screen.getByText('rt-1')).toBeInTheDocument()
        // 工作负载
        expect(screen.getByText('Deployment / Pod')).toBeInTheDocument()
        expect(screen.getByText('pod-1')).toBeInTheDocument()
        expect(screen.getByText('pod-2')).toBeInTheDocument()
        expect(screen.getByText('Available:True')).toBeInTheDocument()
        // 基础设施
        expect(screen.getByText('Node / Service / Lease')).toBeInTheDocument()
        expect(screen.getByText('svc-1')).toBeInTheDocument()
        expect(screen.getByText('lease-1')).toBeInTheDocument()
        expect(screen.getByText('worker · schedulable · zone-a')).toBeInTheDocument()
        // 事件
        expect(screen.getByText('BackOff')).toBeInTheDocument()
        expect(screen.getByText('Back-off restarting failed container')).toBeInTheDocument()
    })

    it('Trace 列表：点击打开详情面板，加载态', async () => {
        loadOverview(base)
        h.detailState.isPending = true
        h.detailState.isError = false
        h.detailState.error = undefined
        render(<DataOverviewPage />)
        const firstTrace = base.traces[0]
        fireEvent.click(screen.getAllByText(`${firstTrace.rootService} · ${firstTrace.rootOperation}`)[0])
        expect(screen.getByText(/正在读取 Jaeger Span/)).toBeInTheDocument()
    }, 10000)

    it('Trace 列表：详情错误态展示错误信息', async () => {
        loadOverview(base)
        h.detailState.isPending = false
        h.detailState.isError = true
        h.detailState.error = { message: 'jaeger down' }
        render(<DataOverviewPage />)
        const firstTrace = base.traces[0]
        fireEvent.click(screen.getAllByText(`${firstTrace.rootService} · ${firstTrace.rootOperation}`)[0])
        expect(screen.getByText('jaeger down')).toBeInTheDocument()
    }, 10000)

    it('Trace 列表：详情数据态展示 spans/links 并支持关闭', async () => {
        loadOverview(base)
        h.detailState.isPending = false
        h.detailState.isError = false
        h.detailState.error = undefined
        h.detailState.data = {
            data: {
                traceId: base.traces[0].traceId,
                spans: [
                    {
                        spanId: 'span-1',
                        service: 'svc-a',
                        operation: 'op-root',
                        startTime: '2026-08-20T00:00:00.000Z',
                        durationMs: 100,
                        status: 'ok',
                        attributes: {},
                        events: [],
                    },
                    {
                        spanId: 'span-2',
                        parentSpanId: 'span-1',
                        service: 'svc-b',
                        operation: 'op-child',
                        startTime: '2026-08-20T00:00:01.000Z',
                        durationMs: 50,
                        status: 'error',
                        attributes: {},
                        events: [{ name: 'retry', time: '2026-08-20T00:00:01.500Z' }],
                    },
                ],
                entityLinks: [{ apiVersion: 'v1', kind: 'Pod', namespace: 'default', name: 'pod-1' }],
            },
        }
        render(<DataOverviewPage />)
        const firstTrace = base.traces[0]
        fireEvent.click(screen.getAllByText(`${firstTrace.rootService} · ${firstTrace.rootOperation}`)[0])
        expect(screen.getByText('svc-a · op-root')).toBeInTheDocument()
        expect(screen.getByText('svc-b · op-child')).toBeInTheDocument()
        expect(screen.getByText('1 span events')).toBeInTheDocument()
        expect(screen.getByText('Pod/pod-1')).toBeInTheDocument()
        fireEvent.click(screen.getByRole('button', { name: '关闭' }))
        expect(screen.queryByText('svc-a · op-root')).not.toBeInTheDocument()
    }, 10000)

    it('Trace 为空：显示空提示', () => {
        const empty = structuredClone(base)
        empty.traces = []
        loadOverview(empty)
        render(<DataOverviewPage />)
        expect(screen.getByText('当前窗口没有 Trace，或 Jaeger Provider 不可用')).toBeInTheDocument()
    })
})