export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected'
export type ClusterNodeRole = 'control-plane' | 'worker'
export type ClusterNodeStatus = 'running' | 'offline' | 'unknown'
export type ExecutionMode = 'apply' | 'test'
export type ExecutionPhase = 'standby' | 'running' | 'error'
export type AsyncPhase = 'idle' | 'pending' | 'success' | 'error'

export interface ClusterNode {
    id: string
    name: string
    role: ClusterNodeRole
    status: ClusterNodeStatus
    ready: boolean
    zone: string
    gpuCapacity: number
}

export interface ClusterSnapshot {
    id: string
    name: string
    provider: string
    context: string
    version: string
    connectionStatus: ConnectionStatus
    controlPlane: ClusterNode
    workers: ClusterNode[]
    checkedAt: string
    simulationRunSupported: boolean
    simulationRateSupported: boolean
    providers: Record<string, ProviderHealth>
    serverTime: string
    logicalTime: string
    clockRate: number
    clockAppliedRate: number
    clockResourceVersion: string
    clockConverged: boolean
    clockSynchronizedInstances: number
    clockTotalInstances: number
    clockState: string
}

export interface ProviderHealth {
    state: string
    observedAt: string
    error?: string
    retention?: string
    storage?: string
}

export interface DistributionReceipt {
    id: string
    clusterId: string
    acceptedNodes: number
    resources: {
        models: number
        nodes: number
        tenants: number
        total: number
        revision: string
    }
    createdAt: string
}

export interface SimulationRateReceipt {
    rate: number
    resourceVersion: string
    convergence: string
}
