import { apiRequest } from '@/api/client'
import type { ApiEnvelope } from '@/types/api.types'
import type { ConfigurationReadModel } from '@/types/config.types'
import type {
    ClusterNode,
    ClusterSnapshot,
    DistributionReceipt,
    ProviderHealth,
    SimulationRateReceipt,
} from '@/types/control-plane.types'
import type { Snapshot } from '@/types/time.types'
import { createClientId } from '@/lib/clientId'

interface BackendNode {
    ref: { name: string; uid?: string }
    role: 'control-plane' | 'worker'
    ready: boolean
    phase: string
    schedulable: boolean
    zone?: string
    version?: string
    capacity?: Record<string, string>
}

interface BootstrapData {
    cluster: {
        name: string
        context?: string
        version?: string
        connected: boolean
        cacheSynced: boolean
        cacheSyncedAt?: string
        nodeCount: number
        readyNodes: number
    }
    clock: {
        serverTime: string
        logicalTime: string
        rate: number
        appliedRate: number
        resourceVersion?: string
        converged: boolean
        synchronizedInstances: number
        totalInstances: number
        state: string
        capabilities: {
            canSetRate: boolean
            simulatorAcceleration: boolean
        }
    }
    counts: Record<string, number>
    nodes: BackendNode[]
    providers: Record<string, ProviderHealth>
    timeline: Snapshot[]
}

const unavailableControlPlane = (): ClusterNode => ({
    id: 'control-plane-unavailable',
    name: '未发现控制平面节点',
    role: 'control-plane',
    status: 'unknown',
    ready: false,
    zone: '',
    gpuCapacity: 0,
})

export const createInitialCluster = (): ClusterSnapshot => ({
    id: 'cluster-pending',
    name: 'Kubernetes Cluster',
    provider: 'Kubernetes API',
    context: '等待 Backend',
    version: '',
    connectionStatus: 'connecting',
    controlPlane: unavailableControlPlane(),
    workers: [],
    checkedAt: new Date(0).toISOString(),
    simulationRunSupported: false,
    simulationRateSupported: false,
    providers: {},
    serverTime: new Date(0).toISOString(),
    logicalTime: new Date(0).toISOString(),
    clockRate: 1,
    clockAppliedRate: 1,
    clockResourceVersion: '',
    clockConverged: false,
    clockSynchronizedInstances: 0,
    clockTotalInstances: 0,
    clockState: 'running',
})

const mapNode = (node: BackendNode): ClusterNode => ({
    id: node.ref.uid || node.ref.name,
    name: node.ref.name,
    role: node.role,
    status: node.ready ? 'running' : node.phase === 'Unknown' ? 'unknown' : 'offline',
    ready: node.ready,
    zone: node.zone || '未标注可用区',
    gpuCapacity: Number.parseInt(node.capacity?.['nvidia.com/gpu'] || '0', 10) || 0,
})

export async function fetchBootstrap(): Promise<BootstrapData> {
    const response = await apiRequest<ApiEnvelope<BootstrapData>>('/bootstrap')
    return response.data
}

export async function fetchClusterSnapshot(
    _current?: ClusterSnapshot,
): Promise<ClusterSnapshot> {
    const bootstrap = await fetchBootstrap()
    const nodes = bootstrap.nodes.map(mapNode)
    const controlPlane = nodes.find((node) => node.role === 'control-plane')
        ?? unavailableControlPlane()
    const workers = nodes.filter((node) => node.role === 'worker')
    return {
        id: bootstrap.cluster.context || bootstrap.cluster.name,
        name: bootstrap.cluster.name,
        provider: 'Kubernetes API',
        context: bootstrap.cluster.context || 'in-cluster',
        version: bootstrap.cluster.version || '',
        connectionStatus:
            bootstrap.cluster.connected && bootstrap.cluster.cacheSynced
                ? 'connected'
                : 'connecting',
        controlPlane,
        workers,
        checkedAt: bootstrap.clock.serverTime,
        simulationRunSupported: false,
        simulationRateSupported:
            bootstrap.clock.capabilities.canSetRate &&
            bootstrap.clock.capabilities.simulatorAcceleration,
        providers: bootstrap.providers,
        serverTime: bootstrap.clock.serverTime,
        logicalTime: bootstrap.clock.logicalTime,
        clockRate: bootstrap.clock.rate,
        clockAppliedRate: bootstrap.clock.appliedRate,
        clockResourceVersion: bootstrap.clock.resourceVersion || '',
        clockConverged: bootstrap.clock.converged,
        clockSynchronizedInstances: bootstrap.clock.synchronizedInstances,
        clockTotalInstances: bootstrap.clock.totalInstances,
        clockState: bootstrap.clock.state,
    }
}

interface OperationReceipt {
    results: Array<{
        resourceVersion?: string
        convergence: string
        error?: string
    }>
}

export async function updateSimulationRate(
    rate: number,
    resourceVersion: string,
): Promise<SimulationRateReceipt> {
    const response = await apiRequest<ApiEnvelope<OperationReceipt>>('/clock/rate', {
        method: 'PATCH',
        headers: {
            'Idempotency-Key': createClientId('simulation-rate'),
        },
        body: JSON.stringify({
            rate,
            ...(resourceVersion ? { resourceVersion } : {}),
            dryRun: false,
        }),
    })
    const result = response.data.results[0]
    if (!result) throw new Error('Backend 未返回倍速更新结果')
    return {
        rate,
        resourceVersion: result.resourceVersion || resourceVersion,
        convergence: result.convergence,
    }
}

export async function fetchReplayTimeline(): Promise<Snapshot[]> {
    const response = await apiRequest<ApiEnvelope<{ timeline: Snapshot[] }>>('/replay?limit=1000')
    return response.data.timeline
}

/**
 * Controller 会自动收敛 CR 变更。此操作校验 Backend cache 已观察到的配置，
 * 不再模拟向 Worker 分发浏览器本地对象。
 */
export async function distributeConfiguration(
    cluster: ClusterSnapshot,
): Promise<DistributionReceipt> {
    const response = await apiRequest<ApiEnvelope<ConfigurationReadModel>>('/configuration')
    const data = response.data
    const onlineWorkers = cluster.workers.filter(
        (node) => node.ready && node.status === 'running',
    )
    const revision = response.meta.sourceVersions.kubernetes || response.meta.servedAt
    return {
        id: response.meta.requestId,
        clusterId: cluster.id,
        acceptedNodes: onlineWorkers.length,
        resources: {
            models: data.models.length,
            nodes: data.workerNodes.length,
            tenants: data.tenants.length,
            total: data.models.length + data.workerNodes.length + data.tenants.length,
            revision,
        },
        createdAt: response.meta.servedAt,
    }
}
