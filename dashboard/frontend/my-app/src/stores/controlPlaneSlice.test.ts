import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
    canRunTest,
    onlineWorkerCount,
    useControlPlaneStore,
} from '@/stores/controlPlaneSlice'
import type { ClusterNode, ClusterSnapshot } from '@/types/control-plane.types'

const makeNode = (overrides: Partial<ClusterNode> = {}): ClusterNode => ({
    id: 'node-1',
    name: 'worker-1',
    role: 'worker',
    status: 'running',
    ready: true,
    zone: 'zone-a',
    gpuCapacity: 2,
    ...overrides,
})

const makeCluster = (overrides: Partial<ClusterSnapshot> = {}): ClusterSnapshot => ({
    ...useControlPlaneStore.getInitialState().cluster,
    connectionStatus: 'connected',
    simulationRunSupported: true,
    simulationRateSupported: true,
    clockResourceVersion: '',
    clockConverged: false,
    workers: [makeNode()],
    ...overrides,
})

const okResponse = (body: unknown, status = 200): Response =>
    new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })

describe('controlPlaneSlice', () => {
    let originalFetch: typeof globalThis.fetch

    beforeEach(() => {
        originalFetch = globalThis.fetch
        useControlPlaneStore.setState(useControlPlaneStore.getInitialState())
    })

    afterEach(() => {
        globalThis.fetch = originalFetch
    })

    it('onlineWorkerCount 只统计 ready 且 running 的 Worker', () => {
        const cluster = makeCluster({
            workers: [
                makeNode(),
                makeNode({ id: 'w2', name: 'worker-2', ready: false, status: 'offline' }),
                makeNode({ id: 'w3', name: 'worker-3', ready: true, status: 'offline' }),
                makeNode({ id: 'w4', name: 'control-plane', role: 'control-plane', status: 'running', ready: true }),
            ],
        })
        expect(onlineWorkerCount(cluster)).toBe(2)
    })

    it('canRunTest 需要支持 + 已连接 + 在线 Worker', () => {
        expect(canRunTest(makeCluster())).toBe(true)
        expect(canRunTest(makeCluster({ simulationRunSupported: false }))).toBe(false)
        expect(canRunTest(makeCluster({ connectionStatus: 'connecting' }))).toBe(false)
        expect(canRunTest(makeCluster({ workers: [] }))).toBe(false)
    })

    it('setExecutionMode 在不可运行 test 时拒绝并保持 apply', () => {
        const store = useControlPlaneStore.getState()
        expect(store.setExecutionMode('test')).toBe(false)
        const state = useControlPlaneStore.getState()
        expect(state.executionMode).toBe('apply')
        expect(state.executionPhase).toBe('standby')
    })

    it('setExecutionMode 可运行时进入 test，回 apply 复位 standby', () => {
        useControlPlaneStore.setState({ cluster: makeCluster() })
        expect(useControlPlaneStore.getState().setExecutionMode('test')).toBe(true)
        let state = useControlPlaneStore.getState()
        expect(state.executionMode).toBe('test')
        expect(state.executionPhase).toBe('running')
        expect(useControlPlaneStore.getState().setExecutionMode('apply')).toBe(true)
        state = useControlPlaneStore.getState()
        expect(state.executionMode).toBe('apply')
        expect(state.executionPhase).toBe('standby')
    })

    it('forceApplyMode 从 test/running 复位，apply/standby 时为空操作', () => {
        useControlPlaneStore.setState({ cluster: makeCluster() })
        useControlPlaneStore.getState().setExecutionMode('test')
        useControlPlaneStore.getState().forceApplyMode()
        let state = useControlPlaneStore.getState()
        expect(state.executionMode).toBe('apply')
        expect(state.executionPhase).toBe('standby')

        useControlPlaneStore.getState().forceApplyMode()
        state = useControlPlaneStore.getState()
        expect(state.executionMode).toBe('apply')
        expect(state.executionPhase).toBe('standby')
    })

    it('refreshCluster 成功后进入 connected 并映射集群字段', async () => {
        const bootstrap = {
            data: {
                cluster: {
                    name: 'kind-dev',
                    context: 'kind-hello-k8s-ai-dev',
                    version: 'v1.30.0',
                    connected: true,
                    cacheSynced: true,
                    nodeCount: 3,
                    readyNodes: 2,
                },
                clock: {
                    serverTime: '2026-08-20T00:00:00.000Z',
                    logicalTime: '2026-08-20T00:00:00.000Z',
                    rate: 1,
                    appliedRate: 1,
                    resourceVersion: '42',
                    converged: true,
                    synchronizedInstances: 3,
                    totalInstances: 3,
                    state: 'running',
                    capabilities: { canSetRate: true, simulatorAcceleration: true },
                },
                counts: {},
                nodes: [
                    { ref: { name: 'master' }, role: 'control-plane', ready: true, phase: 'Running', schedulable: false, zone: 'z1' },
                    { ref: { name: 'worker-a' }, role: 'worker', ready: true, phase: 'Running', schedulable: true, zone: 'z1', capacity: { 'nvidia.com/gpu': '4' } },
                    { ref: { name: 'worker-b' }, role: 'worker', ready: false, phase: 'Unknown', schedulable: false },
                ],
                providers: { prometheus: { state: 'ready', observedAt: '2026-08-20T00:00:00.000Z' } },
            },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }
        globalThis.fetch = vi.fn(async () => okResponse(bootstrap))
        await useControlPlaneStore.getState().refreshCluster()
        const state = useControlPlaneStore.getState()
        expect(state.refreshPhase).toBe('success')
        expect(state.cluster.connectionStatus).toBe('connected')
        expect(state.cluster.context).toBe('kind-hello-k8s-ai-dev')
        expect(state.cluster.controlPlane.name).toBe('master')
        expect(state.cluster.workers).toHaveLength(2)
        expect(state.cluster.workers[0]).toMatchObject({ name: 'worker-a', status: 'running', ready: true, gpuCapacity: 4 })
        expect(state.cluster.workers[1]).toMatchObject({ name: 'worker-b', status: 'unknown', ready: false })
        expect(state.cluster.simulationRateSupported).toBe(true)
        expect(state.cluster.clockResourceVersion).toBe('42')
        expect(state.cluster.clockConverged).toBe(true)
        expect(state.cluster.providers.prometheus?.state).toBe('ready')
    })

    it('refreshCluster 失败后置 disconnected 并回退 apply/error', async () => {
        globalThis.fetch = vi.fn(async () => {
            throw new Error('Backend 不可达')
        })
        await useControlPlaneStore.getState().refreshCluster()
        const state = useControlPlaneStore.getState()
        expect(state.refreshPhase).toBe('error')
        expect(state.cluster.connectionStatus).toBe('disconnected')
        expect(state.executionMode).toBe('apply')
        expect(state.executionPhase).toBe('error')
        expect(state.lastError).toBe('Backend 不可达')
    })

    it('refreshCluster 进行中时忽略重复触发', async () => {
        let resolveFetch: (value: Response) => void = () => {}
        globalThis.fetch = vi.fn(() => new Promise<Response>((resolve) => {
            resolveFetch = resolve
        }))
        const first = useControlPlaneStore.getState().refreshCluster()
        void useControlPlaneStore.getState().refreshCluster()
        expect(globalThis.fetch).toHaveBeenCalledTimes(1)
        resolveFetch(okResponse({
            data: {
                cluster: { name: 'x', context: 'x', connected: true, cacheSynced: true, nodeCount: 0, readyNodes: 0 },
                clock: {
                    serverTime: '2026-08-20T00:00:00.000Z', logicalTime: '2026-08-20T00:00:00.000Z',
                    rate: 1, appliedRate: 1, converged: true, synchronizedInstances: 0, totalInstances: 0, state: 'running',
                    capabilities: { canSetRate: false, simulatorAcceleration: false },
                },
                counts: {}, nodes: [], providers: {},
            },
            meta: { requestId: 'r1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }))
        await first
    })

    it('distributeConfig 用在线 Worker 数生成回执', async () => {
        useControlPlaneStore.setState({ cluster: makeCluster({ workers: [makeNode(), makeNode({ id: 'w2', name: 'worker-2' })] }) })
        globalThis.fetch = vi.fn(async () => okResponse({
            data: {
                asOf: '2026-08-20T00:00:00.000Z',
                availability: 'available',
                models: [{}, {}],
                workerNodes: [{}, {}, {}],
                tenants: [{}],
                policies: { tenantModel: [], tenantNode: [], modelNode: [] },
                orchestrators: [],
                simulationClocks: [],
                simulatorInstances: [],
                tenantPerformance: [],
                tenantRuntimes: [],
            },
            meta: {
                requestId: 'dist-1',
                servedAt: '2026-08-20T00:00:00.000Z',
                partial: false,
                warnings: [],
                sourceVersions: { kubernetes: 'v100' },
            },
        }))
        const receipt = await useControlPlaneStore.getState().distributeConfig()
        expect(receipt).not.toBeNull()
        expect(receipt?.acceptedNodes).toBe(2)
        expect(receipt?.resources).toEqual({ models: 2, nodes: 3, tenants: 1, total: 6, revision: 'v100' })
        expect(useControlPlaneStore.getState().distributionPhase).toBe('success')
    })

    it('distributeConfig 失败置 error 并返回 null', async () => {
        globalThis.fetch = vi.fn(async () => {
            throw new Error('读取配置失败')
        })
        const receipt = await useControlPlaneStore.getState().distributeConfig()
        expect(receipt).toBeNull()
        const state = useControlPlaneStore.getState()
        expect(state.distributionPhase).toBe('error')
        expect(state.lastError).toBe('读取配置失败')
    })

    it('setSimulationRate 在未就绪/非法参数时不发请求', async () => {
        const fetchMock = vi.fn<typeof globalThis.fetch>(async () => okResponse({}, 200))
        globalThis.fetch = fetchMock

        useControlPlaneStore.setState({ cluster: makeCluster({ simulationRateSupported: false }) })
        expect(await useControlPlaneStore.getState().setSimulationRate(5)).toBe(false)

        useControlPlaneStore.setState({ cluster: makeCluster({ connectionStatus: 'disconnected' }) })
        expect(await useControlPlaneStore.getState().setSimulationRate(5)).toBe(false)

        useControlPlaneStore.setState({ cluster: makeCluster({ clockResourceVersion: '17', clockConverged: false }) })
        expect(await useControlPlaneStore.getState().setSimulationRate(5)).toBe(false)

        useControlPlaneStore.setState({ cluster: makeCluster() })
        expect(await useControlPlaneStore.getState().setSimulationRate(0)).toBe(false)
        expect(await useControlPlaneStore.getState().setSimulationRate(2.5)).toBe(false)
        expect(await useControlPlaneStore.getState().setSimulationRate(21)).toBe(false)
        expect(fetchMock).not.toHaveBeenCalled()
    })

    it('setSimulationRate 成功提交 PATCH 并更新时钟状态', async () => {
        useControlPlaneStore.setState({
            cluster: makeCluster({ clockResourceVersion: '17', clockConverged: true }),
            simulationRatePhase: 'idle',
        })
        const fetchMock = vi.fn<typeof globalThis.fetch>(async () => okResponse({
            data: { results: [{ resourceVersion: '18', convergence: 'pending' }] },
            meta: { requestId: 'rate-1', servedAt: '2026-08-20T00:00:00.000Z', partial: false, warnings: [], sourceVersions: {} },
        }, 202))
        globalThis.fetch = fetchMock
        expect(await useControlPlaneStore.getState().setSimulationRate(5)).toBe(true)
        const state = useControlPlaneStore.getState()
        expect(state.simulationRatePhase).toBe('success')
        expect(state.cluster.clockRate).toBe(5)
        expect(state.cluster.clockResourceVersion).toBe('18')
        expect(state.cluster.clockConverged).toBe(false)

        expect(String(fetchMock.mock.calls[0]?.[0])).toBe('/api/v1/clock/rate')
        expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('PATCH')
        expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ rate: 5, resourceVersion: '17', dryRun: false })
        expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get('Idempotency-Key')?.startsWith('simulation-rate-')).toBe(true)
    })

    it('setSimulationRate 失败置 error 并返回 false', async () => {
        useControlPlaneStore.setState({
            cluster: makeCluster({ clockResourceVersion: '17', clockConverged: true }),
        })
        globalThis.fetch = vi.fn(async () => {
            throw new Error('提交倍速失败')
        })
        expect(await useControlPlaneStore.getState().setSimulationRate(5)).toBe(false)
        const state = useControlPlaneStore.getState()
        expect(state.simulationRatePhase).toBe('error')
        expect(state.lastError).toBe('提交倍速失败')
    })
})
